import React from 'react';
import { Plus, Search, X } from 'lucide-react';
import type { ContainerStatus, InstanceDTO } from '../types';

interface SidebarProps {
  instances: InstanceDTO[];
  totalCount: number;
  selectedName: string | null;
  filterText: string;
  isFiltering: boolean;
  filterInputRef: React.RefObject<HTMLInputElement>;
  width: number;
  onSelect: (inst: InstanceDTO) => void;
  onFilterChange: (value: string) => void;
  onFocusFilter: () => void;
  onClearFilter: () => void;
  onNewInstance: () => void;
  onToggleInstance: (inst: InstanceDTO) => void;
}

function statusMeta(status: ContainerStatus): { symbol: string; color: string; label: string } {
  switch (status) {
    case 'READY':
      return { symbol: '●', color: 'var(--status-ready)', label: 'RUNNING' };
    case 'STARTING':
      return { symbol: '◐', color: 'var(--status-starting)', label: 'STARTING' };
    case 'UNKNOWN':
      return { symbol: '?', color: 'var(--status-unknown)', label: 'UNKNOWN' };
    default:
      return { symbol: '○', color: 'var(--status-stopped)', label: 'STOPPED' };
  }
}

export const Sidebar: React.FC<SidebarProps> = ({
  instances,
  totalCount,
  selectedName,
  filterText,
  isFiltering,
  filterInputRef,
  width,
  onSelect,
  onFilterChange,
  onFocusFilter,
  onClearFilter,
  onNewInstance,
  onToggleInstance,
}) => {
  return (
    <aside
      className="shrink-0 flex flex-col border-r h-full"
      style={{
        width: `${width}px`,
        backgroundColor: 'var(--bg-surface)',
        borderColor: 'var(--border-subtle)',
      }}
    >
      {/* Sidebar header */}
      <div
        className="px-3 py-2 flex items-center justify-between border-b"
        style={{ borderColor: 'var(--border-subtle)' }}
      >
        <div className="flex items-center gap-1.5">
          <span
            className="text-[11px] font-semibold uppercase tracking-wider"
            style={{ color: 'var(--text-muted)' }}
          >
            {isFiltering || filterText
              ? `Filter (${instances.length}/${totalCount})`
              : `DB Instances (${totalCount})`}
          </span>
        </div>
        <button
          type="button"
          onClick={onNewInstance}
          title="New Database Instance [n]"
          className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] font-medium border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--badge-bg)',
            borderColor: 'var(--border-active)',
            color: 'var(--accent-primary)',
          }}
        >
          <Plus className="w-3 h-3" />
          <span>New</span>
        </button>
      </div>

      {/* Live Search box */}
      <div
        className="px-2.5 py-2 border-b"
        style={{ borderColor: 'var(--border-subtle)' }}
      >
        <div
          className="flex items-center gap-1.5 px-2 py-1 rounded border"
          style={{
            backgroundColor: 'var(--bg-app)',
            borderColor: isFiltering ? 'var(--border-active)' : 'var(--border-subtle)',
          }}
        >
          <Search className="w-3.5 h-3.5 shrink-0" style={{ color: 'var(--text-muted)' }} />
          <input
            ref={filterInputRef}
            type="text"
            value={filterText}
            onFocus={onFocusFilter}
            onChange={(e) => onFilterChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault();
                onClearFilter();
                (e.target as HTMLInputElement).blur();
              } else if (e.key === 'Enter') {
                e.preventDefault();
                (e.target as HTMLInputElement).blur();
              }
            }}
            placeholder="Filter instances… [/]"
            className="w-full bg-transparent outline-none text-[12px] font-mono placeholder:opacity-60"
            style={{ color: 'var(--text-primary)' }}
          />
          {filterText && (
            <button
              type="button"
              onClick={onClearFilter}
              className="p-0.5 rounded hover:opacity-80"
              style={{ color: 'var(--text-muted)' }}
              title="Clear filter [Esc]"
            >
              <X className="w-3 h-3" />
            </button>
          )}
        </div>
      </div>

      {/* Instances list */}
      <div className="flex-1 overflow-y-auto py-1">
        {instances.length === 0 ? (
          <div className="px-4 py-8 text-center space-y-2">
            {filterText ? (
              <>
                <p className="text-[12px]" style={{ color: 'var(--text-muted)' }}>
                  No matches found.
                </p>
                <button
                  type="button"
                  onClick={onClearFilter}
                  className="text-[11px] underline"
                  style={{ color: 'var(--accent-primary)' }}
                >
                  Reset filter [Esc]
                </button>
              </>
            ) : (
              <>
                <p className="text-[12px]" style={{ color: 'var(--text-secondary)' }}>
                  No instances configured.
                </p>
                <button
                  type="button"
                  onClick={onNewInstance}
                  className="inline-flex items-center gap-1 px-2.5 py-1 rounded text-[11px] border"
                  style={{
                    backgroundColor: 'var(--badge-bg)',
                    borderColor: 'var(--border-active)',
                    color: 'var(--accent-primary)',
                  }}
                >
                  <Plus className="w-3 h-3" />
                  <span>Create instance [n]</span>
                </button>
              </>
            )}
          </div>
        ) : (
          instances.map((inst) => {
            const selected = inst.name === selectedName;
            const meta = statusMeta(inst.status);
            const engineBadge = inst.engine_type === 'sqlserver' ? 'MSSQL' : 'PG';
            const runtimeLabel = inst.runtime === 'podman' ? 'Podman' : 'Docker';
            return (
              <div
                key={inst.name}
                role="button"
                tabIndex={0}
                onClick={() => onSelect(inst)}
                onDoubleClick={() => onToggleInstance(inst)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') onSelect(inst);
                }}
                title={`${inst.name} (${meta.label}) — Double-click to ${
                  inst.status === 'READY' || inst.status === 'STARTING' ? 'stop' : 'start'
                }`}
                className="group relative flex items-center justify-between px-3 py-1.5 cursor-pointer border-l-2 transition-colors"
                style={{
                  backgroundColor: selected ? 'var(--bg-selected)' : 'transparent',
                  borderLeftColor: selected ? 'var(--border-active)' : 'transparent',
                }}
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span
                    className="text-[11px] leading-none shrink-0"
                    style={{ color: meta.color }}
                    title={meta.label}
                  >
                    {meta.symbol}
                  </span>
                  <div className="min-w-0">
                    <div className="flex items-center gap-1.5">
                      <span
                        className="text-[12.5px] font-medium truncate"
                        style={{
                          color: selected ? 'var(--text-primary)' : 'var(--text-secondary)',
                        }}
                      >
                        {inst.name}
                      </span>
                    </div>
                    <div
                      className="flex items-center gap-1.5 text-[10.5px] font-mono"
                      style={{ color: 'var(--text-muted)' }}
                    >
                      <span>{runtimeLabel}</span>
                      <span>·</span>
                      <span style={{ color: 'var(--accent-teal)' }}>{engineBadge}</span>
                      <span>·</span>
                      <span>:{inst.port}</span>
                    </div>
                  </div>
                </div>
                <span
                  className="text-[10px] font-mono px-1.5 py-0.5 rounded opacity-80 shrink-0"
                  style={{
                    backgroundColor: 'var(--bg-app)',
                    color: meta.color,
                    border: '1px solid var(--border-subtle)',
                  }}
                >
                  {meta.label === 'RUNNING' ? 'UP' : meta.label === 'STARTING' ? 'BOOT' : 'OFF'}
                </span>
              </div>
            );
          })
        )}
      </div>
    </aside>
  );
};
