package core

import (
	"path"
	"sort"
	"strings"
)

// AfterChangePlan is what a set of changed files needs before Magento shows them: the cache types
// to clean, whether the generated code has to be compiled again, whether the schema and data
// patches have to be applied, and whether static files are involved.
type AfterChangePlan struct {
	CacheTypes   []string          `json:"cache_types"`
	DiCompile    bool              `json:"di_compile"`
	SetupUpgrade bool              `json:"setup_upgrade"`
	Static       bool              `json:"static"`
	Paths        []AfterChangePath `json:"paths"`
}

// AfterChangePath is one changed file and what it needs, in the words of AfterChangeNeeds.
type AfterChangePath struct {
	Path  string   `json:"path"`
	Needs []string `json:"needs"`
}

var (
	pageCaches   = []string{"cache:layout", "cache:block_html", "cache:full_page"}
	configCache  = []string{"cache:config"}
	diCompile    = []string{"di:compile", "cache:config"}
	setupUpgrade = []string{"setup:upgrade"}
)

// AfterChangeNeeds is what one changed file needs, by what kind of file it is — read from its
// name and the directories it is in, which is what Magento reads it by:
//
//	etc/db_schema.xml, etc/db_schema_whitelist.json  setup:upgrade
//	Setup/**.php (patches, install and upgrade)       setup:upgrade
//	etc/module.xml, registration.php, theme.xml       setup:upgrade
//	app/etc/config.php                                setup:upgrade
//	etc/**/di.xml, etc/extension_attributes.xml       di:compile, cache:config
//	any other etc/**.xml, etc/schema.graphqls         cache:config
//	*.phtml                                           cache:block_html, cache:full_page
//	layout/*.xml, page_layout/*.xml                   cache:layout, cache:block_html, cache:full_page
//	i18n/*.csv                                        cache:translate, cache:full_page
//	web/**, requirejs-config.js                       static, cache:full_page
//	anything else, a PHP class among them             nothing
//
// The first that matches decides. A class needs nothing because developer mode reads it on every
// request and generates what it needs; a changed constructor or plugin is the exception a path
// cannot show. "static" means pub/static and var/view_preprocessed hold an older copy, which no
// cache type covers.
func AfterChangeNeeds(changed string) []string {
	clean := strings.TrimPrefix(strings.ReplaceAll(strings.TrimSpace(changed), `\`, "/"), "./")
	if clean == "" {
		return nil
	}

	parts := strings.Split(clean, "/")
	base := parts[len(parts)-1]
	directories := parts[:len(parts)-1]
	extension := path.Ext(base)

	in := func(name string) bool {
		for _, directory := range directories {
			if directory == name {
				return true
			}
		}

		return false
	}

	parent := ""
	if len(directories) > 0 {
		parent = directories[len(directories)-1]
	}

	switch {
	case base == "db_schema.xml" || base == "db_schema_whitelist.json":
		return setupUpgrade
	case extension == ".php" && in("Setup") && !in("Test"):
		return setupUpgrade
	case base == "module.xml" && parent == "etc", base == "registration.php", base == "theme.xml":
		return setupUpgrade
	case clean == "app/etc/config.php" || strings.HasSuffix(clean, "/app/etc/config.php"):
		return setupUpgrade
	case (base == "di.xml" || base == "extension_attributes.xml") && in("etc"):
		return diCompile
	case in("etc") && (extension == ".xml" || base == "schema.graphqls"):
		return configCache
	case extension == ".phtml":
		return pageCaches[1:]
	case extension == ".xml" && (in("layout") || in("page_layout")):
		return pageCaches
	case extension == ".csv" && in("i18n"):
		return []string{"cache:translate", "cache:full_page"}
	case in("web") || base == "requirejs-config.js":
		return []string{"static", "cache:full_page"}
	}

	return nil
}

// PlanAfterChange is what a set of changed files needs together: each file's needs, and their
// union. Blank paths are skipped.
func PlanAfterChange(changed []string) AfterChangePlan {
	plan := AfterChangePlan{CacheTypes: []string{}, Paths: []AfterChangePath{}}
	types := map[string]bool{}

	for _, one := range changed {
		if strings.TrimSpace(one) == "" {
			continue
		}

		needs := append([]string{}, AfterChangeNeeds(one)...)

		for _, need := range needs {
			switch {
			case need == "setup:upgrade":
				plan.SetupUpgrade = true
			case need == "di:compile":
				plan.DiCompile = true
			case need == "static":
				plan.Static = true
			case strings.HasPrefix(need, "cache:"):
				types[strings.TrimPrefix(need, "cache:")] = true
			}
		}

		plan.Paths = append(plan.Paths, AfterChangePath{Path: one, Needs: needs})
	}

	for name := range types {
		plan.CacheTypes = append(plan.CacheTypes, name)
	}

	sort.Strings(plan.CacheTypes)

	return plan
}
