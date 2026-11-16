import React, { useEffect, useState } from 'react';
import { ExternalLink, Eye, EyeOff, Lock, Save, Wand2, X } from 'lucide-react';
import type { AppMetadata, InstanceDTO, SaveInstanceRequest } from '../types';
import { backend, extractErrorMessage } from '../lib/backend';

interface WizardDockProps {
  mode: 'create' | 'edit';
  metadata: AppMetadata;
  initialInstance: InstanceDTO | null;
  onSave: (req: SaveInstanceRequest) => Promise<void>;
  onOpenEditor?: (name: string) => void;
  onCancel: () => void;
}

function versionDisplayLabel(engine: string, version: string): string {
  if (engine === 'sqlserver') {
    if (version === '2022') return '2022 (Full Developer)';
    if (version === 'azure-edge') return 'Azure SQL Edge (ARM64)';
  }
  return version;
}

export const WizardDock: React.FC<WizardDockProps> = ({
  mode,
  metadata,
  initialInstance,
  onSave,
  onOpenEditor,
  onCancel,
}) => {
  const isEdit = mode === 'edit' && initialInstance !== null;

  const [engine, setEngine] = useState<string>(
    isEdit ? initialInstance.engine_type : 'postgres'
  );
  const [runtime, setRuntime] = useState<string>(
    isEdit ? initialInstance.runtime : 'docker'
  );
  const [version, setVersion] = useState<string>(
    isEdit
      ? initialInstance.version ||
          (initialInstance.engine_type === 'sqlserver'
            ? metadata.default_sqlserver_version
            : metadata.default_postgres_version)
      : metadata.default_postgres_version
  );

  const [name, setName] = useState<string>(isEdit ? initialInstance.name : '');
  const [containerName, setContainerName] = useState<string>(
    isEdit ? initialInstance.container_name : ''
  );
  const [port, setPort] = useState<string>(
    isEdit ? String(initialInstance.port) : '5432'
  );
  const [database, setDatabase] = useState<string>(
    isEdit ? initialInstance.database : ''
  );
  const [password, setPassword] = useState<string>(
    isEdit ? initialInstance.password : 'postgres'
  );
  const [memoryLimit, setMemoryLimit] = useState<string>(
    isEdit ? initialInstance.memory_limit : '512M'
  );

  const [containerTouched, setContainerTouched] = useState<boolean>(isEdit);
  const [dbTouched, setDbTouched] = useState<boolean>(isEdit);
  const [portTouched, setPortTouched] = useState<boolean>(isEdit);
  const [passTouched, setPassTouched] = useState<boolean>(isEdit);
  const [memTouched, setMemTouched] = useState<boolean>(isEdit);
  const [showPassword, setShowPassword] = useState<boolean>(false);
  const [derivedVolume, setDerivedVolume] = useState<string>(
    isEdit ? initialInstance.volume : ''
  );
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [formError, setFormError] = useState<string | null>(null);

  const availableVersions =
    engine === 'sqlserver' ? metadata.sqlserver_versions : metadata.postgres_versions;

  // Initialize free port on create mode
  useEffect(() => {
    if (isEdit) return;
    let active = true;
    backend
      .suggestFreePort(engine === 'sqlserver' ? 1433 : 5432)
      .then((free) => {
        if (active) setPort(String(free));
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [isEdit]);

  // Live preview derived volume name via Go backend
  useEffect(() => {
    let active = true;
    if (!name.trim()) {
      setDerivedVolume('');
      return;
    }
    backend
      .previewDerivedVolume(engine, name, version)
      .then((vol) => {
        if (active) setDerivedVolume(vol);
      })
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [engine, name, version]);

  const handleEngineSwitch = async (nextEngine: string) => {
    if (nextEngine === engine) return;
    const nextDefaultVer =
      nextEngine === 'sqlserver'
        ? metadata.default_sqlserver_version
        : metadata.default_postgres_version;
    setEngine(nextEngine);
    setVersion(nextDefaultVer);

    if (!isEdit) {
      try {
        const defs = await backend.getEngineDefaults(nextEngine, name, nextDefaultVer);
        if (!containerTouched && defs.container_name) setContainerName(defs.container_name);
        if (!portTouched && defs.port) setPort(defs.port);
        if (!passTouched && defs.password) setPassword(defs.password);
        if (!memTouched && defs.memory_limit) setMemoryLimit(defs.memory_limit);
      } catch {
        if (!portTouched) setPort(nextEngine === 'sqlserver' ? '1433' : '5432');
        if (!passTouched) setPassword(nextEngine === 'sqlserver' ? 'SuperPassword123!' : 'postgres');
        if (!memTouched) setMemoryLimit(nextEngine === 'sqlserver' ? '2G' : '512M');
      }
    }
  };

  const handleVersionChange = (nextVer: string) => {
    setVersion(nextVer);
    if (engine === 'sqlserver') {
      if (nextVer === 'azure-edge' && (memoryLimit === '2G' || !memoryLimit)) {
        setMemoryLimit('1G');
      } else if (nextVer !== 'azure-edge' && (memoryLimit === '1G' || !memoryLimit)) {
        setMemoryLimit('2G');
      }
    }
  };

  const handleNameChange = (raw: string) => {
    setName(raw);
    if (!isEdit) {
      const sanitized = raw
        .trim()
        .toLowerCase()
        .replace(/[\s-]+/g, '_')
        .replace(/_+/g, '_')
        .replace(/^_+|_+$/g, '');
      const prefix = engine === 'sqlserver' ? 'sql' : 'pg';
      if (!containerTouched) {
        setContainerName(sanitized ? `${prefix}_${sanitized}` : '');
      }
      if (!dbTouched) {
        setDatabase(sanitized ? `${sanitized}_db` : '');
      }
    }
  };

  const handleSuggestPort = async () => {
    try {
      const free = await backend.suggestFreePort(engine === 'sqlserver' ? 1433 : 5432);
      setPort(String(free));
    } catch {
      // ignore
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    if (!name.trim()) {
      setFormError('Instance name is required.');
      return;
    }
    const parsedPort = Number(port);
    if (!Number.isInteger(parsedPort) || parsedPort < 1 || parsedPort > 65535) {
      setFormError('Port must be a valid integer between 1 and 65535.');
      return;
    }

    setSubmitting(true);
    try {
      await onSave({
        is_edit: isEdit,
        original_name: isEdit ? initialInstance.name : '',
        engine,
        runtime,
        version,
        name: name.trim(),
        container_name: containerName.trim(),
        port: String(parsedPort),
        database: database.trim(),
        password: password.trim(),
        memory_limit: memoryLimit.trim() || (engine === 'sqlserver' ? '2G' : '512M'),
      });
    } catch (err) {
      setFormError(extractErrorMessage(err));
    } finally {
      setSubmitting(false);
    }
  };

  const memoryHint =
    engine === 'sqlserver' && version !== 'azure-edge'
      ? 'Recommended: 2G min for MSSQL'
      : 'Recommended: 512M - 1G';

  return (
    <section
      className="shrink-0 flex flex-col border-t max-h-[380px]"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderColor: 'var(--border-active)',
      }}
    >
      {/* Header */}
      <div
        className="px-4 py-2 border-b flex items-center justify-between shrink-0"
        style={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-subtle)' }}
      >
        <div className="flex items-center gap-2">
          <span className="text-[12px] font-semibold" style={{ color: 'var(--accent-secondary)' }}>
            {isEdit ? `Edit Instance: ${initialInstance.name}` : 'New Database Instance'}
          </span>
          {derivedVolume && (
            <span
              className="text-[11px] font-mono px-1.5 py-0.5 rounded border"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--accent-teal)',
              }}
            >
              Volume: {derivedVolume}
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {isEdit && onOpenEditor && (
            <button
              type="button"
              onClick={() => onOpenEditor(initialInstance.name)}
              title="Open .env in external editor [o]"
              className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] border hover:brightness-110"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-secondary)',
              }}
            >
              <ExternalLink className="w-3 h-3" />
              <span>External Editor</span>
              <span className="kbd">o</span>
            </button>
          )}
          <button
            type="button"
            onClick={onCancel}
            title="Cancel [Esc]"
            className="p-1 rounded border hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Form body */}
      <form onSubmit={handleSubmit} className="overflow-y-auto p-4 space-y-3">
        {formError && (
          <div
            className="px-3 py-1.5 rounded border text-[12px]"
            style={{
              backgroundColor: 'rgba(224, 108, 117, 0.12)',
              borderColor: 'var(--status-stopped)',
              color: 'var(--status-stopped)',
            }}
          >
            {formError}
          </div>
        )}

        {/* Engine, Runtime, Version */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              1. Engine
            </label>
            <div className="flex gap-1.5">
              {(['postgres', 'sqlserver'] as const).map((eng) => {
                const active = engine === eng;
                return (
                  <button
                    key={eng}
                    type="button"
                    onClick={() => handleEngineSwitch(eng)}
                    className="flex-1 py-1.5 px-2 rounded text-[12px] font-medium border transition-colors"
                    style={{
                      backgroundColor: active ? 'var(--bg-selected)' : 'var(--bg-app)',
                      borderColor: active ? 'var(--border-active)' : 'var(--border-subtle)',
                      color: active ? 'var(--text-primary)' : 'var(--text-muted)',
                    }}
                  >
                    {eng === 'postgres' ? 'Postgres' : 'SQL Server'}
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <label className="flex items-center gap-1 text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              <span>2. Runtime</span>
              {isEdit && (
                <span className="inline-flex items-center gap-0.5 text-[10px]" style={{ color: 'var(--status-starting)' }}>
                  <Lock className="w-2.5 h-2.5" /> locked
                </span>
              )}
            </label>
            <div className="flex gap-1.5">
              {(['docker', 'podman'] as const).map((rt) => {
                const active = runtime === rt;
                return (
                  <button
                    key={rt}
                    type="button"
                    disabled={isEdit}
                    onClick={() => setRuntime(rt)}
                    className="flex-1 py-1.5 px-2 rounded text-[12px] font-medium border transition-colors disabled:opacity-60"
                    style={{
                      backgroundColor: active ? 'var(--bg-selected)' : 'var(--bg-app)',
                      borderColor: active ? 'var(--border-active)' : 'var(--border-subtle)',
                      color: active ? 'var(--text-primary)' : 'var(--text-muted)',
                    }}
                  >
                    {rt === 'docker' ? 'Docker' : 'Podman'}
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              3. Version
            </label>
            <select
              value={version}
              onChange={(e) => handleVersionChange(e.target.value)}
              className="w-full py-1.5 px-2.5 rounded text-[12px] font-mono border outline-none"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-primary)',
              }}
            >
              {availableVersions.map((ver) => (
                <option key={ver} value={ver}>
                  {versionDisplayLabel(engine, ver)}
                </option>
              ))}
            </select>
          </div>
        </div>

        {engine === 'sqlserver' && (
          <div
            className="px-2.5 py-1.5 rounded border text-[11.5px]"
            style={{
              backgroundColor: 'var(--bg-app)',
              borderColor: 'var(--border-subtle)',
              color: version === 'azure-edge' ? 'var(--status-starting)' : 'var(--text-muted)',
            }}
          >
            {version === 'azure-edge'
              ? '⚠ Note: Retired/frozen on ARM64 by Microsoft. No SQL Agent (512M-1G RAM).'
              : '💡 Note: Full SQL Server 2022. Recommended 2G min RAM (Rosetta 2 on Apple Silicon).'}
          </div>
        )}

        {/* Text inputs grid */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              4. Instance Name *
            </label>
            <input
              type="text"
              autoFocus
              value={name}
              onChange={(e) => handleNameChange(e.target.value)}
              placeholder="my_new_instance"
              className="w-full py-1.5 px-2.5 rounded text-[12px] font-mono border outline-none focus:border-[var(--border-active)]"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-primary)',
              }}
            />
          </div>

          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              5. Container Name
            </label>
            <input
              type="text"
              value={containerName}
              onChange={(e) => {
                setContainerTouched(true);
                setContainerName(e.target.value);
              }}
              placeholder={engine === 'sqlserver' ? 'sql_my_instance' : 'pg_my_instance'}
              className="w-full py-1.5 px-2.5 rounded text-[12px] font-mono border outline-none focus:border-[var(--border-active)]"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-primary)',
              }}
            />
          </div>

          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              6. Port *
            </label>
            <div className="flex gap-1">
              <input
                type="text"
                value={port}
                onChange={(e) => {
                  setPortTouched(true);
                  setPort(e.target.value);
                }}
                placeholder="5432"
                className="w-full py-1.5 px-2.5 rounded text-[12px] font-mono border outline-none focus:border-[var(--border-active)]"
                style={{
                  backgroundColor: 'var(--bg-app)',
                  borderColor: 'var(--border-subtle)',
                  color: 'var(--text-primary)',
                }}
              />
              <button
                type="button"
                onClick={handleSuggestPort}
                title="Suggest next available TCP port"
                className="px-2 rounded border hover:brightness-110 shrink-0"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  borderColor: 'var(--border-subtle)',
                  color: 'var(--accent-primary)',
                }}
              >
                <Wand2 className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>

          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              7. Database
            </label>
            <input
              type="text"
              value={database}
              onChange={(e) => {
                setDbTouched(true);
                setDatabase(e.target.value);
              }}
              placeholder="my_new_db"
              className="w-full py-1.5 px-2.5 rounded text-[12px] font-mono border outline-none focus:border-[var(--border-active)]"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-primary)',
              }}
            />
          </div>

          <div>
            <label className="block text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              8. Password
            </label>
            <div className="relative flex items-center">
              <input
                type={showPassword ? 'text' : 'password'}
                value={password}
                onChange={(e) => {
                  setPassTouched(true);
                  setPassword(e.target.value);
                }}
                className="w-full py-1.5 pl-2.5 pr-8 rounded text-[12px] font-mono border outline-none focus:border-[var(--border-active)]"
                style={{
                  backgroundColor: 'var(--bg-app)',
                  borderColor: 'var(--border-subtle)',
                  color: 'var(--text-primary)',
                }}
              />
              <button
                type="button"
                onClick={() => setShowPassword((v) => !v)}
                className="absolute right-2 opacity-70 hover:opacity-100"
                style={{ color: 'var(--text-muted)' }}
                title={showPassword ? 'Hide password' : 'Show password'}
              >
                {showPassword ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
              </button>
            </div>
          </div>

          <div>
            <label className="flex items-center justify-between text-[11px] font-semibold mb-1" style={{ color: 'var(--text-muted)' }}>
              <span>9. Memory Limit</span>
              <span className="font-normal text-[10px]">{memoryHint}</span>
            </label>
            <input
              type="text"
              value={memoryLimit}
              onChange={(e) => {
                setMemTouched(true);
                setMemoryLimit(e.target.value);
              }}
              placeholder="512M"
              className="w-full py-1.5 px-2.5 rounded text-[12px] font-mono border outline-none focus:border-[var(--border-active)]"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-primary)',
              }}
            />
          </div>
        </div>

        {/* Footer actions */}
        <div className="flex items-center justify-end gap-2 pt-1">
          <button
            type="button"
            onClick={onCancel}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] border hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            <span>Cancel</span>
            <span className="kbd">Esc</span>
          </button>
          <button
            type="submit"
            disabled={submitting}
            className="flex items-center gap-1.5 px-3.5 py-1.5 rounded text-[12px] font-semibold border hover:brightness-110 disabled:opacity-50"
            style={{
              backgroundColor: 'var(--badge-bg)',
              borderColor: 'var(--border-active)',
              color: 'var(--accent-primary)',
            }}
          >
            <Save className="w-3.5 h-3.5" />
            <span>{submitting ? 'Saving…' : isEdit ? 'Save Changes' : 'Create Instance'}</span>
          </button>
        </div>
      </form>
    </section>
  );
};
