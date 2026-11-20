package cli

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

//
// What bash's printf does to text, for the pages ported from the shell that have to come out byte
// for byte the same.
//
// Two places matter. A value printed through `%b` has its backslash escapes expanded, which is
// what every print_* helper of console/components/print_message.sh does with its argument. And a
// value pasted into the format string itself — print_help.sh prints a command's description as
// `printf "%1s$description\n"` — is a format: its escapes are expanded and its `%` directives are
// directives, with no argument left to fill them.
//

// expandB is bash's `printf %b` of one argument, and whether a `\c` asked for the output to stop
// there.
func expandB(text string) (string, bool) {
	var out strings.Builder

	for i := 0; i < len(text); i++ {
		if text[i] != '\\' || i+1 == len(text) {
			out.WriteByte(text[i])

			continue
		}

		i++

		switch c := text[i]; c {
		case 'c':
			return out.String(), true
		case '0':
			value, used := octal(text[i+1:], 3)
			out.WriteByte(byte(value))
			i += used
		case 'x', 'u', 'U':
			if !hexEscape(&out, text, &i) {
				out.WriteByte('\\')
				out.WriteByte(c)
			}
		default:
			if expanded, ok := simpleEscape(c); ok {
				out.WriteByte(expanded)
			} else {
				out.WriteByte('\\')
				out.WriteByte(c)
			}
		}
	}

	return out.String(), false
}

// bashB is expandB when nothing after a `\c` matters.
func bashB(text string) string {
	expanded, _ := expandB(text)

	return expanded
}

// bashFormat is bash's printf of a format with no arguments left: escapes expanded, `%%` a percent
// sign, and every other directive filled with what an absent argument is — nothing for a string,
// zero for a number — padded to its width.
func bashFormat(format string) string {
	var out strings.Builder

	for i := 0; i < len(format); i++ {
		switch format[i] {
		case '\\':
			if i+1 == len(format) {
				out.WriteByte('\\')

				continue
			}

			i++

			switch c := format[i]; {
			case c >= '0' && c <= '7':
				value, used := octal(format[i:], 3)
				out.WriteByte(byte(value))
				i += used - 1
			case c == 'x' || c == 'u' || c == 'U':
				if !hexEscape(&out, format, &i) {
					out.WriteByte('\\')
					out.WriteByte(c)
				}
			case c == '"' || c == '\'' || c == '?':
				out.WriteByte(c)
			default:
				if expanded, ok := simpleEscape(c); ok {
					out.WriteByte(expanded)
				} else {
					out.WriteByte('\\')
					out.WriteByte(c)
				}
			}
		case '%':
			used := directive(&out, format[i+1:])
			i += used
		default:
			out.WriteByte(format[i])
		}
	}

	return out.String()
}

// directive writes what one conversion with no argument prints and returns how much of the format
// it took, after the `%`.
func directive(out *strings.Builder, rest string) int {
	if rest == "" {
		out.WriteByte('%')

		return 0
	}

	if rest[0] == '%' {
		out.WriteByte('%')

		return 1
	}

	i := 0
	left := false

	for i < len(rest) && strings.IndexByte("-+ #0", rest[i]) >= 0 {
		left = left || rest[i] == '-'
		i++
	}

	start := i
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}

	width, _ := strconv.Atoi(rest[start:i])

	if i < len(rest) && rest[i] == '.' {
		i++
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
	}

	if i == len(rest) {
		out.WriteByte('%')
		out.WriteString(rest)

		return len(rest)
	}

	filled := ""

	switch rest[i] {
	case 's', 'b', 'q', 'c':
	case 'd', 'i', 'o', 'u', 'x', 'X':
		filled = "0"
	case 'e', 'E', 'f', 'F', 'g', 'G', 'a', 'A':
		filled = "0.000000"
	default:
		out.WriteByte('%')
		out.WriteString(rest[:i+1])

		return i + 1
	}

	out.WriteString(pad(filled, width, left))

	return i + 1
}

// pad is printf's width: counted in bytes, as bash counts it.
func pad(text string, width int, left bool) string {
	if len(text) >= width {
		return text
	}

	fill := strings.Repeat(" ", width-len(text))
	if left {
		return text + fill
	}

	return fill + text
}

func simpleEscape(c byte) (byte, bool) {
	switch c {
	case '\\':
		return '\\', true
	case 'a':
		return '\a', true
	case 'b':
		return '\b', true
	case 'e', 'E':
		return 0x1b, true
	case 'f':
		return '\f', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case 'v':
		return '\v', true
	}

	return 0, false
}

// octal reads up to limit octal digits.
func octal(text string, limit int) (int, int) {
	value, used := 0, 0

	for used < limit && used < len(text) && text[used] >= '0' && text[used] <= '7' {
		value = value*8 + int(text[used]-'0')
		used++
	}

	return value & 0xff, used
}

// hexEscape expands \xHH, \uHHHH and \UHHHHHHHH with text[*at] on the letter, and reports whether
// there was a digit to expand.
func hexEscape(out *strings.Builder, text string, at *int) bool {
	letter := text[*at]
	limit := map[byte]int{'x': 2, 'u': 4, 'U': 8}[letter]

	value, used := 0, 0
	for used < limit && *at+1+used < len(text) {
		digit, err := strconv.ParseUint(string(text[*at+1+used]), 16, 8)
		if err != nil {
			break
		}

		value = value*16 + int(digit)
		used++
	}

	if used == 0 {
		return false
	}

	if letter == 'x' {
		out.WriteByte(byte(value))
	} else {
		var buffer [utf8.UTFMax]byte
		out.Write(buffer[:utf8.EncodeRune(buffer[:], rune(value))])
	}

	*at += used

	return true
}
