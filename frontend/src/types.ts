export type EngineType = 'postgres' | 'sqlserver';
export type RuntimeType = 'docker' | 'podman';
export type ContainerStatus = 'READY' | 'STARTING' | 'STOPPED' | 'UNKNOWN';
export type EngineHealth = 'ONLINE' | 'OFFLINE' | 'NOT_INSTALLED';
export type ThemeId = 'zed-dark' | 'night-owl';

export interface AppMetadata {
  project_root: string;
  instances_dir: string;
  postgres_versions: string[];
  sqlserver_versions: string[];
  default_postgres_version: string;
  default_sqlserver_version: string;
  engine_health_interval_ms: number;
}

export interface InstanceDTO {
  id: string;
  name: string;
  engine_type: string;
  runtime: string;
  env_file_path: string;
  container_name: string;
  project_name: string;
  port: number;
  database: string;
  user: string;
  password: string;
  schema: string;
  volume: string;
  version: string;
  status: ContainerStatus;
  cpu_usage: string;
  memory_usage: string;
  memory_limit: string;
  connection_uri: string;
  backend_env: string;
  cli_command: string;
}

export interface EnginesHealthDTO {
  docker: EngineHealth;
  podman: EngineHealth;
}

export interface SaveInstanceRequest {
  is_edit: boolean;
  original_name: string;
  engine: string;
  runtime: string;
  version: string;
  name: string;
  container_name: string;
  port: string;
  database: string;
  password: string;
  memory_limit: string;
}

export interface SaveInstanceResponse {
  name: string;
  env_path: string;
  derived_volume: string;
  source_volume: string;
  volume_changed: boolean;
  needs_restart_prompt: boolean;
  cleanup_old_env_path: string;
}

export interface LogLineEvent {
  stream_id: number;
  instance_name: string;
  line: string;
}

export interface LogClosedEvent {
  stream_id: number;
  instance_name: string;
  error?: string;
}

export type ConfirmState =
  | { kind: 'purge'; inst: InstanceDTO }
  | { kind: 'delete'; inst: InstanceDTO }
  | { kind: 'engine-start-retry'; runtime: string; inst: InstanceDTO }
  | { kind: 'engine-stop'; runtime: string }
  | { kind: 'quit-stop-engine'; runtime: string }
  | {
      kind: 'restart-after-edit';
      oldName: string;
      newName: string;
      oldEnvPath: string;
      volumeChanged: boolean;
      derivedVolume: string;
    };
