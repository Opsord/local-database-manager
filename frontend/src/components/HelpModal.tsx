import React from 'react';
import { X } from 'lucide-react';

interface HelpModalProps {
  onClose: () => void;
}

const SHORTCUTS: Array<[string, string]> = [
  ['Ctrl+K / Enter', 'Open Command Palette (actions, instances, themes)'],
  ['↑ / ↓ or k / j', 'Navigate through database instances'],
  ['/', 'Live Search / Filter instances (or filter inside Logs)'],
  ['Space', 'Start or Stop selected container instance'],
  ['c', 'Copy connection URI to clipboard (or visible logs in Logs dock)'],
  ['E / x', 'Copy backend .env configuration block to clipboard'],
  ['l', 'Open live log streamer dock for selected container'],
  ['n', 'Create new database instance with wizard'],
  ['e', 'Toggle Container Engines dock (Start / Stop Docker & Podman)'],
  ['o', 'Open selected instance .env file in external editor'],
  ['d', 'Purge container and persistent volume (down -v); keeps .env'],
  ['D', 'Delete instance: purge container+volume and remove .env'],
  ['r', 'Reload instances & recheck Docker / Podman daemon health'],
  ['u', 'Check for updates or install available update'],
  ['f / Space (Logs)', 'Toggle Following / Paused auto-scroll in Logs dock'],
  ['y / n', 'Confirm or cancel active confirmation prompt'],
  ['?', 'Toggle this keyboard shortcuts reference'],
  ['q', 'Quit application (prompts to stop running engine if online)'],
  ['Esc', 'Close active dock, modal, or clear filter'],
];

export const HelpModal: React.FC<HelpModalProps> = ({ onClose }) => {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/55"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl rounded-md border shadow-2xl overflow-hidden"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderColor: 'var(--border-active)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          className="px-4 py-2.5 border-b flex items-center justify-between"
          style={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-subtle)' }}
        >
          <span className="text-[13px] font-semibold" style={{ color: 'var(--accent-secondary)' }}>
            Keyboard Shortcuts & Guide
          </span>
          <button
            type="button"
            onClick={onClose}
            className="p-1 rounded hover:opacity-80"
            style={{ color: 'var(--text-muted)' }}
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-4 max-h-[75vh] overflow-y-auto space-y-4">
          <div className="space-y-1.5">
            {SHORTCUTS.map(([keys, desc]) => (
              <div
                key={keys}
                className="flex items-center gap-3 py-1 border-b text-[12px]"
                style={{ borderColor: 'var(--border-subtle)' }}
              >
                <span className="w-40 shrink-0 font-mono font-semibold" style={{ color: 'var(--accent-primary)' }}>
                  {keys}
                </span>
                <span style={{ color: 'var(--text-primary)' }}>{desc}</span>
              </div>
            ))}
          </div>

          <div
            className="p-3 rounded border space-y-1 text-[12px]"
            style={{ backgroundColor: 'var(--bg-app)', borderColor: 'var(--border-subtle)' }}
          >
            <div className="font-semibold" style={{ color: 'var(--accent-secondary)' }}>
              Database Drivers & Tools
            </div>
            <div style={{ color: 'var(--text-muted)' }}>
              • Postgres: psql, pgAdmin, DBeaver, Prisma, TypeORM, GORM
            </div>
            <div style={{ color: 'var(--text-muted)' }}>
              • SQL Server: sqlcmd, SSMS, Azure Data Studio, EF Core
            </div>
          </div>

          <div className="flex justify-end">
            <button
              type="button"
              onClick={onClose}
              className="flex items-center gap-1.5 px-3 py-1 rounded text-[12px] border hover:brightness-110"
              style={{
                backgroundColor: 'var(--bg-elevated)',
                borderColor: 'var(--border-subtle)',
                color: 'var(--text-primary)',
              }}
            >
              <span>Close</span>
              <span className="kbd">Esc</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
