import { useEffect, useState } from "react";
import { Check, Lock, Moon, Sun } from "lucide-react";
import { api, errText, type Settings as S, type SystemInfo } from "../lib/api";
import { Button, Callout, Card, CopyButton, Field, Section, Stat } from "../components/ui";
import { short } from "../lib/format";

export function Settings({ theme, setTheme, onLock }: { theme: "dark" | "light"; setTheme: (t: "dark" | "light") => void; onLock: () => void }) {
  const [s, setS] = useState<S | null>(null);
  const [sys, setSys] = useState<SystemInfo | null>(null);
  const [cpus, setCpus] = useState("");
  const [mem, setMem] = useState("");
  const [msg, setMsg] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    api.Settings().then((v) => {
      setS(v);
      setCpus(String(v.max_cpus || ""));
      setMem(String(v.max_memory_mb || ""));
    });
    api.System().then(setSys);
  }, []);

  async function save() {
    setError("");
    setMsg("");
    try {
      await api.SaveLimits(Number(cpus) || 0, Number(mem) || 0);
      setMsg("Saved. Applies the next time the node starts.");
    } catch (e) {
      setError(errText(e));
    }
  }

  if (!s) return null;
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h1 className="display text-[22px] font-semibold leading-tight sm:text-[26px]">Settings</h1>
        <p className="text-[13px] text-mute">Node identity, resource limits, appearance, and key security.</p>
      </div>

      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-2">
        <div className="flex flex-col gap-6">
          <Section title="This node">
            <Card className="flex flex-col gap-5 ring-1 ring-line/50">
              <div className="grid grid-cols-1 gap-5 sm:grid-cols-2">
                <Stat label="Name" value={s.moniker || "—"} />
                <Stat label="Network" value={s.network} />
              </div>
              <div className="flex items-end justify-between gap-3 rounded-[18px] bg-raised/55 p-3.5">
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="text-[12px] font-semibold text-mute">Operator address</span>
                  <span className="selectable truncate font-mono text-[13px]" title={s.operator}>
                    {s.operator ? short(s.operator) : "—"}
                  </span>
                  {s.operator && <span className="selectable truncate font-mono text-[11px] text-faint">{s.operator}</span>}
                </div>
                {s.operator && <CopyButton value={s.operator} />}
              </div>
              <div className="flex items-end justify-between gap-3 rounded-[18px] bg-raised/55 p-3.5">
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="text-[12px] font-semibold text-mute">Data folder</span>
                  <span className="selectable break-all font-mono text-[12px] text-ink">{sys?.data_dir || "—"}</span>
                </div>
                {sys?.data_dir && <CopyButton value={sys.data_dir} />}
              </div>
            </Card>
          </Section>

          <Section title="Appearance and security">
            <Card className="flex flex-col gap-4 ring-1 ring-line/50">
              <div className="flex flex-wrap items-center gap-3">
                <Button
                  variant="secondary"
                  icon={theme === "dark" ? <Sun size={16} aria-hidden /> : <Moon size={16} aria-hidden />}
                  onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
                >
                  {theme === "dark" ? "Light theme" : "Dark theme"}
                </Button>
                <Button
                  variant="secondary"
                  icon={<Lock size={16} aria-hidden />}
                  onClick={async () => {
                    await api.Lock();
                    onLock();
                  }}
                >
                  Lock now
                </Button>
              </div>
              <p className="text-[13px] text-faint">
                Locking forgets the password from memory. An active community verification loop keeps running; starting it again requires the password.
              </p>
            </Card>
          </Section>
        </div>

        <div className="flex flex-col gap-6">
          <Section title="Resource limits">
            <Card className="flex flex-col gap-5 ring-1 ring-line/50">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <Field
                  label="CPU cores"
                  type="number"
                  min={1}
                  max={sys?.cpus}
                  value={cpus}
                  placeholder={String(Math.max(1, Math.floor((sys?.cpus ?? 2) / 2)))}
                  hint={`Of ${sys?.cpus ?? "?"}. Empty = half.`}
                  onChange={(e) => setCpus(e.target.value)}
                />
                <Field
                  label="Memory (MB)"
                  type="number"
                  min={1024}
                  step={512}
                  value={mem}
                  placeholder="4096"
                  hint="Soft limit for the node. Empty = 4096."
                  onChange={(e) => setMem(e.target.value)}
                />
              </div>
              <Callout>
                Limits apply to both the node daemon and the verification worker the next time they start.
              </Callout>
              {msg && (
                <p className="flex items-center gap-2 text-[13px] font-semibold text-up" role="status">
                  <Check size={16} aria-hidden />
                  <span>{msg}</span>
                </p>
              )}
              {error && <Callout tone="danger">{error}</Callout>}
              <Button className="self-start" onClick={save}>
                Save limits
              </Button>
            </Card>
          </Section>

          <Section title="How it runs">
            <Card className="flex flex-col gap-3 text-[13px] text-mute ring-1 ring-line/50">
              <div className="flex justify-between gap-4 border-b border-line/40 pb-2.5">
                <span className="font-semibold text-ink">Chain storage</span>
                <span className="text-right">Lean pruning keeps disk usage flat</span>
              </div>
              <div className="flex justify-between gap-4 border-b border-line/40 pb-2.5">
                <span className="font-semibold text-ink">Process controls</span>
                <span className="text-right">Node and verification start/pause independently</span>
              </div>
              <div className="flex justify-between gap-4">
                <span className="font-semibold text-ink">Key storage</span>
                <span className="text-right">Encrypted file keyring; password stays in memory only</span>
              </div>
            </Card>
          </Section>
        </div>
      </div>
    </div>
  );
}
