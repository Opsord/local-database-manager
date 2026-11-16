import React from 'react';
import { AlertTriangle } from 'lucide-react';
import type { ConfirmState } from '../types';

interface ConfirmBannerProps {
  confirm: ConfirmState;
  busy: boolean;
  onConfirm: () => void;
  onCancel: () => void;
  onDismiss?: () => void;
}

function describeConfirm(confirm: ConfirmState): {
  title: string;
  detail: string;
  danger: boolean;
  cancelLabel: string;
  confirmLabel: string;
} {
  switch (confirm.kind) {
    case 'purge':
      return {
        title: `Purge container and volume for '${confirm.inst.name}'?`,
        detail: 'Runs down -v and permanently deletes persistent volume data. Keeps the .env file.',
        danger: true,
        cancelLabel: 'Cancel',
        confirmLabel: 'Confirm',
      };
    case 'delete':
      return {
        title: `Delete instance '${confirm.inst.name}'?`,
        detail: 'Purges container + volume AND removes the .env file from instances/.',
        danger: true,
        cancelLabel: 'Cancel',
        confirmLabel: 'Confirm',
      };
    case 'engine-start-retry': {
      const name = confirm.runtime === 'podman' ? 'Podman' : 'Docker';
      return {
        title: `${name} is offline. Start ${name} and retry starting '${confirm.inst.name}'?`,
        detail: `Launches ${name} daemon and waits for readiness before starting the instance.`,
        danger: false,
        cancelLabel: 'Cancel',
        confirmLabel: 'Confirm',
      };
    }
    case 'engine-stop': {
      const name = confirm.runtime === 'podman' ? 'Podman' : 'Docker';
      return {
        title: `Stop ${name} engine?`,
        detail: `All running ${name} containers will stop when the daemon shuts down.`,
        danger: true,
        cancelLabel: 'Cancel',
        confirmLabel: 'Confirm',
      };
    }
    case 'quit-stop-engine': {
      const name = confirm.runtime === 'podman' ? 'Podman' : 'Docker';
      return {
        title: `${name} is running. Stop ${name} before quitting?`,
        detail: `Press 'y' to stop ${name} & quit, 'n' to keep running & quit, or [Esc] to cancel.`,
        danger: false,
        cancelLabel: 'Keep & Quit',
        confirmLabel: 'Stop & Quit',
      };
    }
    case 'restart-after-edit':
      return {
        title: `Restart '${confirm.newName}' with the updated configuration?`,
        detail: confirm.volumeChanged
          ? `Volume will change to ${confirm.derivedVolume}. Previous volume is kept until purged.`
          : 'Stops the running container and restarts it using the updated .env settings.',
        danger: false,
        cancelLabel: 'Cancel',
        confirmLabel: 'Confirm',
      };
  }
}

export const ConfirmBanner: React.FC<ConfirmBannerProps> = ({
  confirm,
  busy,
  onConfirm,
  onCancel,
  onDismiss,
}) => {
  const { title, detail, danger, cancelLabel, confirmLabel } = describeConfirm(confirm);
  const accent = danger ? 'var(--status-stopped)' : 'var(--status-starting)';

  return (
    <div
      className="px-4 py-2.5 border-t flex flex-wrap items-center justify-between gap-3 shrink-0"
      style={{
        backgroundColor: 'var(--bg-elevated)',
        borderColor: accent,
      }}
    >
      <div className="flex items-start gap-2.5 min-w-0">
        <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" style={{ color: accent }} />
        <div className="min-w-0">
          <div className="text-[12.5px] font-semibold" style={{ color: 'var(--text-primary)' }}>
            {title}
          </div>
          <div className="text-[11.5px]" style={{ color: 'var(--text-muted)' }}>
            {detail}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        {confirm.kind === 'quit-stop-engine' && onDismiss && (
          <button
            type="button"
            disabled={busy}
            onClick={onDismiss}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] border hover:brightness-110 disabled:opacity-50"
            style={{
              backgroundColor: 'var(--bg-surface)',
              borderColor: 'var(--border-subtle)',
              color: 'var(--text-muted)',
            }}
          >
            <span>Dismiss</span>
            <span className="kbd">Esc</span>
          </button>
        )}
        <button
          type="button"
          disabled={busy}
          onClick={onCancel}
          className="flex items-center gap-1.5 px-2.5 py-1 rounded text-[11.5px] border hover:brightness-110 disabled:opacity-50"
          style={{
            backgroundColor: 'var(--bg-surface)',
            borderColor: 'var(--border-subtle)',
            color: 'var(--text-secondary)',
          }}
        >
          <span>{cancelLabel}</span>
          <span className="kbd">n</span>
        </button>
        <button
          type="button"
          disabled={busy}
          onClick={onConfirm}
          className="flex items-center gap-1.5 px-3 py-1 rounded text-[11.5px] font-semibold border hover:brightness-110 disabled:opacity-50"
          style={{
            backgroundColor: danger ? 'rgba(224, 108, 117, 0.18)' : 'rgba(229, 192, 123, 0.18)',
            borderColor: accent,
            color: accent,
          }}
        >
          <span>{busy ? 'Working…' : confirmLabel}</span>
          <span className="kbd">y</span>
        </button>
      </div>
    </div>
  );
};
