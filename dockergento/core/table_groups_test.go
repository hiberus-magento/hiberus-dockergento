package core

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// groupsForTest is a small catalogue with the shapes the real one has: plain names, patterns, and
// a group made of other groups.
func groupsForTest() []TableGroup {
	return []TableGroup{
		{ID: "log", Tables: []string{"report_event", "report_viewed_*"}},
		{ID: "sessions", Tables: []string{"session", "persistent_session"}},
		{ID: "stripped", Tables: []string{"@log", "@sessions", "system_config_snapshot"}},
		{ID: "saas", Tables: []string{"cde_*", "*_sync_batches"}},
	}
}

func TestStructureOnlyPatternsExpandsGroupsNamesAndPatterns(t *testing.T) {
	cases := []struct {
		name  string
		rules []string
		want  []string
	}{
		{name: "nothing", rules: nil, want: []string{}},
		{name: "a group", rules: []string{"@saas"}, want: []string{"cde_*", "*_sync_batches"}},
		{name: "a table name", rules: []string{"my_custom_log"}, want: []string{"my_custom_log"}},
		{name: "a pattern", rules: []string{"tmp_*"}, want: []string{"tmp_*"}},
		{
			name:  "a group of groups, in order",
			rules: []string{"@stripped"},
			want:  []string{"report_event", "report_viewed_*", "session", "persistent_session", "system_config_snapshot"},
		},
		{
			name:  "layers that repeat themselves say each pattern once",
			rules: []string{"@saas", "cde_*", "@log", "@stripped"},
			want:  []string{"cde_*", "*_sync_batches", "report_event", "report_viewed_*", "session", "persistent_session", "system_config_snapshot"},
		},
		{name: "a dollar is a legal table character", rules: []string{"a$b"}, want: []string{"a$b"}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := StructureOnlyPatterns(groupsForTest(), tt.rules)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("patterns (-want +got):\n%s", diff)
			}
		})
	}
}

// TestStructureOnlyPatternsRefusesWhatItCannotPassSafely: the patterns go into a shell script on
// the remote server, so only what a Magento table name is made of gets through — no quote, no
// space, no other glob character than *.
func TestStructureOnlyPatternsRefusesWhatItCannotPassSafely(t *testing.T) {
	cases := []struct {
		name, rule, says string
	}{
		{name: "an unknown group", rule: "@nope", says: `unknown table group "@nope"`},
		{name: "a quote", rule: "a'b", says: `"a'b" is not a table name or pattern`},
		{name: "a space inside", rule: "a b", says: `"a b" is not a table name or pattern`},
		{name: "a question mark", rule: "log_?", says: `"log_?" is not a table name or pattern`},
		{name: "a bracket", rule: "log_[ab]", says: `"log_[ab]" is not a table name or pattern`},
		{name: "a command", rule: "$(reboot)", says: `"$(reboot)" is not a table name or pattern`},
		{name: "a schema-qualified name", rule: "main.log", says: `"main.log" is not a table name or pattern`},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := StructureOnlyPatterns(groupsForTest(), []string{tt.rule})
			if err == nil || !strings.Contains(err.Error(), tt.says) {
				t.Fatalf("error = %v, want it to say %s", err, tt.says)
			}
		})
	}
}

// TestAGroupThatContainsItselfIsRefused rather than expanded forever.
func TestAGroupThatContainsItselfIsRefused(t *testing.T) {
	groups := []TableGroup{{ID: "a", Tables: []string{"@b"}}, {ID: "b", Tables: []string{"@a"}}}

	if _, err := StructureOnlyPatterns(groups, []string{"@a"}); err == nil || !strings.Contains(err.Error(), "contains itself") {
		t.Fatalf("error = %v, want the loop named", err)
	}
}

func TestSplitTableRules(t *testing.T) {
	cases := []struct {
		list string
		want []string
	}{
		{list: "", want: []string{}},
		{list: "@saas", want: []string{"@saas"}},
		{list: "@saas,my_log, tmp_* ,,", want: []string{"@saas", "my_log", "tmp_*"}},
		{list: "@log @sessions", want: []string{"@log", "@sessions"}},
	}

	for _, tt := range cases {
		if diff := cmp.Diff(tt.want, SplitTableRules(tt.list)); diff != "" {
			t.Errorf("SplitTableRules(%q) (-want +got):\n%s", tt.list, diff)
		}
	}
}

// TestTableMatches is the shell's case pattern with * as its only wildcard, which is what the
// remote script applies: the same answer on both sides.
func TestTableMatches(t *testing.T) {
	cases := []struct {
		pattern, table string
		want           bool
	}{
		{pattern: "session", table: "session", want: true},
		{pattern: "session", table: "persistent_session", want: false},
		{pattern: "*_sync_batches", table: "sales_data_exporter_orders_v2_sync_batches", want: true},
		{pattern: "cde_*", table: "cde_products_feed", want: true},
		{pattern: "cde_*", table: "acde_products_feed", want: false},
		{pattern: "*__temp*", table: "catalog_product_index_price__temp1", want: true},
		{pattern: "admin*", table: "admin", want: true},
		{pattern: "*_cl", table: "catalog_product_cl", want: true},
		{pattern: "*_cl", table: "catalog_product_clx", want: false},
	}

	for _, tt := range cases {
		if got := TableMatches(tt.pattern, tt.table); got != tt.want {
			t.Errorf("TableMatches(%q, %q) = %v, want %v", tt.pattern, tt.table, got, tt.want)
		}
	}
}

func TestParseTableGroups(t *testing.T) {
	groups, err := ParseTableGroups([]byte(`{"source": "x", "groups": [{"id": "saas", "description": "d", "tables": ["cde_*"]}]}`))
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]TableGroup{{ID: "saas", Description: "d", Tables: []string{"cde_*"}}}, groups); diff != "" {
		t.Fatalf("groups (-want +got):\n%s", diff)
	}

	if _, err := ParseTableGroups([]byte(`{`)); err == nil {
		t.Fatal("a broken file parsed")
	}
}
