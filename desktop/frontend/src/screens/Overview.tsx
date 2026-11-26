import { useState } from "react";
import { Copy, KeyRound, Pause, Play, Power, Square, Wallet } from "lucide-react";
import { api, errText, type ExitInfo, type Status } from "../lib/api";
import { ago, bytes, nira } from "../lib/format";
import { Button, Callout, Card, Dot, Field, Section, Stat } from "../components/ui";

type Phase = "stopped" | "starting" | "syncing" | "unfunded" | "unregistered" | "onion" | "paused" | "earning" | "unbonding";

export function phaseOf(s: Status): Phase {
  if (!s.node_running) return "stopped";
  if (!s.reachable) return "starting";
  if (s.catching_up) return "syncing";
  if (s.node_status === "UNBONDING") return "unbonding";
  if (s.node_status === "ACTIVE" && !s.has_onion) return "onion";
  if (s.node_status === "ACTIVE") return s.worker_running ? "earning" : "paused";
  return BigInt(s.balance || "0") >= BigInt(s.min_bond || "0") && s.min_bond ? "unregistered" : "unfunded";
}

const HERO: Record<Phase, { title: string; body: string; tone: "up" | "down" | "warn" | "faint"; live: boolean }> = {
  stopped: { title: "Node stopped", body: "Start it to sync the chain and do work.", tone: "faint", live: false },
  starting: { title: "Starting", body: "The node is opening its database.", tone: "warn", live: true },
  syncing: { title: "Syncing", body: "Downloading and checking blocks. Work starts when it has caught up.", tone: "warn", live: true },
  unfunded: { title: "Fund your node", body: "Send at least the minimum bond to your node's address to register.", tone: "warn", live: false },
  unregistered: { title: "Ready to register", body: "Bond NIRA to start being selected for verified work.", tone: "warn", live: false },
  onion: {
    title: "Set up this computer",
    body: "This node is registered, but its selection secret (the onion) is on another computer. Make a new one here; it counts from the next epoch.",
    tone: "warn",
    live: false,
  },
  paused: { title: "Registered, not working", body: "Start the work loop to prove your assigned tasks.", tone: "warn", live: false },
  earning: { title: "Working", body: "Checking the chain and proving tasks when selected.", tone: "up", live: true },
  unbonding: { title: "Leaving", body: "Your bond is unbonding and will be returned automatically.", tone: "faint", live: false },
};

export function Overview({ status, refresh }: { status: Status; refresh: () => void }) {
  const phase = phaseOf(status);
  const hero = HERO[phase];
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [bond, setBond] = useState("");
  const stale = Date.now() - Date.parse(status.updated_at) > 10_000;

  const act = (name: string, fn: () => Promise<unknown>) => async () => {
    setBusy(name);
    setError("");
    try {
      await fn();
      refresh();
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy("");
    }
  };

  const minBond = Number(nira(status.min_bond, 0).replace(/,/g, "")) || 0;

  return (
    <div className="flex flex-col gap-6">
      {/* status hero */}
      <Card className="flex items-center gap-6 p-7">
        <div className="relative grid size-16 shrink-0 place-items-center rounded-full bg-raised">
          <Dot tone={hero.tone} live={hero.live} />
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <h1 className="display text-[26px] font-semibold leading-tight">{hero.title}</h1>
          <p className="text-mute">{hero.body}</p>
          <p className={`text-[12px] ${stale ? "text-warn" : "text-faint"}`} aria-live="polite">
            {stale ? "Status is stale: the app could not reach the node." : `Updated ${ago(status.updated_at)}`}
          </p>
        </div>
        <div className="flex shrink-0 gap-2">
          {phase === "stopped" ? (
            <Button size="lg" icon={<Power size={18} />} loading={busy === "start"} onClick={act("start", api.StartNode)}>
              Start node
            </Button>
          ) : (
            <>
              {phase === "onion" && (
                <Button size="lg" icon={<KeyRound size={18} />} loading={busy === "onion"} onClick={act("onion", api.RotateOnion)}>
                  Set up this computer
                </Button>
              )}
              {phase === "paused" && (
                <Button size="lg" icon={<Play size={18} />} loading={busy === "work"} onClick={act("work", api.StartWork)}>
                  Start working
                </Button>
              )}
              {phase === "earning" && (
                <Button variant="secondary" size="lg" icon={<Pause size={18} />} loading={busy === "pause"} onClick={act("pause", api.StopWork)}>
                  Pause work
                </Button>
              )}
              <Button variant="secondary" size="lg" icon={<Square size={16} />} loading={busy === "stop"} onClick={act("stop", api.StopNode)}>
                Stop
              </Button>
            </>
          )}
        </div>
      </Card>
      {error && <Callout tone="danger">{error}</Callout>}
      {phase === "stopped" && status.node_exit && <ExitNote title="The node stopped" exit={status.node_exit} logPath={status.log_path} />}
      {status.node_running && !status.worker_running && status.work_exit && status.node_status === "ACTIVE" && (
        <ExitNote title="The work loop stopped" exit={status.work_exit} logPath={status.log_path} />
      )}

      {/* register */}
      {(phase === "unfunded" || phase === "unregistered") && (
        <Section title="Register">
          <Card className="flex flex-col gap-5">
            <div className="flex items-end gap-4">
              <div className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="text-[13px] font-semibold text-mute">Your node's address</span>
                <span className="selectable truncate font-mono text-[14px]">{status.operator}</span>
              </div>
              <Button variant="secondary" size="sm" icon={<Copy size={14} />} onClick={() => navigator.clipboard?.writeText(status.operator)}>
                Copy
              </Button>
            </div>
            <div className="grid grid-cols-2 gap-4">
              <Stat label="Balance" value={`${nira(status.balance)} NIRA`} />
              <Stat label="Minimum bond" value={`${nira(status.min_bond)} NIRA`} />
            </div>
            {phase === "unregistered" && (
              <div className="flex items-end gap-3">
                <Field
                  label="Bond (NIRA)"
                  type="number"
                  min={minBond}
                  value={bond}
                  placeholder={String(minBond)}
                  className="flex-1"
                  hint="Locked while registered. Your chance of being selected grows with your bond."
                  onChange={(e) => setBond(e.target.value)}
                />
                <Button
                  size="lg"
                  icon={<Wallet size={18} />}
                  loading={busy === "register"}
                  disabled={!bond || Number(bond) < minBond}
                  onClick={act("register", () => api.Register(Math.floor(Number(bond))))}
                >
                  Register
                </Button>
              </div>
            )}
          </Card>
        </Section>
      )}

      {/* node figures */}
      <div className="grid grid-cols-2 gap-6 xl:grid-cols-3">
        <Section title="Chain">
          <Card className="grid grid-cols-2 gap-5">
            <Stat label="Block height" value={status.height ? status.height.toLocaleString("en-US") : "—"} hint={status.block_time ? `Block ${ago(status.block_time)}` : undefined} />
            <Stat label="Peers" value={status.peers} tone={status.reachable && status.peers === 0 ? "warn" : undefined} />
            <Stat label="Network" value={status.network} hint={status.chain_id} />
            <Stat label="Disk used" value={bytes(status.disk_bytes)} hint="Lean node: stays flat as the chain ages" />
          </Card>
        </Section>
        <Section title="Community node">
          <Card className="grid grid-cols-2 gap-5">
            <Stat label="Status" value={status.node_status ? status.node_status.toLowerCase() : "not registered"} tone={status.node_status === "ACTIVE" ? "up" : undefined} />
            <Stat label="Bond" value={status.bond ? `${nira(status.bond)} NIRA` : "—"} />
            <Stat label="Epoch" value={status.epoch || "—"} hint={status.next_epoch_height ? `Next at block ${Number(status.next_epoch_height).toLocaleString("en-US")}` : undefined} />
            <Stat label="Verified work" value={status.units_total || "0"} hint="Proofs accepted by the chain" />
          </Card>
        </Section>
        <Section title="Rewards">
          <Card className="grid grid-cols-2 gap-5">
            <Stat label="Earned" value={`${nira(status.rewards_total)} NIRA`} tone="up" hint="Paid per verified proof" />
            <Stat label="Slashed" value={`${nira(status.slashed_total)} NIRA`} tone={status.slashed_total && status.slashed_total !== "0" ? "down" : undefined} />
            <Stat label="Balance" value={`${nira(status.balance)} NIRA`} />
            {phase === "unbonding" && <Stat label="Bond returns at" value={`block ${status.unbond_complete_height}`} />}
          </Card>
        </Section>
      </div>

      {status.node_status === "ACTIVE" && (
        <div className="flex justify-end">
          <Button
            variant="ghost"
            size="sm"
            loading={busy === "dereg"}
            onClick={() => {
              if (confirm("Leave the network? Your bond is returned after the unbonding period and you stop earning.")) act("dereg", api.Deregister)();
            }}
          >
            Leave the network
          </Button>
        </div>
      )}
    </div>
  );
}

/** Why a process stopped: its error and last output lines, plus where the full log is. */
function ExitNote({ title, exit, logPath }: { title: string; exit: ExitInfo; logPath: string }) {
  const tail = (exit.tail ?? []).slice(-6);
  return (
    <div role="alert" className="flex flex-col gap-3 rounded-[22px] bg-down/10 p-5">
      <p className="text-[14px] font-bold text-down">
        {title}: <span className="selectable font-mono font-medium">{exit.error}</span>
      </p>
      {tail.length > 0 && (
        <pre className="selectable max-h-48 overflow-auto whitespace-pre-wrap break-all rounded-[16px] bg-canvas/60 p-3 font-mono text-[12px] text-mute">{tail.join("\n")}</pre>
      )}
      <p className="text-[12px] text-faint">
        Full log: <span className="selectable font-mono">{logPath}</span>
      </p>
    </div>
  );
}
