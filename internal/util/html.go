package util

import (
	"html"
	"regexp"
	"strings"
)

var (
	htmlBrRe    = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlLiRe    = regexp.MustCompile(`(?i)<li[^>]*>`)
	htmlBlockRe = regexp.MustCompile(`(?i)</(p|div|li|ul|ol|h[1-6]|tr|table|blockquote|pre)>`)
	htmlTagRe   = regexp.MustCompile(`<[^>]+>`)
	htmlNlRe    = regexp.MustCompile(`\n{3,}`)
)

// HTMLToText converts the HTML that ADO stores in rich-text fields
// (Description, Repro Steps, Acceptance Criteria) into readable plain text.
func HTMLToText(s string) string {
	if s == "" {
		return ""
	}
	s = htmlBrRe.ReplaceAllString(s, "\n")
	s = htmlLiRe.ReplaceAllString(s, "- ")
	s = htmlBlockRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")

	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	s = strings.Join(lines, "\n")
	s = htmlNlRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
