import { Component, type ReactNode } from "react";
import { RotateCcw } from "lucide-react";
import { Button, Callout } from "./ui";

/** A screen that throws shows its error and a way back, never a blank window. */
export class ErrorBoundary extends Component<{ children: ReactNode; resetKey?: string }, { error: Error | null }> {
  state = { error: null as Error | null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidUpdate(prev: { resetKey?: string }) {
    if (prev.resetKey !== this.props.resetKey && this.state.error) this.setState({ error: null });
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="flex max-w-[640px] flex-col gap-4">
        <Callout tone="danger">
          This screen hit an error: <span className="selectable font-mono">{this.state.error.message}</span>
        </Callout>
        <Button variant="secondary" className="self-start" icon={<RotateCcw size={16} />} onClick={() => this.setState({ error: null })}>
          Try again
        </Button>
      </div>
    );
  }
}
