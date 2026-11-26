// Typed bridge to the Go backend (desktop/app.go). Wails exposes bound methods
// on window.go.main.App and events on window.runtime.

export type Network = {
  id: string;
  name: string;
  chain_id: string;
  evm_chain_id: number;
  launched: boolean;
  note: string;
};

export type Settings = {
  network: string;
  moniker: string;
  operator: string;
  max_cpus: number;
  max_memory_mb: number;
  start_at_login: boolean;
};

export type Status = {
  updated_at: string;
  network: string;
  chain_id: string;
  initialized: boolean;
  has_key: boolean;
  locked: boolean;
  nirad_found: boolean;
  node_running: boolean;
  worker_running: boolean;
  reachable: boolean;
  height: number;
  block_time: string;
  catching_up: boolean;
  peers: number;
  operator: string;
  balance: string;
  node_status: "" | "ACTIVE" | "UNBONDING";
  bond: string;
  units_total: string;
  rewards_total: string;
  slashed_total: string;
  epoch: string;
  next_epoch_height: string;
  min_bond: string;
  unbond_complete_height: string;
  disk_bytes: number;
  error?: string;
};

export type LogLine = { time: string; source: "node" | "work"; text: string };
export type SystemInfo = { cpus: number; os: string; data_dir: string };

type Bound = {
  Networks(): Promise<Network[]>;
  State(): Promise<Status>;
  Settings(): Promise<Settings>;
  SaveLimits(cpus: number, memMB: number): Promise<void>;
  System(): Promise<SystemInfo>;
  Setup(network: string, moniker: string): Promise<void>;
  CreateKey(pass: string): Promise<string>;
  ImportKey(mnemonic: string, pass: string): Promise<string>;
  Unlock(pass: string): Promise<void>;
  Lock(): Promise<void>;
  StartNode(): Promise<void>;
  StopNode(): Promise<void>;
  StartWork(): Promise<void>;
  StopWork(): Promise<void>;
  Register(bondNira: number): Promise<string>;
  Deregister(): Promise<string>;
  Logs(n: number): Promise<LogLine[]>;
};

declare global {
  interface Window {
    go?: { main?: { App?: Bound } };
    runtime?: { EventsOn(name: string, cb: (...data: unknown[]) => void): () => void };
  }
}

export const inApp = () => !!window.go?.main?.App;

export const api: Bound = new Proxy({} as Bound, {
  get(_t, name: string) {
    return (...args: unknown[]) => {
      const fn = (window.go?.main?.App as Record<string, (...a: unknown[]) => Promise<unknown>> | undefined)?.[name];
      if (!fn) return Promise.reject(new Error("Open this page inside the Nira Node app"));
      return fn(...args);
    };
  },
});

export function onEvent(name: string, cb: (data: string) => void): () => void {
  return window.runtime?.EventsOn(name, (d) => cb(String(d))) ?? (() => {});
}

/** Error text from a rejected bound call (Wails rejects with a string). */
export const errText = (e: unknown) => (typeof e === "string" ? e : e instanceof Error ? e.message : String(e));
