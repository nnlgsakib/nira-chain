import { useEffect, useState } from "react";
import { Lock, Moon, Sun } from "lucide-react";
import { api, errText, type Settings as S, type SystemInfo } from "../lib/api";
import { Button, Callout, Card, Field, Section, Stat } from "../components/ui";
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
    <div className="flex max-w-[760px] flex-col gap-6">
      <Section title="This node">
        <Card className="grid grid-cols-2 gap-5">
          <Stat label="Name" value={s.moniker || "—"} />
          <Stat label="Network" value={s.network} />
          <Stat label="Address" value={short(s.operator)} copy hint={<span className="selectable font-mono">{s.operator}</span>} />
          <Stat label="Data folder" value={<span className="text-[14px]">{sys?.data_dir}</span>} copy />
        </Card>
      </Section>

      <Section title="Resource limits">
        <Card className="flex flex-col gap-5">
          <div className="grid grid-cols-2 gap-4">
            <Field label="CPU cores" type="number" min={1} max={sys?.cpus} value={cpus} placeholder={String(Math.max(1, Math.floor((sys?.cpus ?? 2) / 2)))} hint={`Of ${sys?.cpus ?? "?"}. Empty = half.`} onChange={(e) => setCpus(e.target.value)} />
            <Field label="Memory (MB)" type="number" min={1024} step={512} value={mem} placeholder="4096" hint="Soft limit for the node. Empty = 4096." onChange={(e) => setMem(e.target.value)} />
          </div>
          {msg && <Callout>{msg}</Callout>}
          {error && <Callout tone="danger">{error}</Callout>}
          <Button className="self-start" onClick={save}>
            Save limits
          </Button>
        </Card>
      </Section>

      <Section title="Appearance and security">
        <Card className="flex flex-wrap items-center gap-3">
          <Button variant="secondary" icon={theme === "dark" ? <Sun size={16} /> : <Moon size={16} />} onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
            {theme === "dark" ? "Light theme" : "Dark theme"}
          </Button>
          <Button
            variant="secondary"
            icon={<Lock size={16} />}
            onClick={async () => {
              await api.Lock();
              onLock();
            }}
          >
            Lock now
          </Button>
          <p className="basis-full text-[13px] text-faint">Locking forgets the password. A running work loop keeps going; starting it again needs the password.</p>
        </Card>
      </Section>
    </div>
  );
}
