import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Check, Copy, X } from 'lucide-react';
import { TitleBar } from './components/TitleBar';
import { Sidebar } from './components/Sidebar';
import { InspectorPane } from './components/InspectorPane';
import { LogsDock } from './components/LogsDock';
import { WizardDock } from './components/WizardDock';
import { EngineDock } from './components/EngineDock';
import { ConfirmBanner } from './components/ConfirmBanner';
import { CommandPalette, type PaletteCommand } from './components/CommandPalette';
import { HelpModal } from './components/HelpModal';
import {
  backend,
  extractErrorMessage,
  hasDesktopBridge,
  subscribeLogEvents,
  subscribeUpdateEvents,
} from './lib/backend';
import type {
  AppMetadata,
  ConfirmState,
  EnginesHealthDTO,
  InstanceDTO,
  SaveInstanceRequest,
  ThemeId,
  UpdateInfoDTO,
} from './types';

type DockMode = 'none' | 'logs' | 'wizard-create' | 'wizard-edit' | 'engines';

const DEFAULT_METADATA: AppMetadata = {
  app_version: 'dev',
  check_updates: true,
  project_root: '',
  instances_dir: '',
  postgres_versions: ['14', '15', '16', '17', '18'],
  sqlserver_versions: ['2022', 'azure-edge'],
  default_postgres_version: '18',
  default_sqlserver_version: '2022',
  engine_health_interval_ms: 5000,
};

const THEME_STORAGE_KEY = 'dbm-desktop-theme';

function loadSavedTheme(): ThemeId {
  try {
    const saved = localStorage.getItem(THEME_STORAGE_KEY);
    if (saved === 'night-owl' || saved === 'zed-dark') {
      return saved;
    }
  } catch {
    // ignore storage errors
  }
  return 'zed-dark';
}

export default function App() {
  const [theme, setTheme] = useState<ThemeId>(loadSavedTheme);
  const [metadata, setMetadata] = useState<AppMetadata>(DEFAULT_METADATA);
  const [instances, setInstances] = useState<InstanceDTO[]>([]);
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const [engines, setEngines] = useState<EnginesHealthDTO>({
    docker: 'OFFLINE',
    podman: 'OFFLINE',
  });

  const [filterText, setFilterText] = useState('');
  const [isFiltering, setIsFiltering] = useState(false);

  const [dockMode, setDockMode] = useState<DockMode>('none');
  const [editTarget, setEditTarget] = useState<InstanceDTO | null>(null);

  const [logInstanceName, setLogInstanceName] = useState<string>('');
  const [logLines, setLogLines] = useState<string[]>([]);
  const [logFollowing, setLogFollowing] = useState<boolean>(true);
  const [logFilterText, setLogFilterText] = useState<string>('');

  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [busyAction, setBusyAction] = useState<string | null>(null);
  const [engineBusy, setEngineBusy] = useState<boolean>(false);
  const [isRefreshing, setIsRefreshing] = useState<boolean>(false);
  const [updateInfo, setUpdateInfo] = useState<UpdateInfoDTO | null>(null);

  const [paletteOpen, setPaletteOpen] = useState<boolean>(false);
  const [helpOpen, setHelpOpen] = useState<boolean>(false);

  const [statusMsg, setStatusMsg] = useState<string>('Ready.');
  const [statusIsErr, setStatusIsErr] = useState<boolean>(false);
  const [errorCopied, setErrorCopied] = useState<boolean>(false);
  const statusTimerRef = useRef<number | null>(null);
  const errorCopyTimerRef = useRef<number | null>(null);

  const [sidebarWidth, setSidebarWidth] = useState<number>(280);
  const resizingRef = useRef<boolean>(false);

  const sidebarFilterRef = useRef<HTMLInputElement>(null);
  const logFilterRef = useRef<HTMLInputElement>(null);
  const refreshInFlightRef = useRef<boolean>(false);
  const activeLogTargetRef = useRef<{ name: string; streamId: number | null }>({
    name: '',
    streamId: null,
  });

  // Sync theme attribute to <html>
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    try {
      localStorage.setItem(THEME_STORAGE_KEY, theme);
    } catch {
      // ignore
    }
  }, [theme]);

  const showStatus = useCallback((msg: string, isErr = false, clearAfterMs = 4000) => {
    if (statusTimerRef.current !== null) {
      window.clearTimeout(statusTimerRef.current);
      statusTimerRef.current = null;
    }
    setErrorCopied(false);
    setStatusMsg(msg);
    setStatusIsErr(isErr);
    if (!isErr && clearAfterMs > 0) {
      statusTimerRef.current = window.setTimeout(() => {
        setStatusMsg('Ready.');
        setStatusIsErr(false);
      }, clearAfterMs);
    }
  }, []);

  const dismissStatusError = useCallback(() => {
    if (statusTimerRef.current !== null) {
      window.clearTimeout(statusTimerRef.current);
      statusTimerRef.current = null;
    }
    setErrorCopied(false);
    setStatusMsg('Ready.');
    setStatusIsErr(false);
  }, []);

  const handleCopyStatusError = useCallback(async () => {
    if (!statusMsg) return;
    try {
      await backend.copyText(statusMsg);
      if (errorCopyTimerRef.current !== null) {
        window.clearTimeout(errorCopyTimerRef.current);
      }
      setErrorCopied(true);
      errorCopyTimerRef.current = window.setTimeout(() => {
        setErrorCopied(false);
      }, 1800);
    } catch {
      // ignore clipboard errors while preserving error visibility
    }
  }, [statusMsg]);

  const refreshData = useCallback(async (showSpinner = false) => {
    if (!hasDesktopBridge() || refreshInFlightRef.current) return;
    refreshInFlightRef.current = true;
    if (showSpinner) setIsRefreshing(true);
    try {
      const [list, health] = await Promise.all([
        backend.listInstances(),
        backend.getEnginesHealth(),
      ]);
      setInstances(list);
      setEngines(health);
      setSelectedName((curr) => {
        if (curr && list.some((i) => i.name === curr)) return curr;
        return list.length > 0 ? list[0].name : null;
      });
    } catch (err) {
      showStatus(`Error loading instances: ${extractErrorMessage(err)}`, true, 5000);
    } finally {
      refreshInFlightRef.current = false;
      if (showSpinner) setIsRefreshing(false);
    }
  }, [showStatus]);

  // Initial load
  useEffect(() => {
    if (!hasDesktopBridge()) {
      showStatus('Running in browser preview mode (Wails backend not attached).', false, 0);
      return;
    }
    backend
      .getAppMetadata()
      .then((meta) => {
        setMetadata(meta);
        if (meta.check_updates) {
          backend
            .checkForUpdates()
            .then((info) => {
              if (info.update_available) {
                setUpdateInfo(info);
              }
            })
            .catch(() => {});
        }
      })
      .catch(() => {});
    refreshData(true);
  }, [refreshData, showStatus]);

  // Update download/install progress events listener
  useEffect(() => {
    return subscribeUpdateEvents((ev) => {
      if (ev.message) {
        showStatus(ev.message, false, 0);
      }
    });
  }, [showStatus]);

  // Periodic health & metrics polling aligned with config.yml interval
  useEffect(() => {
    if (!hasDesktopBridge()) return;
    const ms = Math.max(2000, metadata.engine_health_interval_ms || 5000);
    const id = window.setInterval(() => {
      refreshData(false);
    }, ms);
    return () => window.clearInterval(id);
  }, [metadata.engine_health_interval_ms, refreshData]);

  // Log events listener
  useEffect(() => {
    const unsub = subscribeLogEvents(
      (ev) => {
        const target = activeLogTargetRef.current;
        if (!target.name || ev.instance_name !== target.name) return;
        if (target.streamId === null) {
          target.streamId = ev.stream_id;
        } else if (ev.stream_id !== target.streamId) {
          return;
        }
        setLogLines((prev) => {
          const next = [...prev, ev.line];
          return next.length > 1000 ? next.slice(next.length - 1000) : next;
        });
      },
      (ev) => {
        const target = activeLogTargetRef.current;
        if (!target.name || ev.instance_name !== target.name) return;
        if (target.streamId !== null && ev.stream_id !== target.streamId) return;
        setLogLines((prev) => [
          ...prev,
          ev.error ? `Log stream error: ${ev.error}` : '--- End of logs stream ---',
        ]);
      }
    );
    return unsub;
  }, []);

  // Resizable sidebar drag handlers
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!resizingRef.current) return;
      const clamped = Math.min(440, Math.max(220, e.clientX));
      setSidebarWidth(clamped);
    };
    const onUp = () => {
      resizingRef.current = false;
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
  }, []);

  const filteredInstances = useMemo(() => {
    const q = filterText.trim().toLowerCase();
    if (!q) return instances;
    return instances.filter(
      (inst) =>
        inst.name.toLowerCase().includes(q) ||
        inst.engine_type.toLowerCase().includes(q) ||
        inst.runtime.toLowerCase().includes(q) ||
        inst.database.toLowerCase().includes(q) ||
        String(inst.port).includes(q)
    );
  }, [instances, filterText]);

  const selectedInstance = useMemo(() => {
    if (filteredInstances.length === 0) return null;
    return (
      filteredInstances.find((i) => i.name === selectedName) ?? filteredInstances[0]
    );
  }, [filteredInstances, selectedName]);

  const closeLogsDock = useCallback(async () => {
    activeLogTargetRef.current = { name: '', streamId: null };
    if (hasDesktopBridge()) {
      await backend.stopLogStream().catch(() => {});
    }
    setDockMode((curr) => (curr === 'logs' ? 'none' : curr));
  }, []);

  const openLogsDock = useCallback(
    async (inst: InstanceDTO) => {
      setConfirm(null);
      activeLogTargetRef.current = { name: inst.name, streamId: null };
      setLogInstanceName(inst.name);
      setLogLines([]);
      setLogFilterText('');
      setLogFollowing(true);
      setDockMode('logs');
      try {
        await backend.startLogStream(inst.name);
      } catch (err) {
        showStatus(`Logs error: ${extractErrorMessage(err)}`, true, 5000);
      }
    },
    [showStatus]
  );

  const selectInstance = useCallback(
    (inst: InstanceDTO) => {
      setSelectedName(inst.name);
      setConfirm((curr) => (curr ? null : curr));
      if (dockMode === 'logs' && inst.name !== logInstanceName) {
        openLogsDock(inst);
      } else if (dockMode === 'wizard-edit' && editTarget?.name !== inst.name) {
        setEditTarget(inst);
      }
    },
    [dockMode, editTarget?.name, logInstanceName, openLogsDock]
  );

  const handleToggleInstance = useCallback(
    async (inst: InstanceDTO) => {
      const running = inst.status === 'READY' || inst.status === 'STARTING';
      setConfirm(null);
      setBusyAction(running ? 'Stopping' : 'Starting');
      showStatus(`${running ? 'Stopping' : 'Starting'} '${inst.name}'...`, false, 0);
      try {
        const updated = running
          ? await backend.stopInstance(inst.name)
          : await backend.startInstance(inst.name);
        setInstances((prev) =>
          prev.map((item) => (item.name === updated.name ? updated : item))
        );
        showStatus(
          `Instance '${inst.name}' ${running ? 'stopped' : 'started'} successfully!`
        );
      } catch (err) {
        const msg = extractErrorMessage(err);
        if (!running && msg.startsWith('ENGINE_OFFLINE:')) {
          setConfirm({
            kind: 'engine-start-retry',
            runtime: inst.runtime || 'docker',
            inst,
          });
          const engineName = inst.runtime === 'podman' ? 'Podman' : 'Docker';
          showStatus(`${engineName} is offline. Start engine and retry?`, true, 0);
        } else {
          showStatus(`Error: ${msg}`, true, 6000);
        }
      } finally {
        setBusyAction(null);
      }
    },
    [showStatus]
  );

  const handleCopy = useCallback(
    async (text: string, label: string) => {
      try {
        await backend.copyText(text);
        showStatus(`${label} copied to clipboard!`, false, 3000);
      } catch (err) {
        showStatus(`Failed to copy: ${extractErrorMessage(err)}`, true, 4000);
      }
    },
    [showStatus]
  );

  const handleOpenEditor = useCallback(
    async (name: string) => {
      try {
        await backend.openEnvInEditor(name);
        showStatus(`Opening '${name}.env' in external editor...`);
      } catch (err) {
        showStatus(`Failed to open editor: ${extractErrorMessage(err)}`, true, 5000);
      }
    },
    [showStatus]
  );

  const handleStartEngine = useCallback(
    async (runtime: string, retryInst?: InstanceDTO) => {
      const displayName = runtime === 'podman' ? 'Podman' : 'Docker';
      setEngineBusy(true);
      showStatus(
        runtime === 'podman' ? 'Starting Podman machine...' : 'Starting Docker...',
        false,
        0
      );
      try {
        await backend.startEngine(runtime);
        const health = await backend.getEnginesHealth();
        setEngines(health);
        showStatus(`${displayName} is online`);
        if (retryInst) {
          await handleToggleInstance(retryInst);
        }
      } catch (err) {
        showStatus(`Failed to start ${displayName}: ${extractErrorMessage(err)}`, true, 6000);
      } finally {
        setEngineBusy(false);
      }
    },
    [handleToggleInstance, showStatus]
  );

  const handleStopEngine = useCallback(
    async (runtime: string) => {
      const displayName = runtime === 'podman' ? 'Podman' : 'Docker';
      setEngineBusy(true);
      showStatus(`Stopping ${displayName}...`, false, 0);
      try {
        await backend.stopEngine(runtime);
        await refreshData(false);
        showStatus(`${displayName} is offline`);
      } catch (err) {
        showStatus(`Failed to stop ${displayName}: ${extractErrorMessage(err)}`, true, 6000);
      } finally {
        setEngineBusy(false);
      }
    },
    [refreshData, showStatus]
  );

  const handleWizardSave = useCallback(
    async (req: SaveInstanceRequest) => {
      const res = await backend.saveInstance(req);
      setDockMode('none');
      setEditTarget(null);
      setSelectedName(res.name);
      await refreshData(false);

      if (res.needs_restart_prompt) {
        setConfirm({
          kind: 'restart-after-edit',
          oldName: req.original_name,
          newName: res.name,
          oldEnvPath: res.cleanup_old_env_path,
          volumeChanged: res.volume_changed,
          derivedVolume: res.derived_volume,
        });
        showStatus(
          res.volume_changed
            ? `Saved (volume → ${res.derived_volume}; old volume kept). Restart container with new config?`
            : 'Saved. Restart container with new config?',
          true,
          0
        );
        return;
      }

      if (req.is_edit) {
        if (res.volume_changed) {
          showStatus(
            `Volume will change to ${res.derived_volume}. Previous volume ${res.source_volume} is kept until purged.`,
            false,
            6000
          );
        } else {
          showStatus(`Instance '${res.name}' saved`);
        }
      } else {
        showStatus(`Instance '${res.name}' created successfully!`);
      }
    },
    [refreshData, showStatus]
  );

  const handleCheckForUpdates = useCallback(
    async (manual: boolean) => {
      if (!hasDesktopBridge()) {
        if (manual) {
          showStatus('Update check is unavailable in browser preview mode.', true, 4000);
        }
        return;
      }
      if (manual && updateInfo?.update_available) {
        setConfirm({ kind: 'update-available', info: updateInfo });
        return;
      }
      if (manual) {
        setBusyAction('Checking updates');
        showStatus('Checking for updates...', false, 0);
      }
      try {
        const info = await backend.checkForUpdates();
        setUpdateInfo(info);
        if (info.update_available) {
          if (manual) {
            setConfirm({ kind: 'update-available', info });
            showStatus(`Update ${info.latest_version} available!`, false, 0);
          }
        } else if (manual) {
          showStatus(`Up to date (${info.current_version}).`, false, 3500);
        }
      } catch (err) {
        if (manual) {
          showStatus(`Update check failed: ${extractErrorMessage(err)}`, true, 5000);
        }
      } finally {
        if (manual) {
          setBusyAction(null);
        }
      }
    },
    [showStatus, updateInfo]
  );

  const executeConfirm = useCallback(async () => {
    if (!confirm) return;
    const current = confirm;
    setConfirm(null);

    switch (current.kind) {
      case 'purge': {
        setBusyAction('Purging');
        showStatus(`Purging '${current.inst.name}' and removing volume...`, false, 0);
        try {
          const updated = await backend.purgeInstance(current.inst.name);
          setInstances((prev) =>
            prev.map((item) => (item.name === updated.name ? updated : item))
          );
          showStatus(`Container and volume for '${current.inst.name}' purged successfully!`);
        } catch (err) {
          showStatus(`Error: ${extractErrorMessage(err)}`, true, 5000);
        } finally {
          setBusyAction(null);
        }
        break;
      }
      case 'delete': {
        setBusyAction('Deleting');
        showStatus(`Deleting '${current.inst.name}'...`, false, 0);
        try {
          await backend.deleteInstance(current.inst.name);
          const rt = (current.inst.runtime || 'docker').toLowerCase();
          const notInstalled =
            (rt === 'docker' && engines.docker === 'NOT_INSTALLED') ||
            (rt === 'podman' && engines.podman === 'NOT_INSTALLED') ||
            (rt !== 'docker' && rt !== 'podman');
          await refreshData(false);
          showStatus(
            notInstalled
              ? `Instance '${current.inst.name}' deleted (.env removed; ${rt} not installed, container purge skipped)`
              : `Instance '${current.inst.name}' deleted (container, volume, and .env removed)`
          );
        } catch (err) {
          showStatus(`Error: ${extractErrorMessage(err)}`, true, 6000);
        } finally {
          setBusyAction(null);
        }
        break;
      }
      case 'engine-start-retry':
        await handleStartEngine(current.runtime, current.inst);
        break;
      case 'engine-stop':
        await handleStopEngine(current.runtime);
        break;
      case 'quit-stop-engine': {
        const displayName = current.runtime === 'podman' ? 'Podman' : 'Docker';
        setEngineBusy(true);
        showStatus(`Stopping ${displayName} and quitting...`, false, 0);
        try {
          await backend.stopEngine(current.runtime);
        } catch {
          // proceed with quit even if stop times out
        } finally {
          setEngineBusy(false);
          backend.quitApp();
        }
        break;
      }
      case 'restart-after-edit': {
        setBusyAction('Restarting');
        showStatus(`Restarting '${current.newName}'...`, false, 0);
        try {
          await backend.restartAfterEdit(
            current.oldName,
            current.newName,
            current.oldEnvPath
          );
          await refreshData(false);
          showStatus(`Instance '${current.newName}' restarted successfully!`);
        } catch (err) {
          await refreshData(false);
          showStatus(
            `Error restarting '${current.newName}': ${extractErrorMessage(err)}`,
            true,
            6000
          );
        } finally {
          setBusyAction(null);
        }
        break;
      }
      case 'update-available': {
        setBusyAction('Updating');
        showStatus(`Downloading and installing ${current.info.latest_version}...`, false, 0);
        try {
          await backend.applyUpdate();
          setUpdateInfo((prev) =>
            prev ? { ...prev, update_available: false } : prev
          );
          setConfirm({
            kind: 'update-restart',
            version: current.info.latest_version,
          });
          showStatus(`Updated to ${current.info.latest_version}! Restart to apply.`, false, 0);
        } catch (err) {
          showStatus(`Update failed: ${extractErrorMessage(err)}`, true, 6000);
        } finally {
          setBusyAction(null);
        }
        break;
      }
      case 'update-restart': {
        setBusyAction('Restarting');
        showStatus(`Restarting application (${current.version})...`, false, 0);
        try {
          await backend.restartApp();
        } catch (err) {
          setBusyAction(null);
          showStatus(`Restart failed: ${extractErrorMessage(err)}`, true, 6000);
        }
        break;
      }
    }
  }, [confirm, engines.docker, engines.podman, handleStartEngine, handleStopEngine, refreshData, showStatus]);

  const cancelConfirm = useCallback(async () => {
    if (!confirm) return;
    const current = confirm;
    setConfirm(null);
    if (current.kind === 'quit-stop-engine') {
      backend.quitApp();
      return;
    }
    if (current.kind === 'restart-after-edit' && current.oldEnvPath) {
      await backend.discardRenameCleanup(current.oldEnvPath).catch(() => {});
      await refreshData(false);
      showStatus('Restart cancelled', false, 2500);
    } else {
      showStatus('Action cancelled', false, 2500);
    }
  }, [confirm, refreshData, showStatus]);

  const dismissConfirm = useCallback(() => {
    setConfirm(null);
    showStatus('Action cancelled', false, 2500);
  }, [showStatus]);

  const paletteCommands: PaletteCommand[] = useMemo(() => {
    const cmds: PaletteCommand[] = [
      {
        id: 'new-instance',
        label: 'New Database Instance',
        category: 'Instance',
        shortcut: 'n',
        run: () => {
          setConfirm(null);
          setEditTarget(null);
          setDockMode('wizard-create');
        },
      },
      {
        id: 'engines',
        label: 'Manage Container Engines (Docker / Podman)',
        category: 'Engine',
        shortcut: 'e',
        run: () => {
          setConfirm(null);
          setDockMode((m) => (m === 'engines' ? 'none' : 'engines'));
        },
      },
      {
        id: 'reload',
        label: 'Reload Instances & Recheck Engine Health',
        category: 'System',
        shortcut: 'r',
        run: () => {
          showStatus('Reloading instances and inspecting runtimes...', false, 2000);
          refreshData(true);
        },
      },
      {
        id: 'check-updates',
        label: updateInfo?.update_available
          ? `Install Update (${updateInfo.latest_version})`
          : 'Check for Updates',
        category: 'System',
        shortcut: 'u',
        run: () => {
          handleCheckForUpdates(true);
        },
      },
      {
        id: 'theme-zed',
        label: 'Theme: Switch to Zed Dark',
        category: 'Theme',
        run: () => setTheme('zed-dark'),
      },
      {
        id: 'theme-night-owl',
        label: 'Theme: Switch to Night Owl (TUI Classic)',
        category: 'Theme',
        run: () => setTheme('night-owl'),
      },
      {
        id: 'help',
        label: 'Show Keyboard Shortcuts & Driver Guide',
        category: 'Help',
        shortcut: '?',
        run: () => setHelpOpen(true),
      },
    ];

    if (selectedInstance) {
      const running =
        selectedInstance.status === 'READY' || selectedInstance.status === 'STARTING';
      cmds.unshift(
        {
          id: 'toggle-selected',
          label: `${running ? 'Stop' : 'Start'} '${selectedInstance.name}'`,
          category: 'Action',
          shortcut: 'Space',
          run: () => handleToggleInstance(selectedInstance),
        },
        {
          id: 'logs-selected',
          label: `View Live Logs for '${selectedInstance.name}'`,
          category: 'Action',
          shortcut: 'l',
          run: () => openLogsDock(selectedInstance),
        },
        {
          id: 'edit-selected',
          label: `Edit Instance '${selectedInstance.name}'`,
          category: 'Action',
          run: () => {
            setConfirm(null);
            setEditTarget(selectedInstance);
            setDockMode('wizard-edit');
          },
        },
        {
          id: 'copy-uri',
          label: `Copy Connection URI for '${selectedInstance.name}'`,
          category: 'Copy',
          shortcut: 'c',
          run: () => handleCopy(selectedInstance.connection_uri, 'Connection URI'),
        },
        {
          id: 'copy-env',
          label: `Export Backend .env Block for '${selectedInstance.name}'`,
          category: 'Copy',
          shortcut: 'E',
          run: () => handleCopy(selectedInstance.backend_env, 'Backend .env block'),
        },
        {
          id: 'open-env',
          label: `Open '${selectedInstance.name}.env' in External Editor`,
          category: 'Action',
          shortcut: 'o',
          run: () => handleOpenEditor(selectedInstance.name),
        },
        {
          id: 'purge-selected',
          label: `Purge Container & Volume for '${selectedInstance.name}'`,
          category: 'Danger',
          shortcut: 'd',
          run: () => setConfirm({ kind: 'purge', inst: selectedInstance }),
        },
        {
          id: 'delete-selected',
          label: `Delete Instance '${selectedInstance.name}' (Purge + Remove .env)`,
          category: 'Danger',
          shortcut: 'D',
          run: () => setConfirm({ kind: 'delete', inst: selectedInstance }),
        }
      );
    }

    for (const inst of instances) {
      cmds.push({
        id: `jump-${inst.name}`,
        label: `Select Instance: ${inst.name} (${inst.engine_type}:${inst.port})`,
        category: 'Navigate',
        run: () => selectInstance(inst),
      });
    }

    return cmds;
  }, [
    handleCheckForUpdates,
    handleCopy,
    handleOpenEditor,
    handleToggleInstance,
    instances,
    openLogsDock,
    refreshData,
    selectInstance,
    selectedInstance,
    showStatus,
    updateInfo,
  ]);

  // Global keyboard shortcuts matching TUI + Zed Command Palette
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && (e.key.toLowerCase() === 'k' || e.key.toLowerCase() === 'p')) {
        e.preventDefault();
        setPaletteOpen((open) => !open);
        return;
      }

      if (paletteOpen) {
        if (e.key === 'Escape') {
          e.preventDefault();
          setPaletteOpen(false);
        }
        return;
      }

      if (helpOpen) {
        if (e.key === 'Escape' || e.key === '?' || e.key === 'Enter') {
          e.preventDefault();
          setHelpOpen(false);
        }
        return;
      }

      const target = e.target as HTMLElement | null;
      const inInput =
        target !== null &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.tagName === 'SELECT' ||
          target.isContentEditable);

      if (inInput) {
        return;
      }

      if (confirm) {
        if (e.key === 'y' || e.key === 'Y') {
          e.preventDefault();
          executeConfirm();
          return;
        }
        if (e.key === 'n' || e.key === 'N') {
          e.preventDefault();
          cancelConfirm();
          return;
        }
        if (e.key === 'Escape') {
          e.preventDefault();
          if (confirm.kind === 'quit-stop-engine') {
            dismissConfirm();
          } else {
            cancelConfirm();
          }
          return;
        }
      }

      if (e.key === 'Escape') {
        e.preventDefault();
        if (dockMode === 'logs') {
          closeLogsDock();
        } else if (dockMode !== 'none') {
          setDockMode('none');
          setEditTarget(null);
        } else if (filterText) {
          setFilterText('');
        } else if (statusIsErr) {
          dismissStatusError();
        }
        return;
      }

      if (dockMode === 'logs') {
        if (e.key === 'f' || e.key === ' ') {
          e.preventDefault();
          setLogFollowing((f) => !f);
          return;
        }
        if (e.key === '/') {
          e.preventDefault();
          logFilterRef.current?.focus();
          return;
        }
        if (e.key === 'c' || e.key === 'y') {
          e.preventDefault();
          const q = logFilterText.trim().toLowerCase();
          const vis = q ? logLines.filter((l) => l.toLowerCase().includes(q)) : logLines;
          if (vis.length === 0) {
            showStatus('No log lines to copy', false, 2500);
          } else {
            handleCopy(vis.join('\n'), `${vis.length} log lines`);
          }
          return;
        }
        if (e.key === 'q') {
          e.preventDefault();
          closeLogsDock();
          return;
        }
      }

      if (dockMode === 'wizard-create' || dockMode === 'wizard-edit') {
        if (e.key === 'o' && dockMode === 'wizard-edit' && editTarget) {
          e.preventDefault();
          handleOpenEditor(editTarget.name);
        }
        return;
      }

      switch (e.key) {
        case '?':
          e.preventDefault();
          setHelpOpen(true);
          break;
        case '/':
          e.preventDefault();
          sidebarFilterRef.current?.focus();
          break;
        case 'Enter':
          if (selectedInstance) {
            e.preventDefault();
            setPaletteOpen(true);
          }
          break;
        case 'q': {
          e.preventDefault();
          if (engines.docker === 'ONLINE' || engines.podman === 'ONLINE') {
            const rt = engines.docker === 'ONLINE' ? 'docker' : 'podman';
            const name = rt === 'podman' ? 'Podman' : 'Docker';
            setConfirm({ kind: 'quit-stop-engine', runtime: rt });
            showStatus(
              `${name} is running. Stop ${name} before quitting? Press 'y' to stop & quit, 'n' to keep running & quit, [Esc] to cancel`,
              true,
              0
            );
          } else {
            backend.quitApp();
          }
          break;
        }
        case 'ArrowDown':
        case 'j': {
          if (filteredInstances.length === 0) break;
          e.preventDefault();
          const idx = filteredInstances.findIndex((i) => i.name === selectedInstance?.name);
          const nextIdx = idx < filteredInstances.length - 1 ? idx + 1 : idx;
          selectInstance(filteredInstances[Math.max(0, nextIdx)]);
          break;
        }
        case 'ArrowUp':
        case 'k': {
          if (filteredInstances.length === 0) break;
          e.preventDefault();
          const idx = filteredInstances.findIndex((i) => i.name === selectedInstance?.name);
          const prevIdx = idx > 0 ? idx - 1 : 0;
          selectInstance(filteredInstances[prevIdx]);
          break;
        }
        case ' ':
          if (selectedInstance && !busyAction) {
            e.preventDefault();
            handleToggleInstance(selectedInstance);
          }
          break;
        case 'c':
          if (selectedInstance) {
            e.preventDefault();
            handleCopy(selectedInstance.connection_uri, `URI for '${selectedInstance.name}'`);
          }
          break;
        case 'E':
        case 'x':
          if (selectedInstance) {
            e.preventDefault();
            handleCopy(
              selectedInstance.backend_env,
              `Backend .env block for '${selectedInstance.name}'`
            );
          }
          break;
        case 'l':
          if (selectedInstance) {
            e.preventDefault();
            openLogsDock(selectedInstance);
          }
          break;
        case 'n':
          e.preventDefault();
          setConfirm(null);
          setEditTarget(null);
          setDockMode('wizard-create');
          break;
        case 'e':
          e.preventDefault();
          setConfirm(null);
          setDockMode((curr) => (curr === 'engines' ? 'none' : 'engines'));
          break;
        case 'o':
          if (selectedInstance) {
            e.preventDefault();
            handleOpenEditor(selectedInstance.name);
          }
          break;
        case 'd':
          if (selectedInstance && !busyAction) {
            e.preventDefault();
            setConfirm({ kind: 'purge', inst: selectedInstance });
          }
          break;
        case 'D':
          if (selectedInstance && !busyAction) {
            e.preventDefault();
            setConfirm({ kind: 'delete', inst: selectedInstance });
          }
          break;
        case 'r':
          e.preventDefault();
          showStatus('Reloading instances and inspecting runtimes...', false, 2000);
          refreshData(true);
          break;
        case 'u':
          if (!busyAction) {
            e.preventDefault();
            handleCheckForUpdates(true);
          }
          break;
      }
    };

    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [
    busyAction,
    cancelConfirm,
    closeLogsDock,
    confirm,
    dismissConfirm,
    dismissStatusError,
    dockMode,
    editTarget,
    engines.docker,
    engines.podman,
    executeConfirm,
    filterText,
    filteredInstances,
    handleCheckForUpdates,
    handleCopy,
    handleOpenEditor,
    handleToggleInstance,
    helpOpen,
    logFilterText,
    logLines,
    openLogsDock,
    paletteOpen,
    refreshData,
    selectInstance,
    selectedInstance,
    showStatus,
    statusIsErr,
  ]);

  return (
    <div
      className="h-screen w-screen flex flex-col overflow-hidden"
      style={{ backgroundColor: 'var(--bg-app)', color: 'var(--text-primary)' }}
    >
      <TitleBar
        selectedInstance={selectedInstance}
        engines={engines}
        engineBusy={engineBusy}
        isRefreshing={isRefreshing}
        theme={theme}
        appVersion={metadata.app_version}
        updateInfo={updateInfo}
        onUpdateClick={() => {
          if (updateInfo?.update_available) {
            setConfirm({ kind: 'update-available', info: updateInfo });
          }
        }}
        onToggleTheme={() =>
          setTheme((prev) => (prev === 'zed-dark' ? 'night-owl' : 'zed-dark'))
        }
        onOpenEngineDock={() =>
          setDockMode((curr) => (curr === 'engines' ? 'none' : 'engines'))
        }
        onRefresh={() => {
          showStatus('Reloading instances and inspecting runtimes...', false, 2000);
          refreshData(true);
        }}
        onOpenPalette={() => setPaletteOpen(true)}
        onOpenHelp={() => setHelpOpen(true)}
      />

      {/* Main Split Workspace */}
      <div className="flex-1 flex min-h-0 overflow-hidden">
        <Sidebar
          instances={filteredInstances}
          totalCount={instances.length}
          selectedName={selectedInstance?.name ?? null}
          filterText={filterText}
          isFiltering={isFiltering}
          filterInputRef={sidebarFilterRef}
          width={sidebarWidth}
          onSelect={selectInstance}
          onFilterChange={(val) => setFilterText(val)}
          onFocusFilter={() => setIsFiltering(true)}
          onClearFilter={() => {
            setFilterText('');
            setIsFiltering(false);
          }}
          onNewInstance={() => {
            setConfirm(null);
            setEditTarget(null);
            setDockMode('wizard-create');
          }}
          onToggleInstance={handleToggleInstance}
        />

        {/* Resizable Zed splitter */}
        <div
          onMouseDown={() => {
            resizingRef.current = true;
          }}
          title="Drag to resize sidebar"
          className="w-1 shrink-0 cursor-col-resize hover:bg-[var(--border-active)] transition-colors"
          style={{ backgroundColor: 'transparent' }}
        />

        {/* Right Workspace Column: Inspector + Optional Bottom Dock */}
        <main className="flex-1 flex flex-col min-w-0 min-h-0 overflow-hidden">
          <InspectorPane
            instance={selectedInstance}
            busyAction={busyAction}
            onToggleRun={handleToggleInstance}
            onOpenLogs={openLogsDock}
            onEdit={(inst) => {
              setConfirm(null);
              setEditTarget(inst);
              setDockMode('wizard-edit');
            }}
            onOpenEditor={(inst) => handleOpenEditor(inst.name)}
            onPurge={(inst) => setConfirm({ kind: 'purge', inst })}
            onDelete={(inst) => setConfirm({ kind: 'delete', inst })}
            onCopy={handleCopy}
            onNewInstance={() => {
              setConfirm(null);
              setEditTarget(null);
              setDockMode('wizard-create');
            }}
          />

          {confirm && (
            <ConfirmBanner
              confirm={confirm}
              busy={busyAction !== null || engineBusy}
              onConfirm={executeConfirm}
              onCancel={cancelConfirm}
              onDismiss={dismissConfirm}
            />
          )}

          {dockMode === 'logs' && (
            <LogsDock
              instanceName={logInstanceName}
              lines={logLines}
              following={logFollowing}
              filterText={logFilterText}
              filterInputRef={logFilterRef}
              onToggleFollow={() => setLogFollowing((f) => !f)}
              onFilterChange={setLogFilterText}
              onCopyLines={(vis) => {
                if (vis.length === 0) {
                  showStatus('No log lines to copy', false, 2500);
                } else {
                  handleCopy(vis.join('\n'), `${vis.length} log lines`);
                }
              }}
              onClearLines={() => setLogLines([])}
              onClose={closeLogsDock}
            />
          )}

          {(dockMode === 'wizard-create' || dockMode === 'wizard-edit') && (
            <WizardDock
              key={dockMode === 'wizard-edit' ? `edit-${editTarget?.name}` : 'create'}
              mode={dockMode === 'wizard-edit' ? 'edit' : 'create'}
              metadata={metadata}
              initialInstance={dockMode === 'wizard-edit' ? editTarget : null}
              onSave={handleWizardSave}
              onOpenEditor={(name) => handleOpenEditor(name)}
              onCancel={() => {
                setDockMode('none');
                setEditTarget(null);
                showStatus(
                  dockMode === 'wizard-edit'
                    ? 'Instance edit cancelled'
                    : 'Instance creation cancelled',
                  false,
                  2500
                );
              }}
            />
          )}

          {dockMode === 'engines' && (
            <EngineDock
              engines={engines}
              busy={engineBusy}
              onStartEngine={(rt) => handleStartEngine(rt)}
              onRequestStopEngine={(rt) =>
                setConfirm({ kind: 'engine-stop', runtime: rt })
              }
              onClose={() => setDockMode('none')}
            />
          )}
        </main>
      </div>

      {/* Zed-style Bottom Status Bar */}
      <footer
        className="h-6 shrink-0 px-3 border-t flex items-center justify-between gap-3 text-[11px] select-none"
        style={{
          backgroundColor: 'var(--bg-panel)',
          borderColor: 'var(--border-subtle)',
        }}
      >
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <span className="font-semibold shrink-0" style={{ color: 'var(--text-muted)' }}>
            Status:
          </span>
          <span
            title={statusIsErr ? statusMsg : undefined}
            className={`truncate font-medium ${statusIsErr ? 'select-text cursor-text' : ''}`}
            style={{
              color: statusIsErr
                ? 'var(--status-stopped)'
                : statusMsg === 'Ready.'
                  ? 'var(--status-ready)'
                  : 'var(--accent-primary)',
            }}
          >
            {busyAction ? `${busyAction}… ` : ''}
            {statusMsg}
          </span>
          {statusIsErr && !confirm && (
            <div className="flex items-center gap-1 shrink-0">
              <button
                type="button"
                onClick={handleCopyStatusError}
                title="Copy error to clipboard"
                className="flex items-center gap-1 px-1.5 py-0 rounded text-[10px] leading-4 border hover:brightness-110 transition-colors"
                style={{
                  backgroundColor: 'var(--bg-elevated)',
                  borderColor: errorCopied ? 'var(--status-ready)' : 'var(--status-stopped)',
                  color: errorCopied ? 'var(--status-ready)' : 'var(--status-stopped)',
                }}
              >
                {errorCopied ? <Check className="w-2.5 h-2.5" /> : <Copy className="w-2.5 h-2.5" />}
                <span>{errorCopied ? 'Copied' : 'Copy'}</span>
              </button>
              <button
                type="button"
                onClick={dismissStatusError}
                title="Dismiss error [Esc]"
                className="p-0.5 rounded hover:brightness-110"
                style={{ color: 'var(--text-muted)' }}
              >
                <X className="w-3 h-3" />
              </button>
            </div>
          )}
        </div>

        <div
          className="hidden md:flex items-center gap-3 shrink-0"
          style={{ color: 'var(--text-muted)' }}
        >
          <span>
            <span className="kbd">↑/↓</span> Nav
          </span>
          <span>
            <span className="kbd">Space</span> Toggle
          </span>
          <span>
            <span className="kbd">/</span> Search
          </span>
          <span>
            <span className="kbd">l</span> Logs
          </span>
          <span>
            <span className="kbd">n</span> New
          </span>
          <span>
            <span className="kbd">e</span> Engines
          </span>
          <span>
            <span className="kbd">Ctrl+K</span> Palette
          </span>
          <span>
            <span className="kbd">?</span> Help
          </span>
        </div>
      </footer>

      {paletteOpen && (
        <CommandPalette
          commands={paletteCommands}
          onClose={() => setPaletteOpen(false)}
        />
      )}

      {helpOpen && <HelpModal onClose={() => setHelpOpen(false)} />}
    </div>
  );
}
