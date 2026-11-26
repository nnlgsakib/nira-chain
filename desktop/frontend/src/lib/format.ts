const WEI = 10n ** 18n;

/** anira (base units, decimal string) → NIRA with up to `dp` decimals. */
export function nira(anira: string | undefined, dp = 4): string {
  if (!anira) return "0";
  let v: bigint;
  try {
    v = BigInt(anira.split(".")[0]);
  } catch {
    return "0";
  }
  const whole = v / WEI;
  const frac = (v % WEI).toString().padStart(18, "0").slice(0, dp).replace(/0+$/, "");
  return whole.toLocaleString("en-US") + (frac ? "." + frac : "");
}

export function bytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 1 : 0)} ${units[i]}`;
}

/** "12 s ago", "3 min ago". */
export function ago(iso: string | undefined, now = Date.now()): string {
  if (!iso) return "never";
  const t = Date.parse(iso);
  if (Number.isNaN(t) || t <= 0) return "never";
  const s = Math.max(0, Math.round((now - t) / 1000));
  if (s < 60) return `${s} s ago`;
  if (s < 3600) return `${Math.round(s / 60)} min ago`;
  return `${Math.round(s / 3600)} h ago`;
}

export const short = (addr: string) => (addr.length > 20 ? `${addr.slice(0, 10)}…${addr.slice(-6)}` : addr);
