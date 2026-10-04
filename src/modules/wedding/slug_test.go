package wedding

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Samuel & Sarah":        "samuel-sarah",
		"  Ádèlé  ":             "adele",
		"Budi--Santoso!!":       "budi-santoso",
		"日本":                    "",
		"A1 b2":                 "a1-b2",
		strings.Repeat("a", 80): strings.Repeat("a", 60),
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBaseSlug(t *testing.T) {
	if got := baseSlug("Samuel", "Sarah Putri"); got != "samuel-sarah" {
		t.Errorf("baseSlug = %q", got)
	}
	got := baseSlug("日本", "中文")
	if !strings.HasPrefix(got, "wedding-") || ValidateSlug(got) != "" {
		t.Errorf("fallback = %q", got)
	}
}

func TestNextFreeSlug(t *testing.T) {
	if got := nextFreeSlug("a-b", nil); got != "a-b" {
		t.Errorf("got %q", got)
	}
	if got := nextFreeSlug("a-b", []string{"a-b", "a-b-2", "a-b-4"}); got != "a-b-3" {
		t.Errorf("got %q", got)
	}
}

func TestValidateSlug(t *testing.T) {
	valid := []string{"samuel-sarah", "a1", "x-2"}
	invalid := []string{"", "Samuel", "a_b", "a--b", "-a", "a-", "a b", "admin", "w", strings.Repeat("a", 61)}
	for _, s := range valid {
		if msg := ValidateSlug(s); msg != "" {
			t.Errorf("%q harus valid: %s", s, msg)
		}
	}
	for _, s := range invalid {
		if ValidateSlug(s) == "" {
			t.Errorf("%q harus invalid", s)
		}
	}
}
