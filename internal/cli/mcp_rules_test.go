package cli

import (
	"strings"
	"testing"
)

//
// What database_query lets through and what config_set takes as a path — the rules
// tests/unit/mcp_query_test.sh held over mcp_tools.sh, case for case.
//

func TestOnlyAStatementThatReadsIsAccepted(t *testing.T) {
	accepted := []string{
		"SELECT value FROM core_config_data LIMIT 1",
		"select 1 from dual",
		"   SELECT 1;  ",
		"SHOW TABLES",
		"DESCRIBE sales_order",
		"EXPLAIN SELECT 1 FROM dual",
	}

	for _, sql := range accepted {
		if refusal := mcpQueryRefusal(sql); refusal != "" {
			t.Errorf("%q refused (%s), want it accepted", sql, refusal)
		}
	}

	refused := []string{
		"UPDATE core_config_data SET value = 1",
		"DELETE FROM sales_order",
		"DROP TABLE sales_order",
		"INSERT INTO admin_user VALUES (1)",
		"TRUNCATE sales_order",
		"",
		"   ",
		// A model does not have to be hostile to produce these: a query built by concatenation
		// arrives looking exactly like an attempt at one
		"SELECT 1; DROP TABLE sales_order",
		"/* SELECT */ DROP TABLE sales_order",
		"-- SELECT 1\nDELETE FROM sales_order",
		"SELECT 1 INTO OUTFILE '/tmp/x'",
		"SELECT 1 into dumpfile '/tmp/x'",
		"SELECT LOAD_FILE('/etc/passwd')",
	}

	for _, sql := range refused {
		if mcpQueryRefusal(sql) == "" {
			t.Errorf("%q accepted, want it refused", sql)
		}
	}
}

func TestTheRefusalSaysWhy(t *testing.T) {
	if refusal := mcpQueryRefusal("DROP TABLE x"); !strings.Contains(refusal, "SELECT") {
		t.Errorf("refusal of a write = %q, want it to name what is allowed", refusal)
	}

	if refusal := mcpQueryRefusal("SELECT 1 INTO OUTFILE '/tmp/x'"); !strings.Contains(refusal, "file") {
		t.Errorf("refusal of a file write = %q, want it to name the reason", refusal)
	}
}

func TestAConfigurationPathLooksLikeOne(t *testing.T) {
	for _, path := range []string{"web/secure/use_in_frontend", "dev/js/merge_files"} {
		if !mcpIsConfigPath(path) {
			t.Errorf("%q refused, want it taken as a configuration path", path)
		}
	}

	for _, path := range []string{"", "web", "/web/secure", "web/secure/", "web secure use",
		"web/secure;rm -rf /", "WEB/SECURE/X", "web/secure/$(id)"} {
		if mcpIsConfigPath(path) {
			t.Errorf("%q taken as a configuration path, want it refused", path)
		}
	}
}

func TestTheWriteToolsAreOnlyOfferedWithWrite(t *testing.T) {
	read, write := mcpToolNames(false), mcpToolNames(true)

	if len(read) != 5 || len(write) != 9 {
		t.Fatalf("tools = %d without --write and %d with it, want 5 and 9", len(read), len(write))
	}

	for _, forbidden := range []string{"setup", "upgrade", "composer", "compile", "install", "import", "down"} {
		for _, name := range write {
			if strings.Contains(name, forbidden) {
				t.Errorf("%s is offered; the operations that need a person are not offered at all", name)
			}
		}
	}
}
