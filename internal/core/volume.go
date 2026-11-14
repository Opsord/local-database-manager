package core

import (
	"strings"
	"unicode"
)

const (
	DefaultPostgresVersion  = "18"
	SQLServer2022           = "2022"
	SQLServerAzureEdge      = "azure-edge"
	DefaultSQLServerVersion = SQLServer2022
)

var (
	PostgresVersions  = []string{"14", "15", "16", "17", "18"}
	SQLServerVersions = []string{SQLServer2022, SQLServerAzureEdge}
)

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

func NormalizeSQLServerVersion(v string) string {
	v = strings.TrimSpace(v)
	for _, allowed := range SQLServerVersions {
		if v == allowed {
			return v
		}
	}
	return DefaultSQLServerVersion
}

func SQLServerImageForVersion(ver string) string {
	if NormalizeSQLServerVersion(ver) == SQLServerAzureEdge {
		return "mcr.microsoft.com/azure-sql-edge:latest"
	}
	return "mcr.microsoft.com/mssql/server:2022-latest"
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
