import { useEffect, useRef, useState } from "react";
import { api, type LogLine } from "../lib/api";
import { Card } from "../components/ui";

/** Live output of the node and the work loop, newest at the bottom. */
export function Logs() {
  const [lines, setLines] = useState<LogLine[]>([]);
  const [source, setSource] = useState<"all" | "node" | "work">("all");
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
    end.current?.scrollIntoView({ block: "end" });
  }, [lines]);

  const shown = lines.filter((l) => source === "all" || l.source === source);
  return (
    <div className="flex h-full flex-col gap-4">
      <div className="flex gap-1 self-start rounded-full bg-surface p-1" role="tablist" aria-label="Source">
        {(["all", "node", "work"] as const).map((s) => (
          <button
            key={s}
            role="tab"
            aria-selected={source === s}
            onClick={() => setSource(s)}
            className={`pressable h-9 rounded-full px-4 text-[13px] font-bold ${source === s ? "bg-ink text-canvas" : "text-mute hover:text-ink"}`}
          >
            {s === "all" ? "All" : s === "node" ? "Node" : "Work loop"}
          </button>
        ))}
      </div>
      <Card className="selectable min-h-0 flex-1 overflow-y-auto p-4 font-mono text-[12px] leading-relaxed">
        {shown.length === 0 ? (
          <p className="text-faint">Nothing yet. Start the node to see its output here.</p>
        ) : (
          shown.map((l, i) => (
            <div key={i} className="flex gap-3">
              <span className="shrink-0 text-faint tabular">{new Date(l.time).toLocaleTimeString()}</span>
              <span className={`w-10 shrink-0 ${l.source === "work" ? "text-accent-soft" : "text-sky"}`}>{l.source}</span>
              <span className="min-w-0 break-all text-mute">{l.text}</span>
            </div>
          ))
        )}
        <div ref={end} />
      </Card>
    </div>
  );
}
