import { useEffect, useState } from "react";
import { Check, ChevronRight, KeyRound, Plus, Server } from "lucide-react";
import { api, errText, onEvent, type Network } from "../lib/api";
import { BrandMark, Button, Callout, Card, Field, Spinner } from "../components/ui";

type Step = "network" | "name" | "key" | "phrase" | "run";

/** First run: pick a network, name the node, create or import the key, then the
 *  backend sets up a lean node. Each step is one centred column. */
export function Setup({ onDone }: { onDone: () => void }) {
  const [step, setStep] = useState<Step>("network");
  const [networks, setNetworks] = useState<Network[]>([]);
  const [network, setNetwork] = useState("");
  const [moniker, setMoniker] = useState("");
  const [mode, setMode] = useState<"new" | "import">("new");
  const [pass, setPass] = useState("");
  const [pass2, setPass2] = useState("");
  const [mnemonic, setMnemonic] = useState("");
  const [saved, setSaved] = useState(false);
  const [steps, setSteps] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api.Networks().then(setNetworks).catch((e) => setError(errText(e)));
    return onEvent("setup:step", (s) => setSteps((x) => [...x, s]));
  }, []);

  const index = ["network", "name", "key", "phrase", "run"].indexOf(step);

  async function runSetup() {
    setBusy(true);
    setError("");
    setSteps([]);
    try {
      await api.Setup(network, moniker.trim());
      setStep("key");
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy(false);
    }
  }

  async function makeKey() {
    if (pass.length < 8) return setError("Use at least 8 characters.");
    if (pass !== pass2) return setError("The passwords do not match.");
    setBusy(true);
    setError("");
    try {
      if (mode === "new") {
        setMnemonic(await api.CreateKey(pass));
        setStep("phrase");
      } else {
        await api.ImportKey(mnemonic, pass);
        onDone();
      }
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-full items-center justify-center overflow-y-auto p-8" data-theme="dark">
      <div className="flex w-full max-w-[520px] flex-col gap-6 animate-rise" key={step}>
        <div className="flex items-center justify-between">
          <BrandMark size={44} />
          <div className="flex gap-1.5" aria-label={`Step ${index + 1} of 4`}>
            {[0, 1, 2, 3].map((i) => (
              <span key={i} className={`h-1.5 w-8 rounded-full ${i <= Math.min(index, 3) ? "brand-fill" : "bg-raised"}`} />
            ))}
          </div>
        </div>

        {step === "network" && (
          <>
            <Title title="Run a Nira community node" body="Your computer checks the chain and earns rewards for verified work. It can go offline at any time without penalty." />
            <div className="flex flex-col gap-3" role="radiogroup" aria-label="Network">
              {networks.map((n) => (
                <button
                  key={n.id}
                  role="radio"
                  aria-checked={network === n.id}
                  disabled={!n.launched}
                  onClick={() => setNetwork(n.id)}
                  className={`pressable flex items-center gap-4 rounded-[22px] p-4 text-left ring-2 ${
                    network === n.id ? "bg-surface ring-accent" : "bg-surface ring-transparent hover:bg-raised"
                  } disabled:cursor-not-allowed disabled:opacity-45`}
                >
                  <span className="grid size-11 place-items-center rounded-full bg-raised">
                    <Server size={20} aria-hidden />
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col">
                    <span className="font-bold">{n.name}</span>
                    <span className="text-[13px] text-mute">
                      {n.chain_id} · {n.note}
                    </span>
                  </span>
                  {network === n.id && <Check size={20} className="text-accent-soft" aria-hidden />}
                </button>
              ))}
            </div>
            <Button size="lg" disabled={!network} onClick={() => setStep("name")} icon={<ChevronRight size={18} />}>
              Continue
            </Button>
          </>
        )}

        {step === "name" && (
          <>
            <Title title="Name your node" body="Shown on-chain next to your node. You can't change it later without registering again." />
            <Field label="Node name" value={moniker} maxLength={32} autoFocus placeholder="my-home-node" onChange={(e) => setMoniker(e.target.value)} />
            {busy && (
              <Card className="flex flex-col gap-2 p-5" aria-live="polite">
                {steps.map((s, i) => (
                  <span key={i} className="flex items-center gap-2 text-[13px] text-mute">
                    {i === steps.length - 1 ? <Spinner size={14} /> : <Check size={14} className="text-up" />} {s}
                  </span>
                ))}
              </Card>
            )}
            {error && <Callout tone="danger">{error}</Callout>}
            <div className="flex gap-3">
              <Button variant="ghost" size="lg" onClick={() => setStep("network")} disabled={busy}>
                Back
              </Button>
              <Button size="lg" className="flex-1" loading={busy} disabled={moniker.trim().length < 2} onClick={runSetup}>
                Set up the node
              </Button>
            </div>
          </>
        )}

        {step === "key" && (
          <>
            <Title title="Your node's key" body="It signs your node's work and holds its bond. The password protects it on this computer." />
            <div className="grid grid-cols-2 gap-2 rounded-full bg-surface p-1" role="tablist">
              {(["new", "import"] as const).map((m) => (
                <button
                  key={m}
                  role="tab"
                  aria-selected={mode === m}
                  onClick={() => setMode(m)}
                  className={`pressable flex h-10 items-center justify-center gap-2 rounded-full text-[14px] font-bold ${
                    mode === m ? "bg-ink text-canvas" : "text-mute hover:text-ink"
                  }`}
                >
                  {m === "new" ? <Plus size={16} /> : <KeyRound size={16} />} {m === "new" ? "Create a new key" : "Import a phrase"}
                </button>
              ))}
            </div>
            {mode === "import" && (
              <label className="flex flex-col gap-1.5">
                <span className="text-[13px] font-semibold text-mute">Recovery phrase</span>
                <textarea
                  value={mnemonic}
                  onChange={(e) => setMnemonic(e.target.value)}
                  rows={3}
                  spellCheck={false}
                  className="rounded-[22px] bg-raised p-4 font-mono text-[14px]"
                  placeholder="12 or 24 words separated by spaces"
                />
              </label>
            )}
            <Field label="Password" type="password" value={pass} onChange={(e) => setPass(e.target.value)} hint="At least 8 characters. Needed when the app starts." autoFocus />
            <Field label="Repeat the password" type="password" value={pass2} onChange={(e) => setPass2(e.target.value)} />
            {error && <Callout tone="danger">{error}</Callout>}
            <Button size="lg" loading={busy} disabled={!pass || !pass2} onClick={makeKey}>
              {mode === "new" ? "Create key" : "Import key"}
            </Button>
          </>
        )}

        {step === "phrase" && (
          <>
            <Title title="Write down your recovery phrase" body="It is the only way to recover the key and its bond. Anyone who has it controls them. It is shown once." />
            <ol className="grid grid-cols-3 gap-2 rounded-[26px] bg-surface p-5">
              {mnemonic.split(" ").map((w, i) => (
                <li key={i} className="flex gap-2 rounded-full bg-raised px-3 py-2 text-[14px] font-semibold">
                  <span className="w-5 text-right text-faint tabular">{i + 1}</span>
                  {w}
                </li>
              ))}
            </ol>
            <Callout tone="warn">Never type it into a website or share it. Nira will never ask for it.</Callout>
            <label className="flex cursor-pointer items-center gap-3 text-[14px]">
              <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} className="size-5 appearance-auto accent-[rgb(var(--accent))]" />
              I have written it down somewhere safe
            </label>
            <Button size="lg" disabled={!saved} onClick={() => { setMnemonic(""); onDone(); }}>
              Done
            </Button>
          </>
        )}
      </div>
    </div>
  );
}

const Title = ({ title, body }: { title: string; body: string }) => (
  <div className="flex flex-col gap-2">
    <h1 className="display text-balance text-[28px] font-semibold leading-tight">{title}</h1>
    <p className="text-[15px] text-mute">{body}</p>
  </div>
);
