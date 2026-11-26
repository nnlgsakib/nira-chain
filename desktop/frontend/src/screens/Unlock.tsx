import { useState } from "react";
import { Lock } from "lucide-react";
import { api, errText } from "../lib/api";
import { BrandMark, Button, Field } from "../components/ui";

/** The key's password is asked once per app start; it stays in memory only. */
export function Unlock({ onDone }: { onDone: () => void }) {
  const [pass, setPass] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api.Unlock(pass);
      onDone();
    } catch (err) {
      setError(errText(err));
      setPass("");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-full items-center justify-center p-8" data-theme="dark">
      <form onSubmit={submit} className="flex w-full max-w-[400px] flex-col items-center gap-6 animate-rise">
        <BrandMark size={64} />
        <div className="flex flex-col items-center gap-1 text-center">
          <h1 className="display text-[24px] font-semibold">Unlock your node</h1>
          <p className="text-mute">Enter the password of your node's key.</p>
        </div>
        <Field label="Password" type="password" value={pass} autoFocus className="w-full" error={error} onChange={(e) => setPass(e.target.value)} />
        <Button type="submit" size="lg" className="w-full" loading={busy} disabled={!pass} icon={<Lock size={16} />}>
          Unlock
        </Button>
      </form>
    </div>
  );
}
