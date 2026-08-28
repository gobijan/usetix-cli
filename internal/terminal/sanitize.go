package terminal

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
)

func SanitizeLine(value string) string {
	value = ansi.Strip(value)
	value = strings.Map(func(character rune) rune {
		switch {
		case character == '\n' || character == '\r' || character == '\t':
			return ' '
		case unicode.IsControl(character):
			return -1
		case isBidiControl(character):
			return ' '
		default:
			return character
		}
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func isBidiControl(character rune) bool {
	return character == '\u061c' ||
		character == '\u200e' || character == '\u200f' ||
		(character >= '\u202a' && character <= '\u202e') ||
		(character >= '\u2066' && character <= '\u2069')
}
