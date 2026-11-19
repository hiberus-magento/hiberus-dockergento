package core

import (
	"crypto/rand"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

//
// The pure half of `install`: what console/commands/install.sh read, decided and rendered before
// and around `setup:install`, without a container or a file.
//

// install.sh's getopts ":u", after `--use-default` became `-u`: the letter, repeated or alone,
// and the long form; it stops at the first word that is not an option, and refuses anything else.
func TestParseInstall(t *testing.T) {
	for _, accepted := range []struct {
		args []string
		want InstallOptions
	}{
		{nil, InstallOptions{}},
		{[]string{"-u"}, InstallOptions{UseDefault: true}},
		{[]string{"--use-default"}, InstallOptions{UseDefault: true}},
		{[]string{"-uu"}, InstallOptions{UseDefault: true}},
		{[]string{"stray", "-x"}, InstallOptions{}},
		{[]string{"--", "-x"}, InstallOptions{}},
		{[]string{"-u", "stray"}, InstallOptions{UseDefault: true}},
	} {
		got, err := ParseInstall(accepted.args, "hm")
		if err != nil || got != accepted.want {
			t.Errorf("ParseInstall(%q) = %+v, %v; want %+v", accepted.args, got, err, accepted.want)
		}
	}

	for _, refused := range [][]string{{"-x"}, {"-ux"}, {"--force-it"}, {"-u", "-y"}} {
		_, err := ParseInstall(refused, "hm")

		var refusal Refusal
		if !errors.As(err, &refusal) || refusal.Kind != "invalid_argument" || refusal.Code != 2 ||
			refusal.Message != "The command is not correct" || refusal.Hint != "hm install [-u|--use-default]" {
			t.Errorf("ParseInstall(%q) = %#v, want the usage refusal", refused, err)
		}
	}
}

// The jq filter only looked for the community edition.
func TestLockedMagentoVersion(t *testing.T) {
	lock := `{"packages": [{"name": "a/b", "version": "1.0"}, {"name": "magento/product-community-edition", "version": "2.4.7-p1"}]}`
	if got := LockedMagentoVersion(lock); got != "2.4.7-p1" {
		t.Errorf("community lock = %q", got)
	}

	if got := LockedMagentoVersion(`{"packages": [{"name": "magento/product-enterprise-edition", "version": "2.4.7"}]}`); got != "" {
		t.Errorf("enterprise lock = %q, want nothing, as jq found nothing", got)
	}

	if got := LockedMagentoVersion("{}"); got != "" {
		t.Errorf("placeholder lock = %q", got)
	}
}

// `${MAGENTO_VERSION%-*}`: the shortest suffix from a dash.
func TestEquivalentFallback(t *testing.T) {
	for version, want := range map[string]string{"2.4.6-p3": "2.4.6", "2.4.7": "2.4.7", "2.4.8-beta-1": "2.4.8-beta"} {
		if got := EquivalentFallback(version); got != want {
			t.Errorf("EquivalentFallback(%q) = %q, want %q", version, got, want)
		}
	}
}

// `jq -r 'keys | last'`: jq sorts keys by codepoint, not by release.
func TestLastSupportedVersion(t *testing.T) {
	table := map[string]string{"2.4.10": "2.4.9", "2.4.9": "2.4.9", "2.3.7": "2.3.7"}
	if got := LastSupportedVersion(table); got != "2.4.9" {
		t.Errorf("LastSupportedVersion = %q, want 2.4.9", got)
	}

	if got := LastSupportedVersion(nil); got != "" {
		t.Errorf("LastSupportedVersion(empty) = %q", got)
	}
}

var baseInstallArguments = []string{
	"--db-host=db", "--backend-frontname=admin", "--use-rewrites=1", "--db-name=magento",
	"--db-user=magento", "--db-password=magento", "--session-save=redis",
	"--session-save-redis-host=redis", "--session-save-redis-db=0",
	"--session-save-redis-disable-locking=1", "--cache-backend=redis",
	"--cache-backend-redis-server=redis", "--cache-backend-redis-db=1", "--page-cache=redis",
	"--page-cache-redis-server=redis", "--page-cache-redis-db=2", "--amqp-host=rabbitmq",
	"--amqp-port=5672", "--amqp-user=user", "--amqp-password=password",
}

var elasticsearchArguments = []string{
	"--elasticsearch-host=search", "--elasticsearch-port=9200",
	"--elasticsearch-username=admin", "--elasticsearch-password=admin",
}

var opensearchArguments = []string{
	"--search-engine=opensearch", "--opensearch-host=search", "--opensearch-port=9200",
	"--opensearch-username=admin", "--opensearch-password=admin",
}

func joined(parts ...[]string) []string {
	out := []string{}
	for _, part := range parts {
		out = append(out, part...)
	}

	return out
}

// The search engine follows the release: none named for 2.3, Elasticsearch after, and OpenSearch
// too from 2.4.6. The base URL closes it.
func TestInstallArguments(t *testing.T) {
	urls := []string{"--base-url=https://shop.test/", "--base-url-secure=https://shop.test/"}

	for _, each := range []struct {
		version, equivalent string
		want                []string
	}{
		{"2.3.7-p4", "2.3.7", joined(baseInstallArguments, urls)},
		{"2.4.5", "2.4.5", joined(baseInstallArguments, elasticsearchArguments, urls)},
		{"2.4.6-p3", "2.4.6", joined(baseInstallArguments, elasticsearchArguments, opensearchArguments, urls)},
		{"2.4.10", "2.4.10", joined(baseInstallArguments, elasticsearchArguments, opensearchArguments, urls)},
	} {
		got := InstallArguments("redis", each.version, each.equivalent, "shop.test")
		if diff := cmp.Diff(each.want, got); diff != "" {
			t.Errorf("InstallArguments(%s) (-want +got):\n%s", each.version, diff)
		}
	}
}

// Magento 2.4.9+ may name the cache service valkey: every redis host follows it.
func TestInstallArgumentsFollowTheCacheHost(t *testing.T) {
	got := strings.Join(InstallArguments("valkey", "2.3.7", "2.3.7", "x"), " ")

	for _, want := range []string{"--session-save-redis-host=valkey", "--cache-backend-redis-server=valkey", "--page-cache-redis-server=valkey"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing from %s", want, got)
		}
	}

	if strings.Contains(got, "host=redis") || strings.Contains(got, "server=redis") {
		t.Errorf("a redis host left in %s", got)
	}
}

// The file get_config wrote when there was none, as `jq '.'` printed it.
func TestDefaultInstallSettingsRender(t *testing.T) {
	want := `{
  "language": "es_ES",
  "currency": "EUR",
  "timezone": "Europe/Madrid",
  "admin-firstname": "hiberus",
  "admin-lastname": "hiberus",
  "admin-email": "noreply@hiberus.com",
  "admin-user": "hiberus",
  "admin-password": ""
}
`
	if got := RenderInstallSettings(DefaultInstallSettings()); got != want {
		t.Errorf("rendered defaults:\n%s\nwant:\n%s", got, want)
	}

	if got := RenderInstallSettings(nil); got != "{}\n" {
		t.Errorf("empty = %q", got)
	}
}

// What is read keeps its order, which is the order of the options passed to setup:install.
func TestReadInstallSettingsKeepsOrder(t *testing.T) {
	settings, err := ReadInstallSettings(`{"b": "2", "a": "1", "n": null}`)
	if err != nil {
		t.Fatal(err)
	}

	if got := RenderInstallSettings(settings); got != "{\n  \"b\": \"2\",\n  \"a\": \"1\",\n  \"n\": null\n}\n" {
		t.Errorf("round trip = %q", got)
	}

	// jq -r prints null as "null"; a missing key is null as well
	if value, _ := InstallSetting(settings, "n"); value != "null" {
		t.Errorf("null reads as %q", value)
	}

	if value, found := InstallSetting(settings, "missing"); found || value != "null" {
		t.Errorf("missing reads as %q, %v", value, found)
	}

	if _, err := ReadInstallSettings("not json"); err == nil {
		t.Error("a broken file must not read")
	}
}

// `. + {"admin-password": $password} | to_entries | map("--" + .key + "=" + .value)`: an existing
// key keeps its place, a new one goes last.
func TestSetupInstallSettingArguments(t *testing.T) {
	settings := DefaultInstallSettings()

	want := []string{"--language=es_ES", "--currency=EUR", "--timezone=Europe/Madrid",
		"--admin-firstname=hiberus", "--admin-lastname=hiberus", "--admin-email=noreply@hiberus.com",
		"--admin-user=hiberus", "--admin-password=s3cret"}
	if diff := cmp.Diff(want, InstallSettingArguments(settings, "s3cret")); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}

	other, _ := ReadInstallSettings(`{"admin-user": "me"}`)
	if diff := cmp.Diff([]string{"--admin-user=me", "--admin-password=p"}, InstallSettingArguments(other, "p")); diff != "" {
		t.Errorf("appended (-want +got):\n%s", diff)
	}

	// The settings themselves are not changed: the password is never written down
	if value, _ := InstallSetting(settings, "admin-password"); value != "" {
		t.Errorf("password leaked into the settings: %q", value)
	}
}

// A setting answered is stored as a string, in place or at the end.
func TestSetInstallSetting(t *testing.T) {
	settings := SetInstallSetting(DefaultInstallSettings(), "currency", "USD")
	settings = SetInstallSetting(settings, "extra", `quo"te`)

	if value, _ := InstallSetting(settings, "currency"); value != "USD" {
		t.Errorf("currency = %q", value)
	}

	rendered := RenderInstallSettings(settings)
	if !strings.Contains(rendered, "\"currency\": \"USD\",\n  \"timezone\"") ||
		!strings.HasSuffix(rendered, "\"extra\": \"quo\\\"te\"\n}\n") {
		t.Errorf("rendered = %s", rendered)
	}
}

// Twenty alphanumerics, ending in a letter and a digit, as hm_generate_admin_password made them.
func TestAdminPassword(t *testing.T) {
	password, err := AdminPassword(strings.NewReader(strings.Repeat("\x00\x01\x02\x03\x04\x05\x06\x07", 64)))
	if err != nil {
		t.Fatal(err)
	}

	if !regexp.MustCompile(`^[A-Za-z0-9]{18}[A-Za-z][0-9]$`).MatchString(password) {
		t.Errorf("password %q has the wrong shape", password)
	}
}

// Thirty-two characters of base32, and the URI authenticator apps read.
func TestOTPSecretAndURI(t *testing.T) {
	secret, err := OTPSecret(strings.NewReader(strings.Repeat("\x00\x01\x02\x03", 64)))
	if err != nil {
		t.Fatal(err)
	}

	if !regexp.MustCompile(`^[A-Z2-7]{32}$`).MatchString(secret) {
		t.Errorf("secret %q has the wrong shape", secret)
	}

	if got := OTPURI("shop.test", "hiberus", "ABC"); got != "otpauth://totp/shop.test:hiberus?secret=ABC&issuer=shop.test" {
		t.Errorf("uri = %q", got)
	}
}

// Drawn from the system's randomness, no two installs share a password or a secret: twenty in a
// row all differ, as hm_generate_admin_password's and hm_generate_otp_secret's did.
func TestGeneratedCredentialsNeverRepeat(t *testing.T) {
	passwords, secrets := map[string]bool{}, map[string]bool{}

	for range 20 {
		password, err := AdminPassword(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		secret, err := OTPSecret(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}

		if passwords[password] || secrets[secret] {
			t.Fatalf("a credential repeated: %q / %q", password, secret)
		}

		passwords[password], secrets[secret] = true, true
	}
}
