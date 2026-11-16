package core

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ContainerStatus represents the current state of a database container.
type ContainerStatus string

const (
	StatusReady    ContainerStatus = "READY"
	StatusStarting ContainerStatus = "STARTING"
	StatusStopped  ContainerStatus = "STOPPED"
	StatusUnknown  ContainerStatus = "UNKNOWN"
)

// DatabaseInstance represents a configured database instance.
type DatabaseInstance struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	EngineType    string            `json:"engine_type"` // "postgres" | "sqlserver"
	Runtime       string            `json:"runtime"`     // "docker" | "podman"
	EnvFilePath   string            `json:"env_file_path"`
	ContainerName string            `json:"container_name"`
	ProjectName   string            `json:"project_name"`
	Port          int               `json:"port"`
	Database      string            `json:"database"`
	User          string            `json:"user"`
	Password      string            `json:"password"`
	Schema        string            `json:"schema"`
	Volume        string            `json:"volume"`
	Version       string            `json:"version"`
	Status        ContainerStatus   `json:"status"`
	CPUUsage      string            `json:"cpu_usage"`
	MemoryUsage   string            `json:"memory_usage"`
	MemoryLimit   string            `json:"memory_limit"`
	RawEnv        map[string]string `json:"raw_env"`
}

// ParseEnvFile parses a .env file located at path and returns a DatabaseInstance.
func ParseEnvFile(filePath string) (*DatabaseInstance, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open env file: %w", err)
	}
	defer file.Close()

	rawEnv := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Strip inline comments if not inside quotes
		if !strings.HasPrefix(val, "\"") && !strings.HasPrefix(val, "'") {
			if commentIdx := strings.Index(val, "#"); commentIdx != -1 {
				val = strings.TrimSpace(val[:commentIdx])
			}
		}

		// Strip surrounding quotes
		if len(val) >= 2 {
			if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
				(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
				val = val[1 : len(val)-1]
			}
		}

		rawEnv[key] = val
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading env file: %w", err)
	}

	baseName := filepath.Base(filePath)
	instanceName := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	if strings.HasPrefix(instanceName, ".") {
		instanceName = strings.TrimPrefix(instanceName, ".")
	}
	if strings.HasPrefix(baseName, ".env.") {
		instanceName = strings.TrimPrefix(baseName, ".env.")
	}

	engine := strings.ToLower(rawEnv["ENGINE"])
	if engine == "" {
		// Heuristic fallback based on keys
		if _, ok := rawEnv["POSTGRES_DB"]; ok {
			engine = "postgres"
		} else if _, ok := rawEnv["SQLSERVER_DB"]; ok || rawEnv["SA_PASSWORD"] != "" {
			engine = "sqlserver"
		} else {
			engine = "postgres"
		}
	}

	runtime := strings.ToLower(rawEnv["RUNTIME"])
	if runtime == "" {
		runtime = "docker"
	}

	containerName := rawEnv["CONTAINER_NAME"]
	if containerName == "" {
		containerName = fmt.Sprintf("%s-%s", engine, instanceName)
	}

	projectName := rawEnv["COMPOSE_PROJECT_NAME"]
	if projectName == "" {
		projectName = containerName
	}

	memLimit := rawEnv["MEMORY_LIMIT"]
	if memLimit == "" {
		if engine == "sqlserver" {
			memLimit = "2G"
		} else {
			memLimit = "512M"
		}
	}

	inst := &DatabaseInstance{
		ID:            instanceName,
		Name:          instanceName,
		EngineType:    engine,
		Runtime:       runtime,
		EnvFilePath:   filePath,
		ContainerName: containerName,
		ProjectName:   projectName,
		Status:        StatusStopped,
		CPUUsage:      "-",
		MemoryUsage:   "-",
		MemoryLimit:   memLimit,
		RawEnv:        rawEnv,
	}

	switch engine {
	case "postgres":
		portStr := rawEnv["POSTGRES_PORT"]
		if portStr == "" {
			portStr = rawEnv["PORT"]
		}
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			port = 5432
		}
		inst.Port = port

		user := rawEnv["POSTGRES_USER"]
		if user == "" {
			user = "postgres"
		}
		inst.User = user
		inst.Password = rawEnv["POSTGRES_PASSWORD"]
		inst.Database = rawEnv["POSTGRES_DB"]

		schema := rawEnv["POSTGRES_SCHEMA"]
		if schema == "" {
			schema = "public"
		}
		inst.Schema = schema
		inst.Version = NormalizePostgresVersion(rawEnv["POSTGRES_VERSION"])
		inst.Volume = rawEnv["POSTGRES_VOLUME"]

	case "sqlserver":
		portStr := rawEnv["SQLSERVER_PORT"]
		if portStr == "" {
			portStr = rawEnv["PORT"]
		}
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			port = 1433
		}
		inst.Port = port

		inst.User = "sa"
		pass := rawEnv["SA_PASSWORD"]
		if pass == "" {
			pass = rawEnv["MSSQL_SA_PASSWORD"]
		}
		inst.Password = pass

		db := rawEnv["SQLSERVER_DB"]
		if db == "" {
			db = rawEnv["MSSQL_DB"]
		}
		inst.Database = db

		schema := rawEnv["SQLSERVER_SCHEMA"]
		if schema == "" {
			schema = "dbo"
		}
		inst.Schema = schema
		inst.Volume = rawEnv["SQLSERVER_VOLUME"]

		ver := rawEnv["SQLSERVER_VERSION"]
		if ver == "" {
			img := rawEnv["SQLSERVER_IMAGE"]
			if strings.Contains(strings.ToLower(img), "azure-sql-edge") {
				ver = SQLServerAzureEdge
			} else {
				ver = SQLServer2022
			}
		}
		inst.Version = NormalizeSQLServerVersion(ver)

		if rawEnv["MEMORY_LIMIT"] == "" {
			if inst.Version == SQLServerAzureEdge {
				inst.MemoryLimit = "1G"
			} else {
				inst.MemoryLimit = "2G"
			}
		}
	}

	return inst, nil
}

// ConnectionURI generates standard database connection string.
func (d *DatabaseInstance) ConnectionURI() string {
	switch d.EngineType {
	case "postgres":
		u := &url.URL{
			Scheme: "postgresql",
			User:   url.UserPassword(d.User, d.Password),
			Host:   fmt.Sprintf("localhost:%d", d.Port),
			Path:   d.Database,
		}
		q := u.Query()
		if d.Schema != "" && d.Schema != "public" {
			q.Set("currentSchema", d.Schema)
		}
		u.RawQuery = q.Encode()
		return u.String()

	case "sqlserver":
		u := &url.URL{
			Scheme: "sqlserver",
			User:   url.UserPassword(d.User, d.Password),
			Host:   fmt.Sprintf("localhost:%d", d.Port),
		}
		q := u.Query()
		if d.Database != "" {
			q.Set("database", d.Database)
		}
		u.RawQuery = q.Encode()
		return u.String()

	default:
		return fmt.Sprintf("localhost:%d", d.Port)
	}
}

// BackendEnvBlock generates a standard multi-variable block ready to paste into application .env files.
func (d *DatabaseInstance) BackendEnvBlock() string {
	uri := d.ConnectionURI()
	return fmt.Sprintf(`DB_ENGINE=%s
DB_HOST=localhost
DB_PORT=%d
DB_USER=%s
DB_PASSWORD=%s
DB_NAME=%s
DB_SCHEMA=%s
DATABASE_URL=%s`, d.EngineType, d.Port, d.User, d.Password, d.Database, d.Schema, uri)
}

// CLICommand generates a direct command line invocation string.
func (d *DatabaseInstance) CLICommand() string {
	switch d.EngineType {
	case "postgres":
		return fmt.Sprintf("psql -h localhost -p %d -U %s -d %s", d.Port, d.User, d.Database)
	case "sqlserver":
		return fmt.Sprintf("sqlcmd -S localhost,%d -U %s -P %s -d %s", d.Port, d.User, d.Password, d.Database)
	default:
		return ""
	}
}

// EngineDefaults returns default prefix, volume prefix, port, password, and memory limit for an engine.
func EngineDefaults(engine string) (prefix, volPrefix, port, pass, mem string) {
	if strings.ToLower(strings.TrimSpace(engine)) == "sqlserver" {
		return "sql", "sqlserver", "1433", "SuperPassword123!", "2G"
	}
	return "pg", "pgdata", "5432", "postgres", "512M"
}

// InstanceFormSpec defines the parameters required to create or update an instance .env file.
type InstanceFormSpec struct {
	Kind          string // "create" | "edit"
	SourceName    string
	SourceEnvPath string
	SourceEngine  string
	SourceVolume  string
	SourceUser    string
	SourceSchema  string
	SourceRawEnv  map[string]string
	WasRunning    bool

	Engine        string
	Runtime       string
	Version       string
	Name          string
	ContainerName string
	Port          string
	Database      string
	Password      string
	MemoryLimit   string
}

// SaveInstanceResult summarizes the outcome of SaveInstanceEnv.
type SaveInstanceResult struct {
	Name               string
	ContainerName      string
	Database           string
	EnvPath            string
	DerivedVolume      string
	VolumeChanged      bool
	NeedsRestartPrompt bool
	CleanupOldEnvPath  string
}

// SaveInstanceEnv writes or updates an instance .env configuration inside instancesDir.
func SaveInstanceEnv(instancesDir string, spec InstanceFormSpec) (SaveInstanceResult, error) {
	name := SanitizeIdent(spec.Name)
	if name == "" {
		return SaveInstanceResult{}, fmt.Errorf("instance name cannot be empty")
	}

	engine := strings.ToLower(strings.TrimSpace(spec.Engine))
	if engine != "sqlserver" {
		engine = "postgres"
	}

	runtimeName := strings.ToLower(strings.TrimSpace(spec.Runtime))
	if runtimeName != "podman" {
		runtimeName = "docker"
	}

	containerName := SanitizeIdent(spec.ContainerName)
	db := SanitizeIdent(spec.Database)
	port := strings.TrimSpace(spec.Port)
	pass := strings.TrimSpace(spec.Password)
	memLimit := strings.TrimSpace(spec.MemoryLimit)
	if memLimit == "" {
		memLimit = "512M"
	}

	var versionForVolume string
	if engine == "postgres" {
		versionForVolume = spec.Version
	}
	volume := DeriveVolumeName(engine, name, versionForVolume)

	var content string
	if engine == "postgres" {
		user := "postgres"
		schema := "public"
		if spec.Kind == "edit" && engine == spec.SourceEngine {
			if strings.TrimSpace(spec.SourceUser) != "" {
				user = spec.SourceUser
			}
			if strings.TrimSpace(spec.SourceSchema) != "" {
				schema = spec.SourceSchema
			}
		}
		version := NormalizePostgresVersion(spec.Version)
		content = fmt.Sprintf(`ENGINE=postgres
RUNTIME=%s

CONTAINER_NAME=%s
COMPOSE_PROJECT_NAME=%s
MEMORY_LIMIT=%s

POSTGRES_PORT=%s
POSTGRES_USER=%s
POSTGRES_PASSWORD=%s
POSTGRES_DB=%s
POSTGRES_SCHEMA=%s
POSTGRES_VERSION=%s
POSTGRES_VOLUME=%s
`, runtimeName, containerName, containerName, memLimit, port, user, pass, db, schema, version, volume)
	} else {
		schema := "dbo"
		if spec.Kind == "edit" && engine == spec.SourceEngine && strings.TrimSpace(spec.SourceSchema) != "" {
			schema = spec.SourceSchema
		}
		version := NormalizeSQLServerVersion(spec.Version)
		image := SQLServerImageForVersion(version)
		content = fmt.Sprintf(`ENGINE=sqlserver
RUNTIME=%s

CONTAINER_NAME=%s
COMPOSE_PROJECT_NAME=%s
MEMORY_LIMIT=%s

SQLSERVER_PORT=%s
SA_PASSWORD=%s
SQLSERVER_DB=%s
SQLSERVER_SCHEMA=%s
SQLSERVER_VOLUME=%s
SQLSERVER_VERSION=%s
SQLSERVER_IMAGE=%s
`, runtimeName, containerName, containerName, memLimit, port, pass, db, schema, volume, version, image)
	}

	if spec.Kind == "edit" && len(spec.SourceRawEnv) > 0 {
		managedKeys := map[string]bool{
			"ENGINE": true, "RUNTIME": true, "CONTAINER_NAME": true, "COMPOSE_PROJECT_NAME": true, "MEMORY_LIMIT": true,
			"POSTGRES_PORT": true, "POSTGRES_USER": true, "POSTGRES_PASSWORD": true, "POSTGRES_DB": true,
			"POSTGRES_SCHEMA": true, "POSTGRES_VERSION": true, "POSTGRES_VOLUME": true,
			"SQLSERVER_PORT": true, "SA_PASSWORD": true, "MSSQL_SA_PASSWORD": true, "SQLSERVER_DB": true, "MSSQL_DB": true,
			"SQLSERVER_SCHEMA": true, "SQLSERVER_VOLUME": true, "SQLSERVER_VERSION": true, "SQLSERVER_IMAGE": true,
		}
		var extras []string
		for k, v := range spec.SourceRawEnv {
			if !managedKeys[k] {
				extras = append(extras, fmt.Sprintf("%s=%s", k, formatEnvValue(v)))
			}
		}
		if len(extras) > 0 {
			sort.Strings(extras)
			content += "\n" + strings.Join(extras, "\n") + "\n"
		}
	}

	if err := os.MkdirAll(instancesDir, 0o755); err != nil {
		return SaveInstanceResult{}, fmt.Errorf("create instances directory: %w", err)
	}

	newPath := filepath.Join(instancesDir, fmt.Sprintf("%s.env", name))
	if err := os.WriteFile(newPath, []byte(content), 0644); err != nil {
		return SaveInstanceResult{}, err
	}

	res := SaveInstanceResult{
		Name:               name,
		ContainerName:      containerName,
		Database:           db,
		EnvPath:            newPath,
		DerivedVolume:      volume,
		VolumeChanged:      spec.Kind == "edit" && spec.SourceVolume != volume,
		NeedsRestartPrompt: spec.Kind == "edit" && spec.WasRunning,
	}

	if spec.Kind == "edit" && spec.SourceEnvPath != "" && newPath != spec.SourceEnvPath {
		if spec.WasRunning {
			res.CleanupOldEnvPath = spec.SourceEnvPath
		} else {
			if err := os.Remove(spec.SourceEnvPath); err != nil && !os.IsNotExist(err) {
				return SaveInstanceResult{}, err
			}
		}
	}

	return res, nil
}

func formatEnvValue(v string) string {
	needsQuotes := strings.Contains(v, "#") || v != strings.TrimSpace(v)
	if !needsQuotes {
		return v
	}
	if !strings.Contains(v, `"`) {
		return `"` + v + `"`
	}
	if !strings.Contains(v, `'`) {
		return `'` + v + `'`
	}
	return v
}
