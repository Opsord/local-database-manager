import type {
  AppMetadata,
  EnginesHealthDTO,
  InstanceDTO,
  LogClosedEvent,
  LogLineEvent,
  SaveInstanceRequest,
  SaveInstanceResponse,
} from '../types';

interface WailsServiceBinding {
  GetAppMetadata(): Promise<AppMetadata>;
  ListInstances(): Promise<InstanceDTO[]>;
  GetEnginesHealth(): Promise<EnginesHealthDTO>;
  StartEngine(runtimeName: string): Promise<void>;
  StopEngine(runtimeName: string): Promise<void>;
  StartInstance(name: string): Promise<InstanceDTO>;
  StopInstance(name: string): Promise<InstanceDTO>;
  PurgeInstance(name: string): Promise<InstanceDTO>;
  DeleteInstance(name: string): Promise<void>;
  SuggestFreePort(startPort: number): Promise<number>;
  PreviewDerivedVolume(engine: string, name: string, version: string): Promise<string>;
  GetEngineDefaults(engine: string, name: string, version: string): Promise<Record<string, string>>;
  SaveInstance(req: SaveInstanceRequest): Promise<SaveInstanceResponse>;
  RestartAfterEdit(oldName: string, newName: string, oldEnvPath: string): Promise<void>;
  DiscardRenameCleanup(oldEnvPath: string): Promise<void>;
  StartLogStream(name: string): Promise<void>;
  StopLogStream(): Promise<void>;
  CopyText(text: string): Promise<void>;
  OpenEnvInEditor(name: string): Promise<void>;
}

interface WailsRuntime {
  EventsOn?(eventName: string, callback: (...data: unknown[]) => void): () => void;
  EventsOff?(eventName: string, ...additionalEventNames: string[]): void;
  Quit?(): void;
}

declare global {
  interface Window {
    go?: {
      desktop?: {
        Service?: WailsServiceBinding;
      };
    };
    runtime?: WailsRuntime;
  }
}

function getService(): WailsServiceBinding {
  const svc = window.go?.desktop?.Service;
  if (!svc) {
    throw new Error('Wails desktop backend bridge is unavailable.');
  }
  return svc;
}

export function hasDesktopBridge(): boolean {
  return Boolean(window.go?.desktop?.Service);
}

export const backend = {
  getAppMetadata: (): Promise<AppMetadata> => getService().GetAppMetadata(),
  listInstances: (): Promise<InstanceDTO[]> => getService().ListInstances(),
  getEnginesHealth: (): Promise<EnginesHealthDTO> => getService().GetEnginesHealth(),
  startEngine: (runtimeName: string): Promise<void> => getService().StartEngine(runtimeName),
  stopEngine: (runtimeName: string): Promise<void> => getService().StopEngine(runtimeName),
  startInstance: (name: string): Promise<InstanceDTO> => getService().StartInstance(name),
  stopInstance: (name: string): Promise<InstanceDTO> => getService().StopInstance(name),
  purgeInstance: (name: string): Promise<InstanceDTO> => getService().PurgeInstance(name),
  deleteInstance: (name: string): Promise<void> => getService().DeleteInstance(name),
  suggestFreePort: (startPort: number): Promise<number> => getService().SuggestFreePort(startPort),
  previewDerivedVolume: (engine: string, name: string, version: string): Promise<string> =>
    getService().PreviewDerivedVolume(engine, name, version),
  getEngineDefaults: (
    engine: string,
    name: string,
    version: string
  ): Promise<Record<string, string>> => getService().GetEngineDefaults(engine, name, version),
  saveInstance: (req: SaveInstanceRequest): Promise<SaveInstanceResponse> =>
    getService().SaveInstance(req),
  restartAfterEdit: (oldName: string, newName: string, oldEnvPath: string): Promise<void> =>
    getService().RestartAfterEdit(oldName, newName, oldEnvPath),
  discardRenameCleanup: (oldEnvPath: string): Promise<void> =>
    getService().DiscardRenameCleanup(oldEnvPath),
  startLogStream: (name: string): Promise<void> => getService().StartLogStream(name),
  stopLogStream: (): Promise<void> => getService().StopLogStream(),
  copyText: async (text: string): Promise<void> => {
    if (hasDesktopBridge()) {
      return getService().CopyText(text);
    }
    await navigator.clipboard.writeText(text);
  },
  openEnvInEditor: (name: string): Promise<void> => getService().OpenEnvInEditor(name),
  quitApp: (): void => {
    window.runtime?.Quit?.();
  },
};

export function subscribeLogEvents(
  onLine: (event: LogLineEvent) => void,
  onClosed: (event: LogClosedEvent) => void
): () => void {
  const rt = window.runtime;
  if (!rt?.EventsOn) {
    return () => {};
  }
  const offLine = rt.EventsOn('logs:line', (payload: unknown) => {
    if (payload && typeof payload === 'object') {
      onLine(payload as LogLineEvent);
    }
  });
  const offClosed = rt.EventsOn('logs:closed', (payload: unknown) => {
    if (payload && typeof payload === 'object') {
      onClosed(payload as LogClosedEvent);
    }
  });
  return () => {
    if (typeof offLine === 'function') offLine();
    if (typeof offClosed === 'function') offClosed();
    rt.EventsOff?.('logs:line', 'logs:closed');
  };
}

export function extractErrorMessage(err: unknown): string {
  if (typeof err === 'string') return err;
  if (err && typeof err === 'object' && 'message' in err) {
    return String((err as { message: unknown }).message);
  }
  return String(err);
}
