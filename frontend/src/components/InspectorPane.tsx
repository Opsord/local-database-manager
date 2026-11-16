import React, { useState } from 'react';
import {
  Check,
  Copy,
  Edit3,
  ExternalLink,
  Flame,
  Play,
  Plus,
  ScrollText,
  Square,
  Trash2,
} from 'lucide-react';
import type { ContainerStatus, InstanceDTO } from '../types';

interface InspectorPaneProps {
  instance: InstanceDTO | null;
  busyAction: string | null;
  onToggleRun: (inst: InstanceDTO) => void;
  onOpenLogs: (inst: InstanceDTO) => void;
  onEdit: (inst: InstanceDTO) => void;
  onOpenEditor: (inst: InstanceDTO) => void;
  onPurge: (inst: InstanceDTO) => void;
  onDelete: (inst: InstanceDTO) => void;
  onCopy: (text: string, label: string) => void;
  onNewInstance: () => void;
}

function statusAppearance(status: ContainerStatus): {
  label: string;
  symbol: string;
  color: string;
} {
  switch (status) {
    case 'READY':
      return { label: 'RUNNING', symbol: '●', color: 'var(--status-ready)' };
    case 'STARTING':
      return { label: 'STARTING', symbol: '◐', color: 'var(--status-starting)' };
    case 'UNKNOWN':
      return { label: 'UNKNOWN', symbol: '?', color: 'var(--status-unknown)' };
    default:
      return { label: 'STOPPED', symbol: '○', color: 'var(--status-stopped)' };
  }
}

function formatVersionDisplay(inst: InstanceDTO): string {
  if (inst.engine_type === 'sqlserver') {
    if (inst.version === 'azure-edge') return 'Azure SQL Edge';
    if (inst.version === '2022') return '2022';
  }
  return inst.version || '-';
}

export const InspectorPane: React.FC<InspectorPaneProps> = ({
  instance,
  busyAction,
  onToggleRun,
  onOpenLogs,
  onEdit,
  onOpenEditor,
  onPurge,
  onDelete,
  onCopy,
  onNewInstance,
}) => {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const handleCopyToken = (key: string, value: string, label: string) => {
    if (!value || value === '-') return;
    onCopy(value, label);
    setCopiedKey(key);
    window.setTimeout(() => {
      setCopiedKey((curr) => (curr === key ? null : curr));
    }, 1800);
  };

  if (!instance) {
    return (
      <div
        className="flex-1 flex flex-col items-center justify-center p-8 text-center"
        style={{ backgroundColor: 'var(--bg-app)' }}
      >
        <p className="text-sm font-medium mb-1" style={{ color: 'var(--text-secondary)' }}>
          Select an instance from the left panel to inspect details.
        </p>
        <p className="text-[12px] mb-4" style={{ color: 'var(--text-muted)' }}>
          Or create a new PostgreSQL / SQL Server container definition.
        </p>
        <button
          type="button"
          onClick={onNewInstance}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded text-[12px] font-medium border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--badge-bg)',
            borderColor: 'var(--border-active)',
            color: 'var(--accent-primary)',
          }}
        >
          <Plus className="w-3.5 h-3.5" />
          <span>New Database Instance [n]</span>
        </button>
      </div>
    );
  }

  const isRunning = instance.status === 'READY' || instance.status === 'STARTING';
  const st = statusAppearance(instance.status);
  const engineLabel = `${instance.engine_type.toUpperCase()} (${instance.runtime.toUpperCase()})`;

  const metadataFields: Array<{ key: string; label: string; value: string; highlight?: boolean }> = [
    { key: 'engine', label: 'Engine', value: engineLabel, highlight: true },
    { key: 'version', label: 'Version', value: formatVersionDisplay(instance) },
    { key: 'container', label: 'Container', value: instance.container_name },
    { key: 'database', label: 'Database', value: instance.database || '-' },
    { key: 'port', label: 'Port', value: String(instance.port), highlight: true },
    { key: 'user', label: 'User', value: instance.user || '-' },
    { key: 'schema', label: 'Schema', value: instance.schema || '-' },
    { key: 'volume', label: 'Volume', value: instance.volume || '-' },
    { key: 'project', label: 'Project', value: instance.project_name || '-' },
    { key: 'password', label: 'Password', value: instance.password || '-' },
  ];

  return (
    <div
      className="flex-1 overflow-y-auto p-4 space-y-4"
      style={{ backgroundColor: 'var(--bg-app)' }}
    >
      {/* Header + Action Toolbar */}
      <div
        className="flex flex-wrap items-center justify-between gap-3 pb-3 border-b"
        style={{ borderColor: 'var(--border-subtle)' }}
      >
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="text-base leading-none" style={{ color: st.color }}>
              {st.symbol}
            </span>
            <h1
              className="text-base font-semibold tracking-tight truncate"
              style={{ color: 'var(--text-primary)' }}
            >
              {instance.name}
            </h1>
            <span
              className="text-[10.5px] font-mono px-1.5 py-0.5 rounded border"
              style={{
                backgroundColor: 'var(--bg-surface)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--accent-primary)',
              }}
            >
              {instance.engine_type}
            </span>
            <span
              className="text-[10.5px] font-mono px-1.5 py-0.5 rounded border"
              style={{
                backgroundColor: 'var(--bg-surface)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--accent-secondary)',
              }}
            >
              {instance.runtime}
            </span>
          </div>
          <p className="text-[11px] font-mono mt-0.5 truncate" style={{ color: 'var(--text-muted)' }}>
            {instance.env_file_path}
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            disabled={busyAction !== null}
            onClick={() => onToggleRun(instance)}
            title="Start or Stop container [Space]"
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] font-medium border transition-colors hover:brightness-110 disabled:opacity-50"
            style={{
              backgroundColor: isRunning ? 'rgba(224, 108, 117, 0.14)' : 'rgba(152, 195, 121, 0.15)',
              borderColor: isRunning ? 'var(--status-stopped)' : 'var(--status-ready)',
              color: isRunning ? 'var(--status-stopped)' : 'var(--status-ready)',
            }}
          >
            {isRunning ? <Square className="w-3 h-3" /> : <Play className="w-3 h-3" />}
            <span>{isRunning ? 'Stop' : 'Start'}</span>
            <span className="kbd">Space</span>
          </button>

          <button
            type="button"
            onClick={() => onOpenLogs(instance)}
            title="View live container logs [l]"
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] border transition-colors hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-surface)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-primary)',
            }}
          >
            <ScrollText className="w-3.5 h-3.5" style={{ color: 'var(--accent-primary)' }} />
            <span>Logs</span>
            <span className="kbd">l</span>
          </button>

          <button
            type="button"
            onClick={() => onEdit(instance)}
            title="Edit instance configuration"
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] border transition-colors hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-surface)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-primary)',
            }}
          >
            <Edit3 className="w-3.5 h-3.5" style={{ color: 'var(--accent-teal)' }} />
            <span>Edit</span>
          </button>

          <button
            type="button"
            onClick={() => onOpenEditor(instance)}
            title="Open .env file in external editor [o]"
            className="flex items-center gap-1 px-2 py-1 rounded text-[11.5px] border transition-colors hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-surface)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            <ExternalLink className="w-3.5 h-3.5" />
            <span>.env</span>
          </button>

          <button
            type="button"
            disabled={busyAction !== null}
            onClick={() => onPurge(instance)}
            title="Purge container and volume (down -v) [d]"
            className="flex items-center gap-1 px-2 py-1 rounded text-[11.5px] border transition-colors hover:brightness-110 disabled:opacity-50"
            style={{
              backgroundColor: 'var(--bg-surface)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--status-starting)',
            }}
          >
            <Flame className="w-3.5 h-3.5" />
            <span>Purge</span>
          </button>

          <button
            type="button"
            disabled={busyAction !== null}
            onClick={() => onDelete(instance)}
            title="Delete instance (purge + remove .env) [D]"
            className="flex items-center gap-1 px-2 py-1 rounded text-[11.5px] border transition-colors hover:brightness-110 disabled:opacity-50"
            style={{
              backgroundColor: 'var(--bg-surface)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--status-stopped)',
            }}
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>Delete</span>
          </button>
        </div>
      </div>

      {/* Live Metrics Strip */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-2.5">
        <div
          onClick={() => handleCopyToken('metric-status', st.label, 'Status')}
          className="px-3 py-2 rounded border cursor-pointer hover:brightness-110 transition-colors"
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
          title="Click to copy status"
        >
          <div className="text-[10.5px] uppercase tracking-wider flex items-center justify-between" style={{ color: 'var(--text-muted)' }}>
            <span>Status</span>
            {copiedKey === 'metric-status' && (
              <span style={{ color: 'var(--accent-teal)' }}>Copied!</span>
            )}
          </div>
          <div className="mt-0.5 text-[12.5px] font-mono font-semibold" style={{ color: st.color }}>
            {st.symbol} {st.label}
          </div>
        </div>

        <div
          onClick={() => handleCopyToken('metric-port', String(instance.port), 'Port')}
          className="px-3 py-2 rounded border cursor-pointer hover:brightness-110 transition-colors"
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
          title="Click to copy port"
        >
          <div className="text-[10.5px] uppercase tracking-wider flex items-center justify-between" style={{ color: 'var(--text-muted)' }}>
            <span>Port</span>
            {copiedKey === 'metric-port' && (
              <span style={{ color: 'var(--accent-teal)' }}>Copied!</span>
            )}
          </div>
          <div className="mt-0.5 text-[12.5px] font-mono font-semibold" style={{ color: 'var(--accent-teal)' }}>
            localhost:{instance.port}
          </div>
        </div>

        <div
          onClick={() => handleCopyToken('metric-cpu', instance.cpu_usage, 'CPU Usage')}
          className={`px-3 py-2 rounded border transition-colors ${
            instance.cpu_usage && instance.cpu_usage !== '-'
              ? 'cursor-pointer hover:brightness-110'
              : ''
          }`}
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
          title={instance.cpu_usage && instance.cpu_usage !== '-' ? 'Click to copy CPU usage' : undefined}
        >
          <div className="text-[10.5px] uppercase tracking-wider flex items-center justify-between" style={{ color: 'var(--text-muted)' }}>
            <span>CPU Usage</span>
            {copiedKey === 'metric-cpu' && (
              <span style={{ color: 'var(--accent-teal)' }}>Copied!</span>
            )}
          </div>
          <div className="mt-0.5 text-[12.5px] font-mono font-semibold" style={{ color: 'var(--text-primary)' }}>
            {instance.cpu_usage || '-'}
          </div>
        </div>

        <div
          onClick={() => handleCopyToken('metric-mem', instance.memory_usage, 'Memory Usage')}
          className={`px-3 py-2 rounded border transition-colors ${
            instance.memory_usage && instance.memory_usage !== '-'
              ? 'cursor-pointer hover:brightness-110'
              : ''
          }`}
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
          title={instance.memory_usage && instance.memory_usage !== '-' ? 'Click to copy memory usage' : undefined}
        >
          <div className="text-[10.5px] uppercase tracking-wider flex items-center justify-between" style={{ color: 'var(--text-muted)' }}>
            <span>Memory (Limit: {instance.memory_limit})</span>
            {copiedKey === 'metric-mem' && (
              <span style={{ color: 'var(--accent-teal)' }}>Copied!</span>
            )}
          </div>
          <div className="mt-0.5 text-[12.5px] font-mono font-semibold truncate" style={{ color: 'var(--text-primary)' }}>
            {instance.memory_usage || '-'}
          </div>
        </div>
      </div>

      {/* Details & Config 2-column grid (Click or Double-click value to copy) */}
      <section
        className="rounded border overflow-hidden"
        style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
      >
        <div
          className="px-3 py-1.5 border-b flex items-center justify-between"
          style={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-subtle)' }}
        >
          <span className="text-[11px] font-semibold uppercase tracking-wider" style={{ color: 'var(--accent-secondary)' }}>
            Details & Config
          </span>
          <span className="text-[10.5px]" style={{ color: 'var(--text-muted)' }}>
            Click any value to copy
          </span>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 divide-y md:divide-y-0" style={{ borderColor: 'var(--border-subtle)' }}>
          {metadataFields.map((f) => {
            const isCopied = copiedKey === f.key;
            return (
              <div
                key={f.key}
                className="flex items-center justify-between px-3 py-1.5 border-b text-[12px]"
                style={{ borderColor: 'var(--border-subtle)' }}
              >
                <span className="font-semibold w-24 shrink-0" style={{ color: 'var(--text-muted)' }}>
                  {f.label}
                </span>
                <button
                  type="button"
                  onClick={() => handleCopyToken(f.key, f.value, f.label)}
                  title={`Click to copy ${f.label}`}
                  className="font-mono text-right truncate px-1.5 py-0.5 rounded transition-colors cursor-pointer"
                  style={{
                    backgroundColor: isCopied ? 'var(--copied-bg)' : 'transparent',
                    color: f.highlight ? 'var(--accent-teal)' : 'var(--text-primary)',
                  }}
                >
                  {f.key === 'password' ? '••••••••' : f.value}
                  {isCopied && (
                    <span className="ml-1.5 text-[10px]" style={{ color: 'var(--status-ready)' }}>
                      ✓
                    </span>
                  )}
                </button>
              </div>
            );
          })}
        </div>
      </section>

      {/* Connection URI + CLI Command + Backend .env Block */}
      <section className="space-y-2.5">
        <div
          className="rounded border p-2.5"
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="text-[11px] font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
              Connection URI
            </span>
            <button
              type="button"
              onClick={() => handleCopyToken('uri', instance.connection_uri, 'Connection URI')}
              className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] border hover:brightness-110"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                borderColor: 'var(--border-subtle)',
                color: copiedKey === 'uri' ? 'var(--status-ready)' : 'var(--accent-primary)',
              }}
            >
              {copiedKey === 'uri' ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
              <span>{copiedKey === 'uri' ? 'Copied' : 'Copy'}</span>
              <span className="kbd">c</span>
            </button>
          </div>
          <code
            onClick={() => handleCopyToken('uri', instance.connection_uri, 'Connection URI')}
            className="block font-mono text-[12px] break-all cursor-pointer select-text"
            style={{ color: 'var(--accent-teal)' }}
          >
            {instance.connection_uri}
          </code>
        </div>

        <div
          className="rounded border p-2.5"
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
        >
          <div className="flex items-center justify-between mb-1">
            <span className="text-[11px] font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
              CLI Command
            </span>
            <button
              type="button"
              onClick={() => handleCopyToken('cli', instance.cli_command, 'CLI Command')}
              className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] border hover:brightness-110"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                borderColor: 'var(--border-subtle)',
                color: copiedKey === 'cli' ? 'var(--status-ready)' : 'var(--accent-primary)',
              }}
            >
              {copiedKey === 'cli' ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
              <span>{copiedKey === 'cli' ? 'Copied' : 'Copy'}</span>
            </button>
          </div>
          <code
            onClick={() => handleCopyToken('cli', instance.cli_command, 'CLI Command')}
            className="block font-mono text-[12px] break-all cursor-pointer select-text"
            style={{ color: 'var(--status-starting)' }}
          >
            {instance.cli_command}
          </code>
        </div>

        <div
          className="rounded border p-2.5"
          style={{ backgroundColor: 'var(--bg-surface)', borderColor: 'var(--border-subtle)' }}
        >
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-[11px] font-semibold uppercase tracking-wider" style={{ color: 'var(--text-muted)' }}>
              Backend .env Block
            </span>
            <button
              type="button"
              onClick={() => handleCopyToken('envblock', instance.backend_env, 'Backend .env block')}
              className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] border hover:brightness-110"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                borderColor: 'var(--border-subtle)',
                color: copiedKey === 'envblock' ? 'var(--status-ready)' : 'var(--accent-primary)',
              }}
            >
              {copiedKey === 'envblock' ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
              <span>{copiedKey === 'envblock' ? 'Copied' : 'Export .env'}</span>
              <span className="kbd">E</span>
            </button>
          </div>
          <pre
            className="font-mono text-[11.5px] leading-relaxed p-2 rounded border overflow-x-auto select-text"
            style={{
              backgroundColor: 'var(--bg-app)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            {instance.backend_env}
          </pre>
        </div>
      </section>
    </div>
  );
};
