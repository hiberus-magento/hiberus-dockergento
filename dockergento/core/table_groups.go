package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// TableGroup is a named set of tables a database dump can take without their data:
// n98-magerun2's db:dump table groups, which Magento developers already know by name, and hm's
// own. A table is a name or a pattern with *, or @another group.
type TableGroup struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Tables      []string `json:"tables"`
}

// ParseTableGroups reads data/table-groups.json.
func ParseTableGroups(contents []byte) ([]TableGroup, error) {
	var file struct {
		Groups []TableGroup `json:"groups"`
	}

	if err := json.Unmarshal(contents, &file); err != nil {
		return nil, err
	}

	return file.Groups, nil
}

// SplitTableRules is a TRANSFER_DB_STRUCTURE_ONLY or --structure-only list: comma separated, as
// the property is documented, and space separated too, as n98-magerun2 writes its groups.
func SplitTableRules(list string) []string {
	rules := []string{}

	for _, rule := range strings.FieldsFunc(list, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
		if rule = strings.TrimSpace(rule); rule != "" {
			rules = append(rules, rule)
		}
	}

	return rules
}

// tablePattern is what a rule may be made of. The patterns become shell case patterns in the
// script that runs on the remote server, so a quote, a space or any glob character other than *
// would either change what matches or what runs; a Magento table name never needs them.
var tablePattern = regexp.MustCompile(`^[A-Za-z0-9_$*]+$`)

// StructureOnlyPatterns expands rules — @groups, table names and * patterns — into the table
// patterns they stand for, each once, in the order they were first named.
func StructureOnlyPatterns(groups []TableGroup, rules []string) ([]string, error) {
	byID := map[string]TableGroup{}
	for _, group := range groups {
		byID[group.ID] = group
	}

	patterns, seen := []string{}, map[string]bool{}

	var expand func(rules []string, within []string) error

	expand = func(rules []string, within []string) error {
		for _, rule := range rules {
			id, isGroup := strings.CutPrefix(rule, "@")
			if !isGroup {
				if !tablePattern.MatchString(rule) {
					return fmt.Errorf("%q is not a table name or pattern: only letters, digits, _, $ and * are allowed", rule)
				}

				if !seen[rule] {
					seen[rule] = true
					patterns = append(patterns, rule)
				}

				continue
			}

			group, known := byID[id]
			if !known {
				return fmt.Errorf("unknown table group %q", rule)
			}

			for _, outer := range within {
				if outer == id {
					return fmt.Errorf("table group %q contains itself", rule)
				}
			}

			if err := expand(group.Tables, append(within, id)); err != nil {
				return err
			}
		}

		return nil
	}

	if err := expand(rules, nil); err != nil {
		return nil, err
	}

	return patterns, nil
}

// TableMatches is whether table is one pattern names: * stands for any run of characters, and
// nothing else is special — the case pattern the remote script applies to the same names.
func TableMatches(pattern, table string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == table
	}

	rest, found := strings.CutPrefix(table, parts[0])
	if !found {
		return false
	}

	last := parts[len(parts)-1]

	for _, part := range parts[1 : len(parts)-1] {
		at := strings.Index(rest, part)
		if at < 0 {
			return false
		}

		rest = rest[at+len(part):]
	}

	return len(rest) >= len(last) && strings.HasSuffix(rest, last)
}
