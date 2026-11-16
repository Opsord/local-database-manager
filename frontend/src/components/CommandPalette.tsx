import React, { useEffect, useMemo, useState } from 'react';
import { Command, X } from 'lucide-react';

export interface PaletteCommand {
  id: string;
  label: string;
  category: string;
  shortcut?: string;
  run: () => void;
}

interface CommandPaletteProps {
  commands: PaletteCommand[];
  onClose: () => void;
}

export const CommandPalette: React.FC<CommandPaletteProps> = ({ commands, onClose }) => {
  const [query, setQuery] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter(
      (cmd) =>
        cmd.label.toLowerCase().includes(q) || cmd.category.toLowerCase().includes(q)
    );
  }, [commands, query]);

  useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  const execute = (cmd: PaletteCommand | undefined) => {
    if (!cmd) return;
    onClose();
    cmd.run();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-16 bg-black/55"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg rounded-md border shadow-2xl overflow-hidden"
        style={{
          backgroundColor: 'var(--bg-surface)',
          borderColor: 'var(--border-active)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          className="px-3 py-2.5 border-b flex items-center gap-2"
          style={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-subtle)' }}
        >
          <Command className="w-4 h-4 shrink-0" style={{ color: 'var(--accent-primary)' }} />
          <input
            type="text"
            autoFocus
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault();
                onClose();
              } else if (e.key === 'ArrowDown') {
                e.preventDefault();
                setActiveIndex((idx) => (filtered.length ? (idx + 1) % filtered.length : 0));
              } else if (e.key === 'ArrowUp') {
                e.preventDefault();
                setActiveIndex((idx) =>
                  filtered.length ? (idx - 1 + filtered.length) % filtered.length : 0
                );
              } else if (e.key === 'Enter') {
                e.preventDefault();
                execute(filtered[activeIndex]);
              }
            }}
            placeholder="Type a command or instance name..."
            className="w-full bg-transparent outline-none text-[13px]"
            style={{ color: 'var(--text-primary)' }}
          />
          <button
            type="button"
            onClick={onClose}
            className="p-0.5 rounded hover:opacity-80"
            style={{ color: 'var(--text-muted)' }}
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="max-h-72 overflow-y-auto py-1">
          {filtered.length === 0 ? (
            <div className="px-4 py-6 text-center text-[12px]" style={{ color: 'var(--text-muted)' }}>
              No matching commands.
            </div>
          ) : (
            filtered.map((cmd, idx) => {
              const active = idx === activeIndex;
              return (
                <button
                  key={cmd.id}
                  type="button"
                  onMouseEnter={() => setActiveIndex(idx)}
                  onClick={() => execute(cmd)}
                  className="w-full px-3 py-2 flex items-center justify-between text-left transition-colors"
                  style={{
                    backgroundColor: active ? 'var(--bg-selected)' : 'transparent',
                  }}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <span
                      className="text-[10.5px] font-mono uppercase px-1.5 py-0.5 rounded border shrink-0"
                      style={{
                        backgroundColor: 'var(--bg-app)',
                        borderColor: 'var(--border-subtle)',
                        color: 'var(--text-muted)',
                      }}
                    >
                      {cmd.category}
                    </span>
                    <span className="text-[12.5px] truncate" style={{ color: 'var(--text-primary)' }}>
                      {cmd.label}
                    </span>
                  </div>
                  {cmd.shortcut && <span className="kbd shrink-0">{cmd.shortcut}</span>}
                </button>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};
