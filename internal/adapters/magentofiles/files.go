// Package magentofiles reads what a project's own files say.
//
// From the files and not from the database or a running container, because the first question
// anybody asks is asked before anything is running — and an answer that needs the environment up
// is no answer at all when the environment is what you are trying to understand.
package magentofiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

// Reader reads them.
type Reader struct{}

// Version is the Magento version, from composer.lock.
func (Reader) Version(root, magentoDir string) string {
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}

	contents, err := os.ReadFile(filepath.Join(root, magentoDir, "composer.lock"))
	if err != nil {
		return ""
	}

	if err := json.Unmarshal(contents, &lock); err != nil {
		return ""
	}

	for _, item := range lock.Packages {
		if item.Name == "magento/product-community-edition" || item.Name == "magento/product-enterprise-edition" {
			return item.Version
		}
	}

	return ""
}

var (
	modePattern  = regexp.MustCompile(`'MAGE_MODE'\s*=>\s*'([^']*)'`)
	adminPattern = regexp.MustCompile(`'frontName'\s*=>\s*'([^']*)'`)
)

// Mode is the deploy mode, from app/etc/env.php.
func (r Reader) Mode(root, magentoDir string) string {
	return first(modePattern, r.env(root, magentoDir))
}

// AdminPath is the admin's front name, and it is not always "admin": Magento generates a random
// one on install unless told otherwise, and a project that has one is a project where /admin is a
// 404.
//
// The database can override it further, through admin/url/use_custom_path. Reading that would
// mean a running database and a query, which is not a price worth paying to build a URL — and
// env.php is what `bin/magento info:adminuri` reports for every project that has not done it.
func (r Reader) AdminPath(root, magentoDir string) string {
	if path := first(adminPattern, r.env(root, magentoDir)); path != "" {
		return path
	}

	return "admin"
}

func (Reader) env(root, magentoDir string) string {
	contents, err := os.ReadFile(filepath.Join(root, magentoDir, "app", "etc", "env.php"))
	if err != nil {
		return ""
	}

	return string(contents)
}

func first(pattern *regexp.Regexp, contents string) string {
	match := pattern.FindStringSubmatch(contents)
	if len(match) < 2 {
		return ""
	}

	return match[1]
}
