package cli

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

//
// JSON the way jq writes it.
//
// The MCP server was a shell script that built every message with jq, and parity with it is
// byte for byte, so this is jq's way of doing it rather than encoding/json's: keys in the order
// they arrived, numbers as the literal they were written in (canonicalised the way jq 1.7 does,
// so `1e2` comes back as `1E+2`), nothing escaped that jq does not escape — `<`, `&` and U+2028
// go out as they are — and `jq -r` printing a string raw and anything else indented.
//

type jqKind int

const (
	jqNull jqKind = iota
	jqFalse
	jqTrue
	jqNumber
	jqString
	jqArray
	jqObject
)

type jqValue struct {
	kind  jqKind
	text  string    // a string's value, or a number's literal
	keys  []string  // an object's keys, in order
	items []jqValue // an array's items, or an object's values in the order of keys
}

func jqText(text string) jqValue { return jqValue{kind: jqString, text: text} }

func jqBool(value bool) jqValue {
	if value {
		return jqValue{kind: jqTrue}
	}

	return jqValue{kind: jqFalse}
}

func jqObjectOf(pairs ...any) jqValue {
	object := jqValue{kind: jqObject}
	for i := 0; i+1 < len(pairs); i += 2 {
		object.keys = append(object.keys, pairs[i].(string))
		object.items = append(object.items, pairs[i+1].(jqValue))
	}

	return object
}

// parseJQ reads one JSON document, or says it is not one.
func parseJQ(data []byte) (jqValue, bool) {
	if !json.Valid(data) {
		return jqValue{}, false
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()

	value, err := decodeJQ(decoder)
	if err != nil {
		return jqValue{}, false
	}

	return value, true
}

func decodeJQ(decoder *json.Decoder) (jqValue, error) {
	token, err := decoder.Token()
	if err != nil {
		return jqValue{}, err
	}

	switch t := token.(type) {
	case nil:
		return jqValue{kind: jqNull}, nil
	case bool:
		return jqBool(t), nil
	case json.Number:
		return jqValue{kind: jqNumber, text: string(t)}, nil
	case string:
		return jqText(t), nil
	case json.Delim:
		if t == '[' {
			array := jqValue{kind: jqArray}
			for decoder.More() {
				item, err := decodeJQ(decoder)
				if err != nil {
					return jqValue{}, err
				}

				array.items = append(array.items, item)
			}

			_, err := decoder.Token()

			return array, err
		}

		object := jqValue{kind: jqObject}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return jqValue{}, err
			}

			item, err := decodeJQ(decoder)
			if err != nil {
				return jqValue{}, err
			}

			// A repeated key keeps its first place and its last value, as in jq
			if at := object.index(key.(string)); at >= 0 {
				object.items[at] = item

				continue
			}

			object.keys = append(object.keys, key.(string))
			object.items = append(object.items, item)
		}

		_, err := decoder.Token()

		return object, err
	}

	return jqValue{}, nil
}

func (v jqValue) index(key string) int {
	for i, k := range v.keys {
		if k == key {
			return i
		}
	}

	return -1
}

// field is `.key` on this value: present tells a missing key from one that is there. ok is false
// when the value is not an object, which is where jq stops with an error and prints nothing.
func (v jqValue) field(key string) (value jqValue, ok bool) {
	if v.kind == jqNull {
		return jqValue{kind: jqNull}, true
	}

	if v.kind != jqObject {
		return jqValue{}, false
	}

	if at := v.index(key); at >= 0 {
		return v.items[at], true
	}

	return jqValue{kind: jqNull}, true
}

// truthy is what `//` asks: everything is, except null and false.
func (v jqValue) truthy() bool {
	return v.kind != jqNull && v.kind != jqFalse
}

// compact is `jq -c`.
func (v jqValue) compact() string {
	var out strings.Builder
	v.write(&out, false, 0)

	return out.String()
}

// pretty is jq's default output.
func (v jqValue) pretty() string {
	var out strings.Builder
	v.write(&out, true, 0)

	return out.String()
}

// raw is `jq -r`: a string as it is, anything else indented.
func (v jqValue) raw() string {
	if v.kind == jqString {
		return v.text
	}

	return v.pretty()
}

func (v jqValue) write(out *strings.Builder, indented bool, depth int) {
	newline := func(depth int) {
		if indented {
			out.WriteString("\n" + strings.Repeat("  ", depth))
		}
	}

	switch v.kind {
	case jqNull:
		out.WriteString("null")
	case jqFalse:
		out.WriteString("false")
	case jqTrue:
		out.WriteString("true")
	case jqNumber:
		out.WriteString(jqNumberText(v.text))
	case jqString:
		writeJQString(out, v.text)
	case jqArray:
		if len(v.items) == 0 {
			out.WriteString("[]")

			return
		}

		out.WriteString("[")
		for i, item := range v.items {
			if i > 0 {
				out.WriteString(",")
			}

			newline(depth + 1)
			item.write(out, indented, depth+1)
		}

		newline(depth)
		out.WriteString("]")
	case jqObject:
		if len(v.keys) == 0 {
			out.WriteString("{}")

			return
		}

		out.WriteString("{")
		for i, key := range v.keys {
			if i > 0 {
				out.WriteString(",")
			}

			newline(depth + 1)
			writeJQString(out, key)
			out.WriteString(":")
			if indented {
				out.WriteString(" ")
			}

			v.items[i].write(out, indented, depth+1)
		}

		newline(depth)
		out.WriteString("}")
	}
}

// writeJQString escapes what jq escapes: the quote, the backslash and the control characters,
// DEL among them. Everything else, non-ASCII included, goes out as it is.
func writeJQString(out *strings.Builder, text string) {
	out.WriteByte('"')

	for _, r := range text {
		switch r {
		case '"':
			out.WriteString(`\"`)
		case '\\':
			out.WriteString(`\\`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 || r == 0x7f {
				out.WriteString(`\u00`)
				out.WriteString(strconv.FormatInt(int64(r)>>4, 16))
				out.WriteString(strconv.FormatInt(int64(r)&0xf, 16))

				continue
			}

			out.WriteRune(r)
		}
	}

	out.WriteByte('"')
}

// jqNumberText is a number literal as jq 1.7 prints it: the decimal's scientific string, which
// keeps `1.50` and `100` as written and turns `1e2` into `1E+2` and `12e-3` into `0.012`.
func jqNumberText(literal string) string {
	sign := ""
	if strings.HasPrefix(literal, "-") {
		sign, literal = "-", literal[1:]
	}

	mantissa, exponent := literal, 0
	if at := strings.IndexAny(literal, "eE"); at >= 0 {
		mantissa = literal[:at]

		parsed, err := strconv.Atoi(strings.TrimPrefix(literal[at+1:], "+"))
		if err != nil {
			return sign + literal
		}

		exponent = parsed
	}

	coefficient := mantissa
	if at := strings.IndexByte(mantissa, '.'); at >= 0 {
		coefficient = mantissa[:at] + mantissa[at+1:]
		exponent -= len(mantissa) - at - 1
	}

	coefficient = strings.TrimLeft(coefficient, "0")
	if coefficient == "" {
		coefficient = "0"
	}

	adjusted := exponent + len(coefficient) - 1

	if exponent <= 0 && adjusted >= -6 {
		if exponent == 0 {
			return sign + coefficient
		}

		point := len(coefficient) + exponent
		if point > 0 {
			return sign + coefficient[:point] + "." + coefficient[point:]
		}

		return sign + "0." + strings.Repeat("0", -point) + coefficient
	}

	text := coefficient[:1]
	if len(coefficient) > 1 {
		text += "." + coefficient[1:]
	}

	if adjusted >= 0 {
		return sign + text + "E+" + strconv.Itoa(adjusted)
	}

	return sign + text + "E" + strconv.Itoa(adjusted)
}
