import React from 'react';
import { Power, Square, X } from 'lucide-react';
import type { EngineHealth, EnginesHealthDTO } from '../types';

interface EngineDockProps {
  engines: EnginesHealthDTO;
  busy: boolean;
  onStartEngine: (runtime: 'docker' | 'podman') => void;
  onRequestStopEngine: (runtime: 'docker' | 'podman') => void;
  onClose: () => void;
}

function healthStyle(health: EngineHealth): { label: string; color: string } {
  switch (health) {
    case 'ONLINE':
      return { label: 'ONLINE', color: 'var(--status-ready)' };
    case 'OFFLINE':
      return { label: 'OFFLINE', color: 'var(--status-stopped)' };
    default:
      return { label: 'NOT INSTALLED', color: 'var(--status-unknown)' };
  }
}

export const EngineDock: React.FC<EngineDockProps> = ({
  engines,
  busy,
  onStartEngine,
  onRequestStopEngine,
  onClose,
}) => {
  const rows: Array<{ runtime: 'docker' | 'podman'; name: string; health: EngineHealth }> = [
    { runtime: 'docker', name: 'Docker', health: engines.docker },
    { runtime: 'podman', name: 'Podman', health: engines.podman },
  ];

  return (
    <section
      className="shrink-0 border-t"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderColor: 'var(--border-active)',
      }}
    >
      <div
        className="px-4 py-2 border-b flex items-center justify-between"
        style={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-subtle)' }}
      >
        <span className="text-[12px] font-semibold" style={{ color: 'var(--accent-secondary)' }}>
          Container Engines
        </span>
        <button
          type="button"
          onClick={onClose}
          title="Close [Esc]"
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

      <div className="p-3 grid grid-cols-1 md:grid-cols-2 gap-3">
        {rows.map((row) => {
          const st = healthStyle(row.health);
          return (
            <div
              key={row.runtime}
              className="flex items-center justify-between px-3 py-2.5 rounded border"
              style={{
                backgroundColor: 'var(--bg-app)',
                borderColor: 'var(--border-subtle)',
              }}
            >
              <div>
                <div className="text-[12.5px] font-semibold" style={{ color: 'var(--text-primary)' }}>
                  {row.name}
                </div>
                <div className="text-[11px] font-mono" style={{ color: st.color }}>
                  ● {st.label}
                </div>
              </div>

              {row.health === 'OFFLINE' && (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => onStartEngine(row.runtime)}
                  className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] font-medium border hover:brightness-110 disabled:opacity-50"
                  style={{
                    backgroundColor: 'rgba(152, 195, 121, 0.15)',
                    borderColor: 'var(--status-ready)',
                    color: 'var(--status-ready)',
                  }}
                >
                  <Power className="w-3.5 h-3.5" />
                  <span>Start {row.name}</span>
                </button>
              )}

              {row.health === 'ONLINE' && (
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => onRequestStopEngine(row.runtime)}
                  className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] font-medium border hover:brightness-110 disabled:opacity-50"
                  style={{
                    backgroundColor: 'rgba(224, 108, 117, 0.14)',
                    borderColor: 'var(--status-stopped)',
                    color: 'var(--status-stopped)',
                  }}
                >
                  <Square className="w-3 h-3" />
                  <span>Stop {row.name}</span>
                </button>
              )}

              {row.health === 'NOT_INSTALLED' && (
                <span className="text-[11px] font-mono" style={{ color: 'var(--text-muted)' }}>
                  Not found in PATH
                </span>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
};
