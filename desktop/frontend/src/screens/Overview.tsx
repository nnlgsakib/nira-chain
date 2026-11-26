import { useState } from "react";
import { KeyRound, Pause, Play, Power, Server, ShieldCheck, Square, Wallet } from "lucide-react";
import { api, errText, type ExitInfo, type Status } from "../lib/api";
import { ago, bytes, nira, short } from "../lib/format";
import { nodeStateOf, verifyStateOf } from "../lib/status";
import { Button, Callout, Card, CopyButton, Dot, Field, Section, Stat, StatusPill } from "../components/ui";

export function Overview({ status, refresh }: { status: Status; refresh: () => void }) {
  const node = nodeStateOf(status);
  const verify = verifyStateOf(status);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [bond, setBond] = useState("");
  const [confirmLeave, setConfirmLeave] = useState(false);
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
  const verifyActionDisabled = verify.key === "node_stopped" || verify.key === "node_starting" || verify.key === "syncing";

  return (
    <div className="flex flex-1 flex-col gap-6">
      {/* Top dashboard header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="display text-[22px] font-semibold leading-tight sm:text-[26px]">Dashboard</h1>
          <p className="text-[13px] text-mute">
            Manage your local chain node and community verification independently.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2.5">
          {status.operator && (
            <div className="flex items-center gap-2 rounded-full bg-surface py-1 pl-3.5 pr-1.5 ring-1 ring-line/60">
              <span className="text-[12px] font-semibold text-mute">Operator</span>
              <span className="selectable font-mono text-[12px] text-ink" title={status.operator}>
                {short(status.operator)}
              </span>
              <CopyButton value={status.operator} size="sm" className="h-7 px-3 text-[12px]" />
            </div>
          )}
          <div className="flex items-center gap-2 rounded-full bg-surface px-3.5 py-2 text-[12px] font-semibold ring-1 ring-line/60" aria-live="polite">
            <Dot tone={stale ? "warn" : "up"} live={!stale} />
            <span className={stale ? "text-warn" : "text-mute"}>
              {stale ? "Status stale: node unreachable" : `Updated ${ago(status.updated_at)}`}
            </span>
          </div>
        </div>
      </div>

      {error && <Callout tone="danger">{error}</Callout>}

      {/* Separate Node & Community Verification controls */}
      <div className="grid grid-cols-1 items-stretch gap-6 xl:grid-cols-2">
        {/* Nira Node control card */}
        <Card className="flex flex-col gap-5 p-6 ring-1 ring-line/50">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex items-center gap-3.5">
              <div className="grid size-11 shrink-0 place-items-center rounded-2xl bg-raised text-sky">
                <Server size={20} aria-hidden />
              </div>
              <div>
                <h2 className="display text-[17px] font-semibold leading-tight">Nira Node</h2>
                <p className="text-[13px] text-mute">Local chain sync &amp; RPC daemon</p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2.5">
              <StatusPill tone={node.tone} live={node.live} label={node.label} />
              {!status.node_running ? (
                <Button
                  size="md"
                  icon={<Power size={16} aria-hidden />}
                  loading={busy === "start"}
                  disabled={!!busy && busy !== "start"}
                  onClick={act("start", api.StartNode)}
                >
                  Start node
                </Button>
              ) : (
                <Button
                  variant="secondary"
                  size="md"
                  icon={<Square size={15} aria-hidden />}
                  loading={busy === "stop"}
                  disabled={!!busy && busy !== "stop"}
                  onClick={act("stop", api.StopNode)}
                >
                  Stop node
                </Button>
              )}
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <p className="text-[14px] text-mute">{node.detail}</p>
            {status.node_running && !status.reachable && status.error && (
              <p className="text-[12px] text-warn">Status: {status.error}</p>
            )}
            {status.worker_running && (
              <p className="text-[12px] text-faint">Stopping the node also pauses community verification.</p>
            )}
          </div>

          {!status.node_running && status.node_exit && (
            <ExitNote title="The node stopped" exit={status.node_exit} logPath={status.log_path} />
          )}

          <div className="mt-auto flex flex-wrap items-center justify-between gap-3 border-t border-line/50 pt-3 text-[13px] text-mute">
            <span>
              RPC <strong className="font-semibold text-ink">{status.reachable ? "Reachable" : status.node_running ? "Starting" : "Offline"}</strong>
              {status.catching_up ? " · Catching up" : status.reachable ? " · Synced" : ""}
            </span>
            <span>
              Network <strong className="font-semibold text-ink">{status.network}</strong>
            </span>
          </div>
        </Card>

        {/* Community Verification control card */}
        <Card className="flex flex-col gap-5 p-6 ring-1 ring-line/50">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex items-center gap-3.5">
              <div className="grid size-11 shrink-0 place-items-center rounded-2xl bg-raised text-accent-soft">
                <ShieldCheck size={20} aria-hidden />
              </div>
              <div>
                <h2 className="display text-[17px] font-semibold leading-tight">Community Verification</h2>
                <p className="text-[13px] text-mute">Proof work loop, bond &amp; rewards</p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2.5">
              <StatusPill tone={verify.tone} live={verify.live} label={verify.label} />
              {verify.key === "verifying" && (
                <Button
                  variant="secondary"
                  size="md"
                  icon={<Pause size={16} aria-hidden />}
                  loading={busy === "pause"}
                  disabled={!!busy && busy !== "pause"}
                  onClick={act("pause", api.StopWork)}
                >
                  Pause verification
                </Button>
              )}
              {verify.key === "paused" && (
                <Button
                  size="md"
                  icon={<Play size={16} aria-hidden />}
                  loading={busy === "work"}
                  disabled={!!busy && busy !== "work"}
                  onClick={act("work", api.StartWork)}
                >
                  Start verification
                </Button>
              )}
              {verify.key === "onion" && (
                <Button
                  size="md"
                  icon={<KeyRound size={16} aria-hidden />}
                  loading={busy === "onion"}
                  disabled={!!busy && busy !== "onion"}
                  onClick={act("onion", api.RotateOnion)}
                >
                  Set up this computer
                </Button>
              )}
              {verifyActionDisabled && (
                <Button size="md" icon={<Play size={16} aria-hidden />} disabled title="Start and sync the Nira node first">
                  Start verification
                </Button>
              )}
            </div>
          </div>

          <p className="text-[14px] text-mute">{verify.detail}</p>

          {verify.key === "verifying" && status.locked && (
            <Callout>Key is locked — verification continues running until stopped.</Callout>
          )}

          {(verify.key === "unfunded" || verify.key === "unregistered") && (
            <div className="flex flex-col gap-4 rounded-[20px] bg-canvas/55 p-4 ring-1 ring-line/50">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="text-[12px] font-semibold text-mute">Node operator address</span>
                  <span className="selectable truncate font-mono text-[13px]">{status.operator}</span>
                </div>
                <CopyButton value={status.operator} />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <Stat label="Available balance" value={`${nira(status.balance)} NIRA`} />
                <Stat label="Minimum bond" value={`${nira(status.min_bond)} NIRA`} />
              </div>
              {verify.key === "unregistered" && (
                <div className="flex flex-wrap items-end gap-3">
                  <Field
                    label="Bond (NIRA)"
                    type="number"
                    min={minBond}
                    value={bond}
                    placeholder={String(minBond)}
                    className="min-w-[180px] flex-1"
                    hint="Locked while registered. Higher bond increases selection chance."
                    onChange={(e) => setBond(e.target.value)}
                  />
                  <Button
                    size="lg"
                    icon={<Wallet size={18} aria-hidden />}
                    loading={busy === "register"}
                    disabled={!bond || Number(bond) < minBond || (!!busy && busy !== "register")}
                    onClick={act("register", () => api.Register(Math.floor(Number(bond))))}
                  >
                    Register node
                  </Button>
                </div>
              )}
            </div>
          )}

          {status.node_running && !status.worker_running && status.work_exit && status.node_status === "ACTIVE" && (
            <ExitNote title="The verification loop stopped" exit={status.work_exit} logPath={status.log_path} />
          )}

          <div className="mt-auto flex flex-wrap items-center justify-between gap-3 border-t border-line/50 pt-3 text-[13px] text-mute">
            <span>
              Epoch <strong className="font-semibold text-ink tabular">{status.epoch || "—"}</strong>
              {status.next_epoch_height
                ? ` · Next at block ${Number(status.next_epoch_height).toLocaleString("en-US")}`
                : ""}
            </span>
            {status.node_status === "ACTIVE" && (
              <div className="flex flex-wrap items-center gap-2">
                {!confirmLeave ? (
                  <Button variant="ghost" size="sm" disabled={!!busy} onClick={() => setConfirmLeave(true)}>
                    Leave network
                  </Button>
                ) : (
                  <>
                    <span className="text-[12px] font-semibold text-warn">Unbond and leave?</span>
                    <Button
                      variant="danger"
                      size="sm"
                      loading={busy === "dereg"}
                      onClick={act("dereg", async () => {
                        await api.Deregister();
                        setConfirmLeave(false);
                      })}
                    >
                      Confirm leave
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      disabled={busy === "dereg"}
                      onClick={() => setConfirmLeave(false)}
                    >
                      Cancel
                    </Button>
                  </>
                )}
              </div>
            )}
          </div>
        </Card>
      </div>

      {/* Telemetry grid */}
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
        <Section title="Chain">
          <Card className="grid grid-cols-2 gap-5 ring-1 ring-line/50">
            <Stat
              label="Block height"
              value={status.height ? status.height.toLocaleString("en-US") : "—"}
              hint={status.block_time ? `Block ${ago(status.block_time)}` : undefined}
            />
            <Stat
              label="Peers"
              value={status.peers}
              tone={status.reachable && status.peers === 0 ? "warn" : undefined}
              hint={status.reachable ? "P2P connections" : "Waiting for node"}
            />
            <Stat label="Network" value={status.network} hint={status.chain_id} />
            <Stat label="Disk used" value={bytes(status.disk_bytes)} hint="Lean pruning: stays flat" />
          </Card>
        </Section>

        <Section title="Community node">
          <Card className="grid grid-cols-2 gap-5 ring-1 ring-line/50">
            <Stat
              label="Registration"
              value={
                status.node_status
                  ? status.node_status.toLowerCase()
                  : verifyActionDisabled
                    ? "waiting for node"
                    : "not registered"
              }
              tone={status.node_status === "ACTIVE" ? "up" : undefined}
            />
            <Stat label="Bond" value={status.bond ? `${nira(status.bond)} NIRA` : "—"} />
            <Stat
              label="Epoch"
              value={status.epoch || "—"}
              hint={status.next_epoch_height ? `Next at block ${Number(status.next_epoch_height).toLocaleString("en-US")}` : undefined}
            />
            <Stat label="Verified work" value={status.units_total || "0"} hint="Proofs accepted by chain" />
          </Card>
        </Section>

        <Section title="Rewards &amp; balance">
          <Card className="grid grid-cols-2 gap-5 ring-1 ring-line/50">
            <Stat label="Earned" value={`${nira(status.rewards_total)} NIRA`} tone="up" hint="Paid per verified proof" />
            <Stat
              label="Slashed"
              value={`${nira(status.slashed_total)} NIRA`}
              tone={status.slashed_total && status.slashed_total !== "0" ? "down" : undefined}
            />
            <Stat label="Balance" value={`${nira(status.balance)} NIRA`} />
            {verify.key === "unbonding" ? (
              <Stat label="Bond returns at" value={`block ${status.unbond_complete_height}`} />
            ) : (
              <Stat label="Minimum bond" value={status.min_bond ? `${nira(status.min_bond)} NIRA` : "—"} />
            )}
          </Card>
        </Section>
      </div>
    </div>
  );
}

/** Why a process stopped: its error and last output lines, plus where the full log is. */
function ExitNote({ title, exit, logPath }: { title: string; exit: ExitInfo; logPath: string }) {
  const tail = (exit.tail ?? []).slice(-6);
  return (
    <div role="alert" className="flex flex-col gap-3 rounded-[20px] bg-down/10 p-4 ring-1 ring-down/25">
      <p className="text-[13px] font-bold text-down">
        {title}: <span className="selectable font-mono font-medium">{exit.error}</span>
      </p>
      {tail.length > 0 && (
        <pre className="selectable max-h-40 overflow-auto whitespace-pre-wrap break-all rounded-[14px] bg-canvas/60 p-3 font-mono text-[12px] text-mute">
          {tail.join("\n")}
        </pre>
      )}
      <p className="text-[12px] text-faint">
        Full log: <span className="selectable font-mono">{logPath}</span>
      </p>
    </div>
  );
}
