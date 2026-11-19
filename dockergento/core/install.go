package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

//
// Installing Magento in a project's containers: what console/commands/install.sh decided before
// and around `setup:install`, without a container or a file in sight.
//

// InstallOptions is what `install` was asked for.
type InstallOptions struct {
	// UseDefault skips the questions about the store and its admin: what data/config.json holds
	// is what is used.
	UseDefault bool
}

// ParseInstall reads install's options the way its getopts ":u" did, `--use-default` being `-u`:
// it stops at `--` or at the first word that is not an option, and refuses any other letter.
func ParseInstall(args []string, binary string) (InstallOptions, error) {
	options := InstallOptions{}

	refuse := Refusal{
		Kind:    "invalid_argument",
		Code:    2,
		Message: "The command is not correct",
		Hint:    binary + " install [-u|--use-default]",
	}

	for _, argument := range args {
		if argument == "--use-default" {
			options.UseDefault = true

			continue
		}

		if argument == "--" || len(argument) < 2 || argument[0] != '-' {
			break
		}

		for _, letter := range argument[1:] {
			if letter != 'u' {
				return InstallOptions{}, refuse
			}

			options.UseDefault = true
		}
	}

	return options, nil
}

// LockedMagentoVersion is the community edition's version in a composer.lock, or "" — the only
// package install.sh's jq looked for.
func LockedMagentoVersion(lock string) string {
	var contents struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}

	if json.Unmarshal([]byte(lock), &contents) != nil {
		return ""
	}

	for _, item := range contents.Packages {
		if item.Name == "magento/product-community-edition" {
			return item.Version
		}
	}

	return ""
}

// EquivalentFallback is `${MAGENTO_VERSION%-*}`: the release without its patch suffix, which is
// what install.sh used when the version was not looked up in the table.
func EquivalentFallback(version string) string {
	if at := strings.LastIndex(version, "-"); at >= 0 {
		return version[:at]
	}

	return version
}

// LastSupportedVersion is `jq -r 'keys | last'` over equivalent_versions.json: the greatest key in
// byte order, which is what the version question offers.
func LastSupportedVersion(equivalents map[string]string) string {
	keys := make([]string, 0, len(equivalents))
	for key := range equivalents {
		keys = append(keys, key)
	}

	if len(keys) == 0 {
		return ""
	}

	sort.Strings(keys)

	return keys[len(keys)-1]
}

// InstallArguments are the options of `setup:install` that do not come from data/config.json, in
// install.sh's order: the services, the search engine the release needs and the base URL.
func InstallArguments(cacheHost, version, equivalent, domain string) []string {
	arguments := []string{
		"--db-host=db",
		"--backend-frontname=admin",
		"--use-rewrites=1",
		"--db-name=magento",
		"--db-user=magento",
		"--db-password=magento",
		"--session-save=redis",
		"--session-save-redis-host=" + cacheHost,
		"--session-save-redis-db=0",
		"--session-save-redis-disable-locking=1",
		"--cache-backend=redis",
		"--cache-backend-redis-server=" + cacheHost,
		"--cache-backend-redis-db=1",
		"--page-cache=redis",
		"--page-cache-redis-server=" + cacheHost,
		"--page-cache-redis-db=2",
		"--amqp-host=rabbitmq",
		"--amqp-port=5672",
		"--amqp-user=user",
		"--amqp-password=password",
	}

	if !strings.HasPrefix(version, "2.3.") {
		arguments = append(arguments,
			"--elasticsearch-host=search",
			"--elasticsearch-port=9200",
			"--elasticsearch-username=admin",
			"--elasticsearch-password=admin")
	}

	// OpenSearch is required from Magento 2.4.6
	if VersionAtLeast(equivalent, "2.4.6") {
		arguments = append(arguments,
			"--search-engine=opensearch",
			"--opensearch-host=search",
			"--opensearch-port=9200",
			"--opensearch-username=admin",
			"--opensearch-password=admin")
	}

	return append(arguments, "--base-url=https://"+domain+"/", "--base-url-secure=https://"+domain+"/")
}

// InstallQuestions are the settings install asks about, in its order. The password is not one of
// them: it is generated unless data/config.json sets one.
var InstallQuestions = []string{
	"language", "currency", "timezone", "admin-firstname", "admin-lastname", "admin-email", "admin-user",
}

// Setting is one entry of data/config.json, kept in the file's order because that order is the
// order of the options setup:install is given.
type Setting struct {
	Key   string
	Value json.RawMessage
}

// DefaultInstallSettings is what data/config.json holds when install has to create it.
func DefaultInstallSettings() []Setting {
	settings := []Setting{}

	for _, pair := range [][2]string{
		{"language", "es_ES"},
		{"currency", "EUR"},
		{"timezone", "Europe/Madrid"},
		{"admin-firstname", "hiberus"},
		{"admin-lastname", "hiberus"},
		{"admin-email", "noreply@hiberus.com"},
		{"admin-user", "hiberus"},
		{"admin-password", ""},
	} {
		settings = SetInstallSetting(settings, pair[0], pair[1])
	}

	return settings
}

// ReadInstallSettings reads data/config.json keeping the order of its keys.
func ReadInstallSettings(contents string) ([]Setting, error) {
	decoder := json.NewDecoder(strings.NewReader(contents))

	opening, err := decoder.Token()
	if err != nil {
		return nil, err
	}

	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return nil, fmt.Errorf("data/config.json is not a JSON object")
	}

	settings := []Setting{}

	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}

		key, _ := token.(string)

		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}

		settings = append(settings, Setting{Key: key, Value: value})
	}

	if _, err := decoder.Token(); err != nil {
		return nil, err
	}

	return settings, nil
}

// RenderInstallSettings writes the settings the way `jq '.'` printed them.
func RenderInstallSettings(settings []Setting) string {
	if len(settings) == 0 {
		return "{}\n"
	}

	lines := make([]string, 0, len(settings))

	for _, setting := range settings {
		var value bytes.Buffer
		if json.Indent(&value, setting.Value, "  ", "  ") != nil {
			value.Reset()
			value.WriteString("null")
		}

		lines = append(lines, "  "+quote(setting.Key)+": "+value.String())
	}

	return "{\n" + strings.Join(lines, ",\n") + "\n}\n"
}

// InstallSetting is a setting as `jq -r` printed it — "null" when it is null or missing — and
// whether the file has it.
func InstallSetting(settings []Setting, key string) (string, bool) {
	for _, setting := range settings {
		if setting.Key == key {
			return text(setting.Value), true
		}
	}

	return "null", false
}

// SetInstallSetting stores a value as a string, where the key is or at the end.
func SetInstallSetting(settings []Setting, key, value string) []Setting {
	raw := json.RawMessage(quote(value))
	out := make([]Setting, 0, len(settings)+1)
	found := false

	for _, setting := range settings {
		if setting.Key == key {
			setting.Value, found = raw, true
		}

		out = append(out, setting)
	}

	if !found {
		out = append(out, Setting{Key: key, Value: raw})
	}

	return out
}

// InstallSettingArguments are the settings as options of setup:install, with the password set to
// the one given: `. + {"admin-password": $password} | to_entries | map("--" + .key + "=" + .value)`.
func InstallSettingArguments(settings []Setting, password string) []string {
	withPassword := SetInstallSetting(settings, "admin-password", password)
	arguments := make([]string, 0, len(withPassword))

	for _, setting := range withPassword {
		value := text(setting.Value)
		if strings.TrimSpace(string(setting.Value)) == "null" {
			// jq's string + null is the string
			value = ""
		}

		arguments = append(arguments, "--"+setting.Key+"="+value)
	}

	return arguments
}

// text is a JSON value as `jq -r` prints it.
func text(raw json.RawMessage) string {
	if strings.TrimSpace(string(raw)) == "null" {
		return "null"
	}

	var value string
	if json.Unmarshal(raw, &value) == nil {
		return value
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return "null"
	}

	return trimmed
}

// quote is a JSON string as jq writes one: no escaping of <, > and &.
func quote(value string) string {
	var out bytes.Buffer

	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.Encode(value) //nolint:errcheck

	return strings.TrimSuffix(out.String(), "\n")
}

const (
	alphanumerics = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	letters       = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	digits        = "0123456789"
	base32        = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
)

// AdminPassword is twenty characters Magento accepts, as hm_generate_admin_password made them:
// eighteen alphanumerics, then a letter and a digit so that both are certain. No symbols, because
// the password travels as an argument through more than one layer of quoting.
func AdminPassword(random io.Reader) (string, error) {
	body, err := draw(random, alphanumerics, 18)
	if err != nil {
		return "", err
	}

	letter, err := draw(random, letters, 1)
	if err != nil {
		return "", err
	}

	digit, err := draw(random, digits, 1)
	if err != nil {
		return "", err
	}

	return body + letter + digit, nil
}

// OTPSecret is a TOTP secret: thirty-two characters of base32, which is 160 bits.
func OTPSecret(random io.Reader) (string, error) {
	return draw(random, base32, 32)
}

// OTPURI is the registration URI authenticator apps read.
func OTPURI(issuer, user, secret string) string {
	return "otpauth://totp/" + issuer + ":" + user + "?secret=" + secret + "&issuer=" + issuer
}

// draw picks characters from an alphabet without bias, the way `tr -dc` over /dev/urandom does:
// bytes that do not map evenly are thrown away.
func draw(random io.Reader, alphabet string, count int) (string, error) {
	limit := 256 - 256%len(alphabet)
	out := make([]byte, 0, count)
	one := make([]byte, 1)

	for len(out) < count {
		if _, err := io.ReadFull(random, one); err != nil {
			return "", err
		}

		if int(one[0]) >= limit {
			continue
		}

		out = append(out, alphabet[int(one[0])%len(alphabet)])
	}

	return string(out), nil
}
