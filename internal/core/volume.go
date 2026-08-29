package core

import "strings"

const DefaultPostgresVersion = "18"

var PostgresVersions = []string{"14", "15", "16", "17", "18"}

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
	name := strings.TrimSpace(instanceName)
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case "postgres":
		return "pgdata_" + name + "_" + NormalizePostgresVersion(version)
	case "sqlserver":
		return "sqlserver_" + name
	default:
		return "data_" + name
	}
}
