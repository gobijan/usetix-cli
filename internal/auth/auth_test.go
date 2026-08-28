package auth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTrip(t *testing.T) {
	t.Setenv("USETIX_NO_KEYRING", "1")
	directory := t.TempDir()
	store := NewStore(directory)
	if store.UsingKeyring() {
		t.Fatal("store unexpectedly uses keyring")
	}

	credentials := Credentials{Token: "token-" + t.Name()}
	if err := store.Save("profile:test", credentials); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load("profile:test")
	if err != nil {
		t.Fatal(err)
	}
	if loaded != credentials {
		t.Fatalf("Load() = %#v, want %#v", loaded, credentials)
	}

	info, err := os.Stat(filepath.Join(directory, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials permissions = %o, want 600", info.Mode().Perm())
	}

	if err := store.Delete("profile:test"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("profile:test"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() after delete = %v, want ErrNotFound", err)
	}
}

func TestSaveRejectsEmptyToken(t *testing.T) {
	t.Setenv("USETIX_NO_KEYRING", "1")
	if err := NewStore(t.TempDir()).Save("test", Credentials{}); err == nil {
		t.Fatal("Save() accepted an empty token")
	}
}
