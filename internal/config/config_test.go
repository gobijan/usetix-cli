package config

import (
	"path/filepath"
	"testing"
)

func TestResolveAPIURLPrecedence(t *testing.T) {
	if got := ResolveAPIURL("https://flag.example", "https://env.example", "https://profile.example"); got != "https://flag.example" {
		t.Fatalf("ResolveAPIURL() = %q", got)
	}
	if got := ResolveAPIURL("", "https://env.example/", "https://profile.example"); got != "https://env.example" {
		t.Fatalf("ResolveAPIURL() = %q", got)
	}
	if got := ResolveAPIURL("", "", "https://profile.example/"); got != "https://profile.example" {
		t.Fatalf("ResolveAPIURL() = %q", got)
	}
}

func TestValidateAPIURL(t *testing.T) {
	valid := []string{"https://app.usetix.io", "http://localhost:3000", "http://127.0.0.1:3000", "http://app.lvh.me:3000", "http://app.localhost:3000"}
	for _, value := range valid {
		if err := ValidateAPIURL(value); err != nil {
			t.Errorf("ValidateAPIURL(%q) = %v", value, err)
		}
	}

	invalid := []string{"http://app.usetix.io", "ftp://app.usetix.io", "https://", "https://user:secret@app.usetix.io", "https://app.usetix.io?token=secret", "https://app.usetix.io/subpath"}
	for _, value := range invalid {
		if err := ValidateAPIURL(value); err == nil {
			t.Errorf("ValidateAPIURL(%q) succeeded", value)
		}
	}
}

func TestDirUsesOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "config")
	got, err := Dir(func(key string) string {
		if key == ConfigDirEnv {
			return want
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Dir() = %q, want %q", got, want)
	}
}
