import { useEffect, useState } from "react";
import { Download, FolderOpen, ShieldCheck } from "lucide-react";
import { api, errText, onProgress, type InstallInfo, type InstallProgress } from "../lib/api";
import { bytes } from "../lib/format";
import { BrandMark, Button, Callout, Card } from "../components/ui";

/** First run: choose the data folder, then download, verify and install nirad. */
export function Install({ onDone }: { onDone: () => void }) {
  const [info, setInfo] = useState<InstallInfo | null>(null);
  const [dir, setDir] = useState("");
  const [busy, setBusy] = useState(false);
  const [prog, setProg] = useState<InstallProgress | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    api.InstallInfo().then((i) => {
      setInfo(i);
      setDir(i.default_dir);
    });
    return onProgress(setProg);
  }, []);

  async function pick() {
    try {
      const d = await api.ChooseFolder(dir);
      if (d) setDir(d);
    } catch (e) {
      setError(errText(e));
    }
  }

  async function install() {
    setBusy(true);
    setError("");
    try {
      await api.Install(dir);
      onDone();
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy(false);
    }
  }

  const pct = prog && prog.total > 0 ? Math.min(100, Math.round((prog.done / prog.total) * 100)) : null;
  if (!info) return <div className="h-full" />;
  return (
    <div className="flex h-full items-center justify-center overflow-y-auto p-8" data-theme="dark">
      <div className="flex w-full max-w-[560px] flex-col gap-6 animate-rise">
        <BrandMark size={52} />
        <div className="flex flex-col gap-2">
          <h1 className="display text-[28px] font-semibold leading-tight">Install Nira Node</h1>
          <p className="text-mute">
            The app downloads the Nira node program ({info.nirad_version}) for {info.platform}, checks it against the release's signed checksums, and keeps
            it and the chain data in the folder you choose.
          </p>
        </div>

        {!info.supported ? (
          <Callout tone="danger">There is no Nira node build for {info.platform} yet.</Callout>
        ) : (
          <>
            <Card className="flex flex-col gap-3 p-5">
              <span className="text-[13px] font-semibold text-mute">Data folder</span>
              <div className="flex items-center gap-3">
                <span className="selectable min-w-0 flex-1 truncate rounded-full bg-raised px-5 py-3 font-mono text-[13px]" title={dir}>
                  {dir}
                </span>
                <Button variant="secondary" icon={<FolderOpen size={16} />} onClick={pick} disabled={busy}>
                  Change
                </Button>
              </div>
              <span className="text-[12px] text-faint">
                Needs room for the chain's current state plus a few GB; a lean node does not grow with the chain's age. The app remembers this in{" "}
                <span className="selectable font-mono">{info.config_path}</span>.
              </span>
            </Card>

            {busy && prog && (
              <Card className="flex flex-col gap-3 p-5" aria-live="polite">
                <div className="flex justify-between text-[13px]">
                  <span className="font-semibold">{prog.stage}</span>
                  <span className="text-mute tabular">{pct !== null ? `${bytes(prog.done)} of ${bytes(prog.total)}` : prog.done ? bytes(prog.done) : ""}</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-raised" role="progressbar" aria-valuenow={pct ?? undefined} aria-valuemin={0} aria-valuemax={100}>
                  <div className={`h-full rounded-full brand-fill transition-[width] duration-300 ${pct === null ? "w-1/3 animate-pulse" : ""}`} style={pct !== null ? { width: `${pct}%` } : undefined} />
                </div>
              </Card>
            )}
            {error && <Callout tone="danger">{error}</Callout>}
            <p className="flex items-center gap-2 text-[13px] text-mute">
              <ShieldCheck size={16} className="text-up" aria-hidden /> Downloaded only from the official Nira release, and refused if its checksum does not match.
            </p>
            <Button size="lg" icon={<Download size={18} />} loading={busy} disabled={!dir} onClick={install}>
              Install
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
