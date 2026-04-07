package util

import (
	"strings"
	"unicode"
)

// DisplayWidth returns the visual column width of a string,
// counting wide characters (CJK, fullwidth) as 2 and others as 1.
func DisplayWidth(s string) int {
	w := 0
	for _, r := range s {
		if IsWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// PadRight pads a string with spaces to the given display width.
func PadRight(s string, width int) string {
	pad := width - DisplayWidth(s)
	if pad <= 0 {
		return s
	}
	return s + strings.Repeat(" ", pad)
}

// IsWide returns true if the rune occupies 2 columns in a terminal.
func IsWide(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hangul, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6)
}
