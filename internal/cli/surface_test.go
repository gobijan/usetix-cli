package cli

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/basecamp/cli/surface"
)

var updateSurface = flag.Bool("update-surface", false, "update the CLI surface snapshot")

func TestSurface(t *testing.T) {
	root, _, err := NewRoot("test", Dependencies{ConfigDirectory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	actual := surface.SnapshotString(root) + "\n"
	path := filepath.Join("..", "..", ".surface")
	if *updateSurface {
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual != string(want) {
		t.Fatalf("CLI surface changed; run go test ./internal/cli -run TestSurface -update-surface\n\nwant:\n%s\ngot:\n%s", want, actual)
	}
}
