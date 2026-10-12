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

// ParseComposeError analyzes stderr from compose and returns an enriched AppError
// with actionable suggestions for the user.
func ParseComposeError(stderr string, cause error) *AppError {
	summary := FormatComposeStderr(stderr)
	lower := strings.ToLower(stderr)

	if strings.Contains(lower, "port is already allocated") ||
		strings.Contains(lower, "bind: address already in use") ||
		strings.Contains(lower, "ports are not available") {
		return NewAppError(
			ErrCodePortInUse,
			summary,
			"Port conflict: edit instance [Enter -> e] to choose next free port",
			cause,
		)
	}

	if strings.Contains(lower, "daemon is not running") ||
		strings.Contains(lower, "cannot connect to the docker daemon") ||
		strings.Contains(lower, "is the docker daemon running") {
		return NewAppError(
			ErrCodeEngineOffline,
			summary,
			"Engine is offline: start it via [e] Engines menu",
			cause,
		)
	}

	return NewAppError(
		ErrCodeComposeFailed,
		summary,
		"Check container logs [l] or configuration",
		cause,
	)
}
