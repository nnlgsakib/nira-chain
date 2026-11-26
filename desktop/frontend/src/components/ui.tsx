import { useEffect, useRef, useState, type ButtonHTMLAttributes, type HTMLAttributes, type InputHTMLAttributes, type ReactNode } from "react";
import { AlertTriangle, Check, Copy, Info } from "lucide-react";

// Kit follows nira-wallet/src/components/ui.tsx: pills for buttons and inputs,
// 26px cards, the mint fill only for the one primary action, text on mint is
// accent-ink, a disabled primary is a neutral raised fill.

export const Spinner = ({ size = 18, className = "" }: { size?: number; className?: string }) => (
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none" aria-hidden className={`animate-spin-fast ${className}`}>
    <circle cx="12" cy="12" r="9" stroke="currentColor" strokeWidth="3" strokeOpacity="0.22" />
    <path d="M21 12a9 9 0 0 0-9-9" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
  </svg>
);

type Variant = "primary" | "secondary" | "ghost" | "danger";
const VARIANT: Record<Variant, string> = {
  primary: "bg-accent text-accent-ink hover:brightness-105 disabled:bg-raised disabled:text-faint",
  secondary: "bg-raised text-ink hover:bg-line disabled:opacity-40",
  ghost: "text-accent-soft hover:bg-raised disabled:opacity-40",
  danger: "bg-down text-white hover:brightness-110 disabled:bg-raised disabled:text-faint",
};
const SIZE = { lg: "h-12 px-6 text-[15px]", md: "h-11 px-5 text-[14px]", sm: "h-9 px-4 text-[13px]" };

export const Button = ({
  variant = "primary",
  size = "md",
  loading = false,
  icon,
  children,
  className = "",
  disabled,
  type = "button",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: Variant;
  size?: keyof typeof SIZE;
  loading?: boolean;
  icon?: ReactNode;
}) => (
  <button
    type={type}
    disabled={disabled || loading}
    aria-busy={loading || undefined}
    className={`pressable inline-flex shrink-0 items-center justify-center gap-2 rounded-full font-bold ${VARIANT[variant]} ${SIZE[size]} ${className}`}
    {...rest}
  >
    {loading ? <Spinner size={16} /> : icon}
    {children}
  </button>
);

export const Card = ({ children, className = "", ...rest }: HTMLAttributes<HTMLDivElement>) => (
  <div className={`rounded-[26px] bg-surface p-6 ${className}`} {...rest}>
    {children}
  </div>
);

export const Section = ({ title, action, children }: { title: string; action?: ReactNode; children: ReactNode }) => (
  <section className="flex flex-col gap-2">
    <div className="flex min-h-9 items-center justify-between gap-3">
      <h2 className="display text-[14px] font-semibold text-mute">{title}</h2>
      {action}
    </div>
    {children}
  </section>
);

/** Label + value, the value selectable when it's worth copying. */
export const Stat = ({
  label,
  value,
  hint,
  copy = false,
  tone,
}: {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  copy?: boolean;
  tone?: "up" | "down" | "warn";
}) => (
  <div className="flex min-w-0 flex-col gap-1">
    <span className="text-[13px] font-medium text-mute">{label}</span>
    <span
      className={`display truncate text-[20px] font-semibold tabular ${copy ? "selectable" : ""} ${
        tone === "up" ? "text-up" : tone === "down" ? "text-down" : tone === "warn" ? "text-warn" : "text-ink"
      }`}
    >
      {value}
    </span>
    {hint && <span className="text-[12px] text-faint">{hint}</span>}
  </div>
);

export type Tone = "up" | "down" | "warn" | "faint";

/** Live state dot: a ring pulses only while the thing is live (status, not decoration). */
export const Dot = ({ tone, live = false }: { tone: Tone; live?: boolean }) => {
  const c = { up: "bg-up", down: "bg-down", warn: "bg-warn", faint: "bg-faint" }[tone];
  return (
    <span className="relative inline-flex size-2.5 shrink-0" aria-hidden>
      {live && <span className={`absolute inset-0 rounded-full ${c} animate-ring`} />}
      <span className={`relative size-2.5 rounded-full ${c}`} />
    </span>
  );
};

/** Compact status pill pairing a live Dot with a readable state label. */
export const StatusPill = ({ tone, live = false, label }: { tone: Tone; live?: boolean; label: string }) => {
  const pill = {
    up: "bg-up/15 text-up ring-up/25",
    down: "bg-down/15 text-down ring-down/25",
    warn: "bg-warn/15 text-warn ring-warn/25",
    faint: "bg-raised text-mute ring-line/60",
  }[tone];
  return (
    <span className={`inline-flex shrink-0 items-center gap-2 rounded-full px-3 py-1 text-[12px] font-bold ring-1 ${pill}`}>
      <Dot tone={tone} live={live} />
      <span>{label}</span>
    </span>
  );
};

/** Copy button with transient "Copied" feedback. */
export const CopyButton = ({
  value,
  label = "Copy",
  size = "sm",
  className = "",
}: {
  value: string;
  label?: string;
  size?: keyof typeof SIZE;
  className?: string;
}) => {
  const [copied, setCopied] = useState(false);
  const timer = useRef<number | null>(null);
  useEffect(() => () => {
    if (timer.current !== null) window.clearTimeout(timer.current);
  }, []);

  return (
    <Button
      variant="secondary"
      size={size}
      className={className}
      disabled={!value}
      icon={copied ? <Check size={14} className="text-up" aria-hidden /> : <Copy size={14} aria-hidden />}
      onClick={() => {
        navigator.clipboard?.writeText(value);
        setCopied(true);
        if (timer.current !== null) window.clearTimeout(timer.current);
        timer.current = window.setTimeout(() => setCopied(false), 1500);
      }}
    >
      {copied ? "Copied" : label}
    </Button>
  );
};

export const Field = ({
  label,
  hint,
  error,
  className = "",
  ...rest
}: InputHTMLAttributes<HTMLInputElement> & { label: string; hint?: string; error?: string }) => (
  <label className={`flex flex-col gap-1.5 ${className}`}>
    <span className="text-[13px] font-semibold text-mute">{label}</span>
    <input
      className={`h-12 rounded-full bg-raised px-5 text-[15px] font-medium text-ink ring-1 ${
        error ? "ring-down" : "ring-transparent focus:ring-accent-soft"
      }`}
      aria-invalid={!!error || undefined}
      {...rest}
    />
    {error ? <span className="px-2 text-[12px] font-medium text-down">{error}</span> : hint && <span className="px-2 text-[12px] text-faint">{hint}</span>}
  </label>
);

/** Info notes are quiet and unboxed; warnings get a box (wallet convention). */
export const Callout = ({ tone = "info", children }: { tone?: "info" | "warn" | "danger"; children: ReactNode }) =>
  tone === "info" ? (
    <p className="flex gap-2 text-[13px] text-mute">
      <Info size={16} className="mt-0.5 shrink-0" aria-hidden />
      <span>{children}</span>
    </p>
  ) : (
    <div
      role={tone === "danger" ? "alert" : undefined}
      className={`flex gap-3 rounded-[22px] p-4 text-[13px] ${tone === "warn" ? "bg-warn/10 text-warn" : "bg-down/10 text-down"}`}
    >
      <AlertTriangle size={18} className="mt-0.5 shrink-0" aria-hidden />
      <span className="text-ink">{children}</span>
    </div>
  );

export const BrandMark = ({ size = 40 }: { size?: number }) => (
  <img src="/brand/nira-tile.png" alt="Nira" width={size} height={size} draggable={false} style={{ width: size, height: size }} />
);
