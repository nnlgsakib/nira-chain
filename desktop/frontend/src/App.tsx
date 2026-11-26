import { useCallback, useEffect, useState } from "react";
import { Activity, LayoutDashboard, Settings as Cog, TerminalSquare } from "lucide-react";
import { api, inApp, type Status } from "./lib/api";
import { Callout, Dot } from "./components/ui";
import { Overview, phaseOf } from "./screens/Overview";
import { Logs } from "./screens/Logs";
import { Settings } from "./screens/Settings";
import { Setup } from "./screens/Setup";
import { Unlock } from "./screens/Unlock";
import { Install } from "./screens/Install";
import { ErrorBoundary } from "./components/ErrorBoundary";

type Tab = "overview" | "logs" | "settings";
const TABS: { id: Tab; label: string; icon: typeof Activity }[] = [
  { id: "overview", label: "Overview", icon: LayoutDashboard },
  { id: "logs", label: "Activity", icon: TerminalSquare },
  { id: "settings", label: "Settings", icon: Cog },
];

export default function App() {
  const [status, setStatus] = useState<Status | null>(null);
  const [installed, setInstalled] = useState<boolean | null>(null);
  const [tab, setTab] = useState<Tab>("overview");
  const [theme, setThemeState] = useState<"dark" | "light">(() => (localStorage.getItem("theme") as "light") || "dark");

  const setTheme = (t: "dark" | "light") => {
    setThemeState(t);
    localStorage.setItem("theme", t);
  };
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  const refresh = useCallback(() => {
    api.InstallInfo().then((i) => setInstalled(i.installed)).catch(() => {});
    api.State().then(setStatus).catch(() => {});
  }, []);
  useEffect(() => {
    refresh();
    const id = setInterval(refresh, 3000);
    return () => clearInterval(id);
  }, [refresh]);

  if (!inApp()) {
    return (
      <div className="grid h-full place-items-center p-8">
        <Callout tone="warn">This is the Nira Node desktop app's interface. Open it through the app.</Callout>
      </div>
    );
  }
  if (installed === false) return <Install onDone={refresh} />;
  if (!status || installed === null) return <div className="h-full" />;
  if (!status.nirad_found) {
    return (
      <div className="grid h-full place-items-center p-8">
        <Callout tone="danger">The node program (nirad) was not found. Reinstall the app, or put nirad next to it.</Callout>
      </div>
    );
  }
  if (!status.initialized || !status.has_key) return <Setup onDone={refresh} />;
  if (status.locked && !status.worker_running) return <Unlock onDone={refresh} />;

  const phase = phaseOf(status);
  const tone = phase === "earning" ? "up" : phase === "stopped" ? "faint" : "warn";
  return (
    <div className="flex h-full">
      <nav className="flex w-[232px] shrink-0 flex-col gap-1 border-r border-line/60 bg-surface/40 p-4" aria-label="Main">
        <div className="mb-6 flex items-center gap-3 px-2 pt-1">
          <img src="/brand/nira-lockup.png" alt="Nira" style={{ height: 24, width: Math.round((24 * 228) / 67) }} draggable={false} />
          <span className="display text-[13px] font-semibold text-mute">Node</span>
        </div>
        {TABS.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            aria-current={tab === id ? "page" : undefined}
            className={`pressable flex h-11 items-center gap-3 rounded-full px-4 text-[14px] font-bold ${
              tab === id ? "bg-ink text-canvas" : "text-mute hover:bg-raised hover:text-ink"
            }`}
          >
            <Icon size={18} aria-hidden /> {label}
          </button>
        ))}
        <div className="mt-auto flex items-center gap-2 rounded-full bg-raised px-4 py-2.5 text-[13px] font-semibold">
          <Dot tone={tone} live={phase === "earning" || phase === "syncing"} />
          <span className="truncate">{status.network} · {phase}</span>
        </div>
      </nav>
      <main className="min-w-0 flex-1 overflow-y-auto p-8">
        <div className="mx-auto h-full max-w-[1180px] animate-fade" key={tab}>
          <ErrorBoundary resetKey={tab}>
            {tab === "overview" && <Overview status={status} refresh={refresh} />}
            {tab === "logs" && <Logs />}
            {tab === "settings" && <Settings theme={theme} setTheme={setTheme} onLock={refresh} />}
          </ErrorBoundary>
        </div>
      </main>
    </div>
  );
}
