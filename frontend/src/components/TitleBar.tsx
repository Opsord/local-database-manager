import React from 'react';
import {
  ArrowUpCircle,
  Command,
  Database,
  HelpCircle,
  Palette,
  RefreshCw,
  Server,
} from 'lucide-react';
import type { EngineHealth, EnginesHealthDTO, InstanceDTO, ThemeId, UpdateInfoDTO } from '../types';

interface TitleBarProps {
  selectedInstance: InstanceDTO | null;
  engines: EnginesHealthDTO;
  engineBusy: boolean;
  isRefreshing: boolean;
  theme: ThemeId;
  appVersion: string;
  updateInfo: UpdateInfoDTO | null;
  onUpdateClick: () => void;
  onToggleTheme: () => void;
  onOpenEngineDock: () => void;
  onRefresh: () => void;
  onOpenPalette: () => void;
  onOpenHelp: () => void;
}

function engineDotColor(health: EngineHealth): string {
  switch (health) {
    case 'ONLINE':
      return 'var(--status-ready)';
    case 'OFFLINE':
      return 'var(--status-stopped)';
    default:
      return 'var(--status-unknown)';
  }
}

function engineShortLabel(health: EngineHealth): string {
  switch (health) {
    case 'ONLINE':
      return 'ONLINE';
    case 'OFFLINE':
      return 'OFFLINE';
    default:
      return 'N/A';
  }
}

export const TitleBar: React.FC<TitleBarProps> = ({
  selectedInstance,
  engines,
  engineBusy,
  isRefreshing,
  theme,
  appVersion,
  updateInfo,
  onUpdateClick,
  onToggleTheme,
  onOpenEngineDock,
  onRefresh,
  onOpenPalette,
  onOpenHelp,
}) => {
  return (
    <header
      className="h-9 shrink-0 flex items-center justify-between px-3 border-b select-none"
      style={{
        backgroundColor: 'var(--bg-panel)',
        borderColor: 'var(--border-subtle)',
      }}
    >
      {/* Left: Zed-style workspace breadcrumb */}
      <div className="flex items-center gap-2 min-w-0">
        <div className="flex items-center gap-1.5 font-medium tracking-tight">
          <Database className="w-3.5 h-3.5" style={{ color: 'var(--accent-primary)' }} />
          <span className="text-[12.5px] font-semibold" style={{ color: 'var(--text-primary)' }}>
            local-database-manager
          </span>
          <span
            className="text-[10.5px] font-mono px-1.5 py-0.2 rounded border"
            style={{
              backgroundColor: 'var(--bg-app)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-muted)',
            }}
          >
            {appVersion}
          </span>
        </div>
        <span style={{ color: 'var(--text-muted)' }}>›</span>
        <span className="text-[12px]" style={{ color: 'var(--text-muted)' }}>
          instances
        </span>
        {selectedInstance && (
          <>
            <span style={{ color: 'var(--text-muted)' }}>›</span>
            <span
              className="text-[12px] font-mono truncate max-w-[180px]"
              style={{ color: 'var(--accent-teal)' }}
              title={selectedInstance.env_file_path}
            >
              {selectedInstance.name}.env
            </span>
          </>
        )}
      </div>

      {/* Right: Engine Badges + Zed utility controls */}
      <div className="flex items-center gap-1.5">
        {updateInfo?.update_available && (
          <button
            type="button"
            onClick={onUpdateClick}
            title={`Update ${updateInfo.latest_version} available [u]`}
            className="flex items-center gap-1 px-2 py-1 rounded text-[11px] font-medium border transition-colors hover:brightness-110"
            style={{
              backgroundColor: 'rgba(229, 192, 123, 0.16)',
              borderColor: 'var(--status-starting)',
              color: 'var(--status-starting)',
            }}
          >
            <ArrowUpCircle className="w-3 h-3" />
            <span>Update {updateInfo.latest_version}</span>
          </button>
        )}

        <button
          type="button"
          onClick={onOpenEngineDock}
          title="Container Engines [e]"
          className="flex items-center gap-2 px-2 py-1 rounded text-[11px] font-mono border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            borderColor: 'var(--border-subtle)',
          }}
        >
          <Server className="w-3 h-3" style={{ color: 'var(--text-muted)' }} />
          <span className="flex items-center gap-1">
            <span
              className="w-1.5 h-1.5 rounded-full"
              style={{ backgroundColor: engineDotColor(engines.docker) }}
            />
            <span style={{ color: 'var(--text-secondary)' }}>Docker</span>
            <span
              className="font-semibold"
              style={{ color: engineDotColor(engines.docker) }}
            >
              {engineShortLabel(engines.docker)}
            </span>
          </span>
          <span style={{ color: 'var(--border-subtle)' }}>|</span>
          <span className="flex items-center gap-1">
            <span
              className="w-1.5 h-1.5 rounded-full"
              style={{ backgroundColor: engineDotColor(engines.podman) }}
            />
            <span style={{ color: 'var(--text-secondary)' }}>Podman</span>
            <span
              className="font-semibold"
              style={{ color: engineDotColor(engines.podman) }}
            >
              {engineShortLabel(engines.podman)}
            </span>
          </span>
          {engineBusy && (
            <span className="ml-1 text-[10px]" style={{ color: 'var(--status-starting)' }}>
              …
            </span>
          )}
        </button>

        <button
          type="button"
          onClick={onOpenPalette}
          title="Command Palette (Ctrl+K)"
          className="flex items-center gap-1.5 px-2 py-1 rounded text-[11px] border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            borderColor: 'var(--border-subtle)',
            color: 'var(--text-secondary)',
          }}
        >
          <Command className="w-3 h-3" style={{ color: 'var(--accent-primary)' }} />
          <span className="hidden sm:inline">Commands</span>
          <span className="kbd">Ctrl+K</span>
        </button>

        <button
          type="button"
          onClick={onRefresh}
          title="Reload instances & engine health [r]"
          className="p-1.5 rounded border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            borderColor: 'var(--border-subtle)',
            color: 'var(--text-secondary)',
          }}
        >
          <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
        </button>

        <button
          type="button"
          onClick={onToggleTheme}
          title={`Switch theme (Current: ${theme === 'zed-dark' ? 'Zed Dark' : 'Night Owl'})`}
          className="flex items-center gap-1 px-2 py-1 rounded text-[11px] border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            borderColor: 'var(--border-subtle)',
            color: 'var(--text-secondary)',
          }}
        >
          <Palette className="w-3 h-3" style={{ color: 'var(--accent-secondary)' }} />
          <span>{theme === 'zed-dark' ? 'Zed Dark' : 'Night Owl'}</span>
        </button>

        <button
          type="button"
          onClick={onOpenHelp}
          title="Keyboard Shortcuts & Help [?]"
          className="p-1.5 rounded border transition-colors hover:brightness-110"
          style={{
            backgroundColor: 'var(--bg-elevated)',
            borderColor: 'var(--border-subtle)',
            color: 'var(--text-secondary)',
          }}
        >
          <HelpCircle className="w-3.5 h-3.5" />
        </button>
      </div>
    </header>
  );
};
