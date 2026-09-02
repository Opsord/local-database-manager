package core

import (
	"strings"
	"unicode"
)

const DefaultPostgresVersion = "18"

var PostgresVersions = []string{"14", "15", "16", "17", "18"}

// SanitizeIdent normalizes identifiers to lowercase snake_case
// (spaces/hyphens → _, collapsed; trimmed).
func SanitizeIdent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ToLower(s)
	var b strings.Builder
	prevUnderscore := false
	for _, r := range s {
		if unicode.IsSpace(r) || r == '-' || r == '_' {
			if !prevUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				prevUnderscore = true
			}
			continue
		}
		b.WriteRune(r)
		prevUnderscore = false
	}
	return strings.Trim(b.String(), "_")
}

func NormalizePostgresVersion(v string) string {
	v = strings.TrimSpace(v)
	for _, allowed := range PostgresVersions {
		if v == allowed {
			return v
		}
	}
	return DefaultPostgresVersion
}

func DeriveVolumeName(engine, instanceName, version string) string {
	name := SanitizeIdent(instanceName)
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "postgres":
		return "pgdata_" + name + "_" + NormalizePostgresVersion(version)
	case "sqlserver":
		return "sqlserver_" + name
	default:
		return "data_" + name
	}
}
