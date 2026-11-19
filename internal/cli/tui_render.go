package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//
// Presentation for the fleet dashboard, ported from console/tasks/tui_render.sh.
//
// Pure on purpose: every function here takes the JSON the binary printed and returns lines, so the
// layout is tested like any other function and only the drawing needs a terminal. The payloads are
// read the way the shell read them with jq: as a stream of documents, because a command that
// failed had `{}` appended to whatever it printed, and a document that does not parse ends the
// stream without taking the ones before it down.
//

// dashCut cuts from the right and marks the cut.
func dashCut(text string, width int) string {
	runes := []rune(text)

	switch {
	case len(runes) <= width:
		return text
	case width <= 0:
		return ""
	case width == 1:
		return string(runes[:1])
	}

	return string(runes[:width-1]) + "~"
}

// dashCutPath cuts from the left: the end of a path is what identifies it.
func dashCutPath(path string, width int) string {
	runes := []rune(path)

	switch {
	case len(runes) <= width:
		return path
	case width <= 0:
		return ""
	case width == 1:
		return string(runes[len(runes)-1:])
	}

	return "~" + string(runes[len(runes)-width+1:])
}

// dashPad is printf's %-*s, counted in characters.
func dashPad(text string, width int) string {
	if missing := width - len([]rune(text)); missing > 0 {
		return text + strings.Repeat(" ", missing)
	}

	return text
}

// dashFleetColumns are the column widths for a terminal width. The path gives way first, and below
// a certain width the branch goes too, rather than becoming an unreadable stub.
func dashFleetColumns(width int) (name, status, services, branch, path int) {
	name, status, services, branch = 22, 9, 5, 18
	path = width - name - status - services - branch - 6

	if path < 12 {
		path = 0
		branch = width - name - status - services - 5

		if branch < 8 {
			branch = 0
		}
	}

	return name, status, services, branch, path
}

// dashFleetHeader is the table's header, aligned with the rows.
func dashFleetHeader(width int) string {
	name, status, services, branch, path := dashFleetColumns(width)

	line := dashPad("PROJECT", name) + " " + dashPad("STATUS", status) + " " + dashPad("UP", services)

	if branch > 0 {
		line += " " + dashPad("BRANCH", branch)
	}

	if path > 0 {
		line += " PATH"
	}

	return line
}

// dashEnvironments are the environments of a `list --json` payload, every document of it.
func dashEnvironments(payload string) []map[string]any {
	environments := []map[string]any{}

	for _, document := range dashDocuments(payload) {
		list, _ := dashAlternative(dashAt(document, "data", "environments"), dashAt(document, "environments")).([]any)

		for _, entry := range list {
			environment, ok := entry.(map[string]any)
			if !ok {
				environment = map[string]any{}
			}

			environments = append(environments, environment)
		}
	}

	return environments
}

// dashFleetRows is one line per environment. An environment with no name has no line.
func dashFleetRows(payload string, width int) []string {
	return dashRowsOf(dashEnvironments(payload), width)
}

func dashRowsOf(environments []map[string]any, width int) []string {
	nameWidth, statusWidth, servicesWidth, branchWidth, pathWidth := dashFleetColumns(width)
	rows := []string{}

	for _, environment := range environments {
		name := dashJoined(environment["name"])
		if name == "" {
			continue
		}

		services := dashText(dashAt(environment, "containers", "running")) + "/" +
			dashText(dashAt(environment, "containers", "total"))

		// The flags go after the name is cut and are never cut themselves: being a worktree or
		// an orphan is the part of the column worth keeping
		flags := ""
		if worktree := dashJoined(environment["worktree"]); worktree != "" {
			flags = " [" + worktree + "]"
		}

		if dashText(environment["orphan"]) == "true" {
			flags += " !"
		}

		room := nameWidth - len([]rune(flags))
		if room < 4 {
			room = 4
		}

		label := dashCut(dashCut(name, room)+flags, nameWidth)
		line := dashPad(label, nameWidth) + " " + dashPad(dashJoined(environment["status"]), statusWidth) +
			" " + dashPad(services, servicesWidth)

		if branchWidth > 0 {
			branch := dashJoined(environment["branch"])
			if branch == "" {
				branch = "-"
			}

			line += " " + dashPad(dashCut(branch, branchWidth), branchWidth)
		}

		if pathWidth > 0 {
			line += " " + dashCutPath(dashJoined(environment["root"]), pathWidth)
		}

		rows = append(rows, line)
	}

	return rows
}

// dashDoctorLines are the warnings and errors of a diagnosis, errors first.
func dashDoctorLines(payload string, width int) []string {
	lines := []string{}

	for _, document := range dashDocuments(payload) {
		checks, _ := dashAlternative(dashAt(document, "data", "checks"), dashAt(document, "checks")).([]any)

		problems := []map[string]any{}
		for _, entry := range checks {
			check, ok := entry.(map[string]any)
			if ok && check["severity"] != "ok" {
				problems = append(problems, check)
			}
		}

		sort.SliceStable(problems, func(a, b int) bool {
			return problems[a]["severity"] == "error" && problems[b]["severity"] != "error"
		})

		for _, check := range problems {
			line := strings.ToUpper(dashJoined(check["severity"])) + "  " + dashJoined(check["id"]) + "  " +
				dashJoined(check["message"])

			lines = append(lines, dashCut(line, width))
		}
	}

	return lines
}

// dashDescription is what the detail reads of a `describe --json` document.
type dashDescription struct {
	Project struct {
		Name   any             `json:"name"`
		Status any             `json:"status"`
		Domain any             `json:"domain"`
		Root   any             `json:"root"`
		URLs   json.RawMessage `json:"urls"`
	} `json:"project"`
	Magento struct {
		Version any `json:"version"`
		Mode    any `json:"mode"`
	} `json:"magento"`
	Services []struct {
		Name  any `json:"name"`
		State any `json:"state"`
		Image any `json:"image"`
	} `json:"services"`
}

// dashDetailLines is what defines one environment. A payload with nothing readable in it is one
// empty line, as it was in the shell, where an empty answer still read as one line.
func dashDetailLines(payload string, width int) []string {
	lines := []string{}

	for _, raw := range dashRawDocuments(payload) {
		var top map[string]json.RawMessage
		if json.Unmarshal(raw, &top) != nil {
			break
		}

		body := raw
		if data, ok := top["data"]; ok && string(data) != "null" && string(data) != "false" {
			body = data
		}

		var d dashDescription
		if json.Unmarshal(body, &d) != nil {
			break
		}

		domain := dashOr(d.Project.Domain, "")
		if domain == "" {
			domain = "-"
		}

		magento := dashOr(d.Magento.Version, "")
		if magento == "" {
			magento = "unknown"
		}

		if mode := dashOr(d.Magento.Mode, ""); mode != "" {
			magento += "  (" + mode + ")"
		}

		described := []string{
			"Project    " + dashOr(d.Project.Name, ""),
			"Status     " + dashOr(d.Project.Status, ""),
			"Domain     " + domain,
			"Magento    " + magento,
			"Root       " + dashOr(d.Project.Root, ""),
			"",
			"URLs",
		}

		for _, pair := range dashPairs(d.Project.URLs) {
			if pair[1] != "" {
				described = append(described, "  "+dashFirst(pair[0]+"         ", 10)+pair[1])
			}
		}

		described = append(described, "", "Services")

		for _, service := range d.Services {
			described = append(described, "  "+dashFirst(dashJoined(service.Name)+"            ", 13)+
				dashFirst(dashJoined(service.State)+"        ", 10)+dashJoined(service.Image))
		}

		for _, line := range described {
			lines = append(lines, dashCut(line, width))
		}
	}

	if len(lines) == 0 {
		return []string{""}
	}

	return lines
}

// ------------------------------------------------------------------ reading jq's way

func dashRawDocuments(payload string) []json.RawMessage {
	decoder := json.NewDecoder(strings.NewReader(payload))
	documents := []json.RawMessage{}

	for {
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil {
			return documents
		}

		documents = append(documents, raw)
	}
}

func dashDocuments(payload string) []any {
	documents := []any{}

	for _, raw := range dashRawDocuments(payload) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()

		var document any
		if decoder.Decode(&document) == nil {
			documents = append(documents, document)
		}
	}

	return documents
}

// dashAt is a path into a document, nil wherever it does not lead.
func dashAt(value any, path ...string) any {
	for _, key := range path {
		object, ok := value.(map[string]any)
		if !ok {
			return nil
		}

		value = object[key]
	}

	return value
}

// dashAlternative is jq's `//`: the first that is neither null nor false.
func dashAlternative(values ...any) any {
	for _, value := range values {
		if value != nil && value != false {
			return value
		}
	}

	return nil
}

// dashOr is `(. // fallback)` written out as text.
func dashOr(value any, fallback string) string {
	if value == nil || value == false {
		return fallback
	}

	return dashText(value)
}

// dashText is jq's tostring.
func dashText(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return typed
	case bool, json.Number:
		return fmt.Sprint(typed)
	case float64:
		return fmt.Sprint(typed)
	}

	encoded, _ := json.Marshal(value)

	return string(encoded)
}

// dashJoined is a value as jq's join writes it: null is nothing.
func dashJoined(value any) string {
	if value == nil {
		return ""
	}

	return dashText(value)
}

// dashFirst is jq's [0:n] on a string.
func dashFirst(text string, n int) string {
	runes := []rune(text)
	if len(runes) > n {
		return string(runes[:n])
	}

	return text
}

// dashPairs are an object's entries in the order they were written, which is the order jq's
// to_entries gives and the order the URLs are listed in.
func dashPairs(raw json.RawMessage) [][2]string {
	pairs := [][2]string{}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return pairs
	}

	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return pairs
		}

		var value any
		if decoder.Decode(&value) != nil {
			return pairs
		}

		name, _ := key.(string)
		pairs = append(pairs, [2]string{name, dashJoined(value)})
	}

	return pairs
}
