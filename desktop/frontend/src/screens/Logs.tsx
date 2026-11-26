import { useEffect, useRef, useState } from "react";
import { ArrowDownToLine, Search } from "lucide-react";
import { api, type LogLine } from "../lib/api";
import { Button, Card } from "../components/ui";

type Source = "all" | "node" | "work";

/** Live output of the node and the work loop, newest at the bottom. */
export function Logs() {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [source, setSource] = useState<Source>("all");
  const [query, setQuery] = useState("");
  const [follow, setFollow] = useState(true);
  const end = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let live = true;
    const tick = () => api.Logs(300).then((l) => live && setLines(l ?? [])).catch(() => {});
    tick();
    const id = setInterval(tick, 2000);
    return () => {
      live = false;
      clearInterval(id);
    };
  }, []);

  useEffect(() => {
    if (follow) {
      end.current?.scrollIntoView({ block: "end" });
    }
  }, [lines, follow, source, query]);

  const counts: Record<Source, number> = {
    all: lines.length,
    node: lines.filter((l) => l.source === "node").length,
    work: lines.filter((l) => l.source === "work").length,
  };
  const q = query.trim().toLowerCase();
  const shown = lines.filter((l) => {
    if (source !== "all" && l.source !== source) return false;
    if (q && !l.text.toLowerCase().includes(q) && !l.source.toLowerCase().includes(q)) return false;
    return true;
  });

  return (
    <div className="flex h-full flex-col gap-5">
      <div className="flex flex-col gap-1">
        <h1 className="display text-[22px] font-semibold leading-tight sm:text-[26px]">Activity</h1>
        <p className="text-[13px] text-mute">Live output from the local Nira node and community verification loop.</p>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex gap-1 self-start rounded-full bg-surface p-1 ring-1 ring-line/50" role="tablist" aria-label="Source">
          {(["all", "node", "work"] as const).map((s) => (
            <button
              key={s}
              role="tab"
              aria-selected={source === s}
              onClick={() => setSource(s)}
              className={`pressable h-9 rounded-full px-4 text-[13px] font-bold ${
                source === s ? "bg-ink text-canvas" : "text-mute hover:text-ink"
              }`}
            >
              {s === "all" ? "All" : s === "node" ? "Node" : "Work loop"}
              <span className="ml-1.5 opacity-60 tabular">{counts[s]}</span>
            </button>
          ))}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <label className="flex h-10 items-center gap-2 rounded-full bg-surface px-4 text-[13px] ring-1 ring-line/60 focus-within:ring-accent-soft">
            <Search size={15} className="text-faint" aria-hidden />
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter logs..."
              aria-label="Filter logs"
              className="w-40 text-[13px] text-ink sm:w-56"
            />
          </label>
          <Button variant={follow ? "secondary" : "primary"} size="sm" onClick={() => setFollow((f) => !f)} aria-pressed={follow}>
            {follow ? "Auto-scroll: On" : "Auto-scroll: Paused"}
          </Button>
          {!follow && (
            <Button
              variant="secondary"
              size="sm"
              icon={<ArrowDownToLine size={14} aria-hidden />}
              onClick={() => end.current?.scrollIntoView({ block: "end", behavior: "smooth" })}
            >
              Latest
            </Button>
          )}
        </div>
      </div>

      <Card className="selectable min-h-0 flex-1 overflow-y-auto p-4 font-mono text-[12px] leading-relaxed ring-1 ring-line/50">
        {shown.length === 0 ? (
          <p className="text-faint">
            {lines.length === 0
              ? "Nothing yet. Start the node or verification loop to see output here."
              : "No log lines match the current filter."}
          </p>
        ) : (
          shown.map((l, i) => (
            <div key={i} className="flex gap-3 py-0.5">
              <span className="shrink-0 text-faint tabular">{new Date(l.time).toLocaleTimeString()}</span>
              <span className={`w-10 shrink-0 font-semibold ${l.source === "work" ? "text-accent-soft" : "text-sky"}`}>{l.source}</span>
              <span className="min-w-0 break-all text-mute">{l.text}</span>
            </div>
          ))
        )}
        <div ref={end} />
      </Card>
    </div>
  );
}
