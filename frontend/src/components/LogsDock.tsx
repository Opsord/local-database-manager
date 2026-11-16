import React, { useEffect, useMemo, useRef } from 'react';
import { Copy, Pause, Play, Search, Trash2, X } from 'lucide-react';

interface LogsDockProps {
  instanceName: string;
  lines: string[];
  following: boolean;
  filterText: string;
  filterInputRef: React.RefObject<HTMLInputElement>;
  onToggleFollow: () => void;
  onFilterChange: (value: string) => void;
  onCopyLines: (visibleLines: string[]) => void;
  onClearLines: () => void;
  onClose: () => void;
}

export const LogsDock: React.FC<LogsDockProps> = ({
  instanceName,
  lines,
  following,
  filterText,
  filterInputRef,
  onToggleFollow,
  onFilterChange,
  onCopyLines,
  onClearLines,
  onClose,
}) => {
  const scrollRef = useRef<HTMLDivElement>(null);

  const visibleLines = useMemo(() => {
    const q = filterText.trim().toLowerCase();
    if (!q) return lines;
    return lines.filter((line) => line.toLowerCase().includes(q));
  }, [lines, filterText]);

  useEffect(() => {
    if (!following || !scrollRef.current) return;
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [visibleLines, following]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
    if (distanceFromBottom > 24 && following) {
      onToggleFollow();
    }
  };

  return (
    <section
      className="h-[290px] shrink-0 flex flex-col border-t"
      style={{
        backgroundColor: 'var(--bg-surface)',
        borderColor: 'var(--border-active)',
      }}
    >
      {/* Dock Header */}
      <div
        className="px-3 py-1.5 border-b flex flex-wrap items-center justify-between gap-2 shrink-0"
        style={{ backgroundColor: 'var(--bg-panel)', borderColor: 'var(--border-subtle)' }}
      >
        <div className="flex items-center gap-2.5">
          <span className="text-[11.5px] font-semibold" style={{ color: 'var(--accent-secondary)' }}>
            Logs: {instanceName}
          </span>
          <button
            type="button"
            onClick={onToggleFollow}
            title="Toggle follow [f / Space]"
            className="flex items-center gap-1 px-2 py-0.5 rounded text-[10.5px] font-mono font-semibold border"
            style={{
              backgroundColor: 'var(--bg-app)',
              borderColor: following ? 'var(--status-ready)' : 'var(--status-starting)',
              color: following ? 'var(--status-ready)' : 'var(--status-starting)',
            }}
          >
            {following ? <Play className="w-2.5 h-2.5" /> : <Pause className="w-2.5 h-2.5" />}
            <span>{following ? '● FOLLOWING' : '⏸ PAUSED'}</span>
          </button>
          <span className="text-[11px] font-mono" style={{ color: 'var(--text-muted)' }}>
            {filterText.trim() ? `${visibleLines.length}/${lines.length} lines` : `${lines.length} lines`}
          </span>
        </div>

        <div className="flex items-center gap-1.5">
          <div
            className="flex items-center gap-1 px-2 py-0.5 rounded border"
            style={{ backgroundColor: 'var(--bg-app)', borderColor: 'var(--border-subtle)' }}
          >
            <Search className="w-3 h-3" style={{ color: 'var(--text-muted)' }} />
            <input
              ref={filterInputRef}
              type="text"
              value={filterText}
              onChange={(e) => onFilterChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  e.preventDefault();
                  if (filterText) {
                    onFilterChange('');
                  } else {
                    (e.target as HTMLInputElement).blur();
                  }
                } else if (e.key === 'Enter') {
                  e.preventDefault();
                  (e.target as HTMLInputElement).blur();
                }
              }}
              placeholder="Filter logs… [/]"
              className="w-40 bg-transparent outline-none text-[11.5px] font-mono"
              style={{ color: 'var(--text-primary)' }}
            />
            {filterText && (
              <button
                type="button"
                onClick={() => onFilterChange('')}
                className="hover:opacity-80"
                style={{ color: 'var(--text-muted)' }}
              >
                <X className="w-3 h-3" />
              </button>
            )}
          </div>

          <button
            type="button"
            onClick={() => onCopyLines(visibleLines)}
            title="Copy visible log lines [c]"
            className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] border hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            <Copy className="w-3 h-3" />
            <span>Copy</span>
          </button>

          <button
            type="button"
            onClick={onClearLines}
            title="Clear log buffer"
            className="flex items-center gap-1 px-2 py-0.5 rounded text-[11px] border hover:brightness-110"
            style={{
              backgroundColor: 'var(--bg-elevated)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-secondary)',
            }}
          >
            <Trash2 className="w-3 h-3" />
            <span>Clear</span>
          </button>

          <button
            type="button"
            onClick={onClose}
            title="Close logs [Esc]"
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

      {/* Log lines body */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto px-3 py-2 font-mono text-[11.5px] leading-relaxed select-text"
        style={{ backgroundColor: 'var(--bg-app)', color: 'var(--text-secondary)' }}
      >
        {visibleLines.length === 0 ? (
          <div className="italic" style={{ color: 'var(--text-muted)' }}>
            {filterText.trim() ? '(No matching log lines)' : 'Waiting for log output...'}
          </div>
        ) : (
          visibleLines.map((line, idx) => (
            <div key={idx} className="whitespace-pre-wrap break-all hover:bg-white/[0.03] px-1 rounded">
              {line}
            </div>
          ))
        )}
      </div>
    </section>
  );
};
