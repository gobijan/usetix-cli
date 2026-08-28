package terminal

import "testing"

func TestSanitizeLine(t *testing.T) {
	input := "\x1b[31mDanger\x1b[0m\nnext\u202Ehidden"
	if got, want := SanitizeLine(input), "Danger next hidden"; got != want {
		t.Fatalf("SanitizeLine() = %q, want %q", got, want)
	}
}
