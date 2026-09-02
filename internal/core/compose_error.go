package core

import (
	"regexp"
	"strings"
	"unicode"
)

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

const composeStderrMaxLen = 220

// FormatComposeStderr summarizes compose provider stderr for TUI status lines.
// Prefers the last useful error lines and skips Podman's external-provider banner.
func FormatComposeStderr(stderr string) string {
	cleaned := ansiEscapeRe.ReplaceAllString(stderr, "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	rawLines := strings.Split(cleaned, "\n")

	var useful []string
	var fallback []string
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fallback = append(fallback, line)
		if isComposeProviderBanner(line) {
			continue
		}
		useful = append(useful, line)
	}

	lines := useful
	if len(lines) == 0 {
		lines = fallback
	}
	if len(lines) == 0 {
		return ""
	}

	const maxLines = 3
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	out := strings.Join(lines, " · ")
	return truncateRunes(out, composeStderrMaxLen)
}

func isComposeProviderBanner(line string) bool {
	lower := strings.ToLower(line)
	return strings.Contains(lower, "executing external compose provider") ||
		(strings.Contains(lower, "podman-compose(1)") && strings.Contains(line, ">>>>"))
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	cut := max - 3
	for cut > 0 && unicode.IsSpace(runes[cut-1]) {
		cut--
	}
	return string(runes[:cut]) + "..."
}
