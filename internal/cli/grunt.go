package cli

import (
	"fmt"
	"io"
	"strings"
)

// grunt prepares a theme's front-end tooling in the php container and compiles its styles.
//
// Each step is one `bash -c` in the container, the commands the shell implementation ran, and the
// first that fails stops the rest, as its `set -e` did. grunt's watch never returns on its own:
// the command holds the terminal until it is interrupted, as before.
func grunt(args []string, stdout, stderr io.Writer, jsonOutput bool) int {
	if len(args) != 2 || !strings.Contains(args[0], "/") {
		return failure(stderr, jsonOutput, "grunt", exitUsage, "invalid_argument",
			"grunt needs a Magento theme and a locale", binaryName()+" grunt <Vendor/theme> <locale_LOCALE>")
	}

	project, code := codeProjectOr(stderr, jsonOutput, "grunt")
	if code != 0 {
		return code
	}

	theme, locale := args[0], args[1]

	steps := []string{
		`if [ -f "package.json.sample" ] && [ ! -f "package.json" ]; then cp package.json.sample package.json; fi`,
		`if [ -f "grunt-config.json.sample" ] && [ ! -f "grunt-config.json" ]; then cp grunt-config.json.sample grunt-config.json; fi`,
		`if [ -f "Gruntfile.js.sample" ] && [ ! -f "Gruntfile.js" ]; then cp Gruntfile.js.sample Gruntfile.js; fi`,
		`npm install && npm update`,
		fmt.Sprintf(`if [ -d "dev/tools/grunt/configs" ]; then echo "module.exports = {magento: {area: 'frontend', name: '%s', locale: '%s', files: ['css/styles-m', 'css/styles-l'], dsl: 'less' }};" > dev/tools/grunt/configs/local-themes.js; fi`,
			theme, locale),
		`grunt exec:magento && grunt watch:magento`,
	}

	for _, step := range steps {
		if status := inside(project, []string{"bash", "-c", step}, "", stdout, stderr, jsonOutput, "grunt"); status != exitOK {
			return status
		}
	}

	return exitOK
}
