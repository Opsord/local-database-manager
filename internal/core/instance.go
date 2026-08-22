package core

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ContainerStatus represents the current state of a database container.
type ContainerStatus string

const (
	StatusRunning ContainerStatus = "RUNNING"
	StatusStopped ContainerStatus = "STOPPED"
	StatusUnknown ContainerStatus = "UNKNOWN"
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
	Status        ContainerStatus   `json:"status"`
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

	inst := &DatabaseInstance{
		ID:            instanceName,
		Name:          instanceName,
		EngineType:    engine,
		Runtime:       runtime,
		EnvFilePath:   filePath,
		ContainerName: containerName,
		ProjectName:   projectName,
		Status:        StatusStopped,
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
