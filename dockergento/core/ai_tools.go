package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

//
// What ai-init, ai-pull and ai-reset share and that needs no disk: the JSON documents they write,
// in the shape jq wrote them, and the rules that decide what is installed.
//

// AIObject is a JSON object that keeps the order of its keys, as jq does. The registration is
// rewritten after every installation, and an entry that moved every time something else was
// installed would be a diff nobody asked for.
type AIObject struct {
	keys   []string
	values map[string]any
}

// NewAIObject is an empty object.
func NewAIObject() *AIObject { return &AIObject{values: map[string]any{}} }

// Set puts a value under a key: in its place when the key is there, last when it is not, which is
// what jq's `.[$k] = v` does.
func (o *AIObject) Set(key string, value any) {
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}

	o.values[key] = value
}

// Get is the value under a key, and whether there is one.
func (o *AIObject) Get(key string) (any, bool) {
	if o == nil {
		return nil, false
	}

	value, ok := o.values[key]

	return value, ok
}

// Delete takes a key out, as jq's `del(.[$k])`.
func (o *AIObject) Delete(key string) {
	if _, ok := o.values[key]; !ok {
		return
	}

	delete(o.values, key)

	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)

			break
		}
	}
}

// Keys is the keys in their order.
func (o *AIObject) Keys() []string {
	if o == nil {
		return nil
	}

	return append([]string(nil), o.keys...)
}

// Len is how many keys there are.
func (o *AIObject) Len() int {
	if o == nil {
		return 0
	}

	return len(o.keys)
}

// ErrAIInvalidJSON is contents jq would not parse.
var ErrAIInvalidJSON = errors.New("invalid JSON")

// ParseAIJSON parses one JSON document, keeping the order of every object's keys: objects are
// *AIObject, arrays []any, numbers json.Number with their literal, and the rest what encoding/json
// makes of them. Anything after the document but white space is invalid, as `jq empty` says.
func ParseAIJSON(contents []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()

	value, err := parseValue(decoder)
	if err != nil {
		return nil, ErrAIInvalidJSON
	}

	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, ErrAIInvalidJSON
	}

	return value, nil
}

func parseValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	switch delimiter := token.(type) {
	case json.Delim:
		switch delimiter {
		case '{':
			object := NewAIObject()

			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}

				value, err := parseValue(decoder)
				if err != nil {
					return nil, err
				}

				object.Set(key.(string), value)
			}

			_, err := decoder.Token()

			return object, err
		case '[':
			items := []any{}

			for decoder.More() {
				value, err := parseValue(decoder)
				if err != nil {
					return nil, err
				}

				items = append(items, value)
			}

			_, err := decoder.Token()

			return items, err
		}

		return nil, ErrAIInvalidJSON
	default:
		return token, nil
	}
}

// EncodeAIJSON is a value as `jq .` prints it: two spaces of indentation, empty containers on one
// line, characters outside ASCII as they are, and a newline at the end — the one `echo` added.
func EncodeAIJSON(value any) []byte {
	var out bytes.Buffer

	encodeValue(&out, value, "")
	out.WriteByte('\n')

	return out.Bytes()
}

func encodeValue(out *bytes.Buffer, value any, indent string) {
	switch typed := value.(type) {
	case *AIObject:
		if typed.Len() == 0 {
			out.WriteString("{}")

			return
		}

		out.WriteString("{\n")

		for i, key := range typed.keys {
			out.WriteString(indent + "  ")
			encodeString(out, key)
			out.WriteString(": ")
			encodeValue(out, typed.values[key], indent+"  ")

			if i < len(typed.keys)-1 {
				out.WriteByte(',')
			}

			out.WriteByte('\n')
		}

		out.WriteString(indent + "}")
	case []any:
		if len(typed) == 0 {
			out.WriteString("[]")

			return
		}

		out.WriteString("[\n")

		for i, item := range typed {
			out.WriteString(indent + "  ")
			encodeValue(out, item, indent+"  ")

			if i < len(typed)-1 {
				out.WriteByte(',')
			}

			out.WriteByte('\n')
		}

		out.WriteString(indent + "]")
	case string:
		encodeString(out, typed)
	case json.Number:
		out.WriteString(typed.String())
	case bool:
		fmt.Fprintf(out, "%t", typed)
	case nil:
		out.WriteString("null")
	default:
		encoded, _ := json.Marshal(typed)
		out.Write(encoded)
	}
}

// encodeString escapes what jq escapes and nothing else.
func encodeString(out *bytes.Buffer, text string) {
	out.WriteByte('"')

	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])
		i += size

		switch {
		case r == '"':
			out.WriteString(`\"`)
		case r == '\\':
			out.WriteString(`\\`)
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\t':
			out.WriteString(`\t`)
		case r == '\r':
			out.WriteString(`\r`)
		case r == '\b':
			out.WriteString(`\b`)
		case r == '\f':
			out.WriteString(`\f`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(out, `\u%04x`, r)
		default:
			out.WriteRune(r)
		}
	}

	out.WriteByte('"')
}

// AIText is a value as `jq -r` prints it, with what jq's `//` falls back to when it is null or
// false.
func AIText(value any, fallback string) string {
	if !present(value) {
		return fallback
	}

	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		return "true"
	default:
		return strings.TrimSuffix(string(EncodeAIJSON(typed)), "\n")
	}
}

// AIJoin is `(. // []) | join(",")` over a configuration's list: what is not a list joins to
// nothing, null items to nothing, and everything else to its text.
func AIJoin(value any) string {
	items, _ := value.([]any)

	texts := make([]string, 0, len(items))
	for _, item := range items {
		texts = append(texts, AIText(item, ""))
	}

	return strings.Join(texts, ",")
}

// AISplit is `split(",") | map(select(length > 0))`: a comma-separated list with the empty ones
// left out.
func AISplit(list string) []any {
	items := []any{}

	for _, item := range strings.Split(list, ",") {
		if item != "" {
			items = append(items, item)
		}
	}

	return items
}

// AITimestamp is `now | strftime("%Y-%m-%d %H:%M:%S")`, which jq formats in UTC.
func AITimestamp(now time.Time) string {
	return now.UTC().Format("2006-01-02 15:04:05")
}

// AIConfiguration is the ai-properties.json that ai-init writes, keys in the order jq built them.
func AIConfiguration(resources, platforms, types string, custom any, now time.Time) *AIObject {
	if custom == nil {
		custom = []any{}
	}

	config := NewAIObject()
	config.Set("resources", AISplit(resources))
	config.Set("platforms", AISplit(platforms))
	config.Set("types", AISplit(types))
	config.Set("custom_repositories", custom)
	config.Set("configured_at", AITimestamp(now))

	return config
}

// AICustomRepository is the entry a custom repository gets in the configuration.
func AICustomRepository(name, url, branch string) *AIObject {
	repository := NewAIObject()
	repository.Set("name", name)
	repository.Set("url", url)
	repository.Set("branch", branch)
	repository.Set("types", []any{})

	return repository
}

// AIRepositoryName is a repository's URL as `basename "$url" .git` names it.
func AIRepositoryName(url string) string {
	name := strings.TrimRight(url, "/")
	if index := strings.LastIndex(name, "/"); index >= 0 {
		name = name[index+1:]
	}

	if name == "" && url != "" {
		return "/"
	}

	if trimmed := strings.TrimSuffix(name, ".git"); trimmed != "" {
		return trimmed
	}

	return name
}

// AISkillTypeMatches is whether a skill is one of the types asked for: named after the type, or
// starting with the type and a dash. The type is a regular expression, as it was in the script's
// `[[ $name =~ ^$type- ]]`, and one that is not a valid expression matches nothing that way.
func AISkillTypeMatches(name string, types []string) bool {
	for _, kind := range types {
		if name == kind {
			return true
		}

		if pattern, err := regexp.Compile("^(?:" + kind + ")-"); err == nil && pattern.MatchString(name) {
			return true
		}
	}

	return false
}

// AIRepository is one entry of the repositories to install from, as the script read each field.
type AIRepository struct {
	Name   string
	URL    string
	Branch string
	Local  bool
}

// ReadAIRepository reads an entry of the merged repository list. ai-init read the URL with no
// fallback, so a repository without one is "null" there; ai-pull read it with `// ""`.
func ReadAIRepository(value any, urlFallback string) AIRepository {
	object, _ := value.(*AIObject)

	field := func(key string) any {
		found, _ := object.Get(key)

		return found
	}

	local, _ := field("local").(bool)

	return AIRepository{
		Name:   AIText(field("name"), "null"),
		URL:    AIText(field("url"), urlFallback),
		Branch: AIText(field("branch"), "main"),
		Local:  local,
	}
}

// ValidAIRepositoryName is whether a repository's name can be the name of the directory it is
// downloaded into: one plain name, never a path that leads somewhere else.
func ValidAIRepositoryName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}

// AIRegistrationEntryFor is what add_registration_entry records for one resource.
func AIRegistrationEntryFor(checksum string, now time.Time, origin, url, branch, version string) *AIObject {
	entry := NewAIObject()
	entry.Set("checksum", checksum)
	entry.Set("installed", AITimestamp(now))
	entry.Set("origin", origin)
	entry.Set("url", url)
	entry.Set("branch", branch)
	entry.Set("version", version)

	return entry
}

// AITruthy is jq's idea of true: anything but null and false.
func AITruthy(value any) bool { return present(value) }
