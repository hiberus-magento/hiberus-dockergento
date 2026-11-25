package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestEachKindOfFileNeedsWhatMagentoNeedsAfterItChanges is the rule, one row per kind of file.
func TestEachKindOfFileNeedsWhatMagentoNeedsAfterItChanges(t *testing.T) {
	cases := []struct {
		path string
		want []string
	}{
		// The schema and the data: only setup:upgrade applies them
		{"app/code/Acme/Shop/etc/db_schema.xml", []string{"setup:upgrade"}},
		{"app/code/Acme/Shop/etc/db_schema_whitelist.json", []string{"setup:upgrade"}},
		{"app/code/Acme/Shop/Setup/Patch/Data/AddAttribute.php", []string{"setup:upgrade"}},
		{"app/code/Acme/Shop/Setup/InstallSchema.php", []string{"setup:upgrade"}},
		{"app/code/Acme/Shop/etc/module.xml", []string{"setup:upgrade"}},
		{"app/code/Acme/Shop/registration.php", []string{"setup:upgrade"}},
		{"app/etc/config.php", []string{"setup:upgrade"}},
		{"app/design/frontend/Acme/theme/theme.xml", []string{"setup:upgrade"}},

		// Dependency injection: the generated code is built from it
		{"app/code/Acme/Shop/etc/di.xml", []string{"di:compile", "cache:config"}},
		{"app/code/Acme/Shop/etc/frontend/di.xml", []string{"di:compile", "cache:config"}},
		{"app/code/Acme/Shop/etc/extension_attributes.xml", []string{"di:compile", "cache:config"}},

		// Every other configuration file is merged into the config cache
		{"app/code/Acme/Shop/etc/config.xml", []string{"cache:config"}},
		{"app/code/Acme/Shop/etc/adminhtml/system.xml", []string{"cache:config"}},
		{"app/code/Acme/Shop/etc/frontend/routes.xml", []string{"cache:config"}},
		{"app/code/Acme/Shop/etc/events.xml", []string{"cache:config"}},
		{"app/code/Acme/Shop/etc/webapi.xml", []string{"cache:config"}},
		{"app/code/Acme/Shop/etc/schema.graphqls", []string{"cache:config"}},

		// What a page is built from
		{"app/code/Acme/Shop/view/frontend/layout/catalog_product_view.xml", []string{"cache:layout", "cache:block_html", "cache:full_page"}},
		{"app/design/frontend/Acme/theme/Magento_Catalog/layout/default.xml", []string{"cache:layout", "cache:block_html", "cache:full_page"}},
		{"app/code/Acme/Shop/view/frontend/page_layout/wide.xml", []string{"cache:layout", "cache:block_html", "cache:full_page"}},
		{"app/code/Acme/Shop/view/frontend/templates/banner.phtml", []string{"cache:block_html", "cache:full_page"}},
		{"app/design/frontend/Acme/theme/Magento_Theme/templates/html/header.phtml", []string{"cache:block_html", "cache:full_page"}},
		{"app/code/Acme/Shop/i18n/es_ES.csv", []string{"cache:translate", "cache:full_page"}},
		{"app/design/frontend/Acme/theme/i18n/es_ES.csv", []string{"cache:translate", "cache:full_page"}},

		// Static files: served from pub/static, which no cache type covers
		{"app/code/Acme/Shop/view/frontend/web/js/slider.js", []string{"static", "cache:full_page"}},
		{"app/design/frontend/Acme/theme/web/css/source/_extend.less", []string{"static", "cache:full_page"}},
		{"app/code/Acme/Shop/view/frontend/requirejs-config.js", []string{"static", "cache:full_page"}},

		// A class is read on every request in developer mode
		{"app/code/Acme/Shop/Model/Price.php", nil},
		{"app/code/Acme/Shop/Test/Unit/Model/PriceTest.php", nil},
		{"README.md", nil},
		{"composer.json", nil},
	}

	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, AfterChangeNeeds(tt.path)); diff != "" {
				t.Fatalf("AfterChangeNeeds(%q) (-want +got):\n%s", tt.path, diff)
			}
		})
	}
}

// TestAPathIsReadHoweverItIsWritten: a model sends what it has, which is sometimes ./-relative,
// absolute or with backslashes.
func TestAPathIsReadHoweverItIsWritten(t *testing.T) {
	for _, path := range []string{
		"./app/code/Acme/Shop/etc/di.xml",
		"/var/www/html/app/code/Acme/Shop/etc/di.xml",
		`app\code\Acme\Shop\etc\di.xml`,
		"  app/code/Acme/Shop/etc/di.xml  ",
	} {
		if got := AfterChangeNeeds(path); !cmp.Equal([]string{"di:compile", "cache:config"}, got) {
			t.Errorf("AfterChangeNeeds(%q) = %v, want di:compile and the config cache", path, got)
		}
	}
}

func TestThePlanIsEveryPathsNeedsTogether(t *testing.T) {
	plan := PlanAfterChange([]string{
		"app/code/Acme/Shop/view/frontend/templates/banner.phtml",
		"app/code/Acme/Shop/etc/di.xml",
		"app/code/Acme/Shop/etc/db_schema.xml",
		"app/code/Acme/Shop/view/frontend/layout/default.xml",
		"app/code/Acme/Shop/Model/Price.php",
		"app/code/Acme/Shop/view/frontend/web/js/a.js",
		"",
	})

	want := AfterChangePlan{
		CacheTypes:   []string{"block_html", "config", "full_page", "layout"},
		DiCompile:    true,
		SetupUpgrade: true,
		Static:       true,
		Paths: []AfterChangePath{
			{Path: "app/code/Acme/Shop/view/frontend/templates/banner.phtml", Needs: []string{"cache:block_html", "cache:full_page"}},
			{Path: "app/code/Acme/Shop/etc/di.xml", Needs: []string{"di:compile", "cache:config"}},
			{Path: "app/code/Acme/Shop/etc/db_schema.xml", Needs: []string{"setup:upgrade"}},
			{Path: "app/code/Acme/Shop/view/frontend/layout/default.xml", Needs: []string{"cache:layout", "cache:block_html", "cache:full_page"}},
			{Path: "app/code/Acme/Shop/Model/Price.php", Needs: []string{}},
			{Path: "app/code/Acme/Shop/view/frontend/web/js/a.js", Needs: []string{"static", "cache:full_page"}},
		},
	}

	if diff := cmp.Diff(want, plan); diff != "" {
		t.Fatalf("PlanAfterChange (-want +got):\n%s", diff)
	}
}

func TestNothingChangedNeedsNothing(t *testing.T) {
	plan := PlanAfterChange(nil)

	if len(plan.CacheTypes) != 0 || plan.DiCompile || plan.SetupUpgrade || plan.Static || len(plan.Paths) != 0 {
		t.Fatalf("PlanAfterChange(nil) = %+v, want nothing needed", plan)
	}

	if plan.CacheTypes == nil || plan.Paths == nil {
		t.Fatal("empty lists are nil, want [] in the JSON")
	}
}
