package core

import "testing"

//
// What the instruction means, decided without creating anything: an environment takes minutes and
// a working Docker, and this is where the mistakes were.
//

func TestTheWordsWardenUsesAreNotUnknownOptions(t *testing.T) {
	options, err := ParseSetup([]string{"--clean-install", "--db-dump=/x.sql"}, "hm")
	if err != nil {
		t.Fatalf("ParseSetup(warden spellings) = %v, want no error", err)
	}

	if !options.Install || options.Dump != "/x.sql" {
		t.Fatalf("ParseSetup(--clean-install --db-dump=/x.sql) = %+v, want install and the dump", options)
	}
}

func TestAnOptionNobodyDeclaredIsRefused(t *testing.T) {
	if _, err := ParseSetup([]string{"--tonteria"}, "hm"); err == nil {
		t.Fatal("ParseSetup(--tonteria) = nil, want a refusal")
	}

	if _, err := ParseSetup([]string{"algo"}, "hm"); err == nil {
		t.Fatal("ParseSetup(a loose argument) = nil, want a refusal")
	}
}

// A directory answer becomes a bind mount, so `app/` and `./app` are the same directory to a
// person and two different strings in a generated file.
func TestTheRootIsWrittenTheSameWayHoweverItIsTyped(t *testing.T) {
	for answer, wanted := range map[string]string{
		"app/":     "./app",
		"app":      "./app",
		"./app":    "./app",
		"/abs/app": "/abs/app",
		".":        ".",
		"":         "actual",
	} {
		if got := MagentoRoot(answer, "actual"); got != wanted {
			t.Fatalf("%q: se escribe %q y hace falta %q", answer, got, wanted)
		}
	}
}

// The rule is Compose's own, measured rather than assumed: accented characters are dropped, not
// transliterated. A machine that derived `acentúado` while Compose derived `acentado` is a tool
// and a Compose disagreeing about the name of the project.
func TestTheNameComposeWouldGive(t *testing.T) {
	for dir, wanted := range map[string]string{
		"/code/Shop":      "shop",
		"/code/mi-tienda": "mi-tienda",
		"/code/acentúado": "acentado",
		"/code/--raro":    "raro",
		"/code/...":       "",
	} {
		if got := DeriveProjectName(dir); got != wanted {
			t.Fatalf("DeriveProjectName(%q) = %q, want %q", dir, got, wanted)
		}
	}
}

// A value with a colon is an image somebody named; anything else is a version of one of ours.
func TestWhichImageAServiceRuns(t *testing.T) {
	if got := ServiceImage("php", "8.3-bookworm"); got != "hiberusmagento/php:8.3-bookworm" {
		t.Fatalf("ServiceImage(php, 8.3-bookworm) = %q, want hiberusmagento/php:8.3-bookworm", got)
	}

	if got := ServiceImage("nginx", "hiberusmagento/nginx:1.18"); got != "hiberusmagento/nginx:1.18" {
		t.Fatalf("ServiceImage(nginx, an image somebody named) = %q, want it unchanged", got)
	}
}

// A mail catcher with no image for this Magento is refused rather than written into a file that
// will fail to pull.
func TestAMailCatcherWithNoImageIsRefused(t *testing.T) {
	if _, err := RenderCompose("<mail_service>", map[string]string{"php": "8.3"}, "mailpit"); err == nil {
		t.Fatal("RenderCompose(mail catcher with no image) = nil, want a refusal")
	}

	if _, err := RenderCompose("x", map[string]string{}, "paloma"); err == nil {
		t.Fatal("RenderCompose(a mail catcher that is not one) = nil, want a refusal")
	}
}

// What the template already mounts is not mounted twice: a repeated volume entry is a mount
// declared twice, and Compose takes the last.
func TestWhatIsAlreadyMountedIsNotMountedAgain(t *testing.T) {
	existing := "- ./app:/var/www/html/app:cached\n"

	mounts := BindMounts([]string{"app", "patches.json", "vendor", "pub"}, ".", ":delegated",
		map[string]bool{"pub": true}, existing)

	if mounts != "- ./patches.json:/var/www/html/patches.json:delegated\n" {
		t.Fatalf("BindMounts(what the template already mounts) = %q, want only what it does not", mounts)
	}
}
