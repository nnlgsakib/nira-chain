import type { Status } from "./api";
import type { Tone } from "../components/ui";

export type NodeState = {
  key: "stopped" | "starting" | "syncing" | "synced";
  label: string;
  detail: string;
  tone: Tone;
  live: boolean;
};

export type VerifyStateKey =
  | "node_stopped"
  | "node_starting"
  | "syncing"
  | "unfunded"
  | "unregistered"
  | "onion"
  | "paused"
  | "verifying"
  | "unbonding";

export type VerifyState = {
  key: VerifyStateKey;
  label: string;
  detail: string;
  tone: Tone;
  live: boolean;
};

/** Derives the local chain-node process state independently of verification. */
export function nodeStateOf(s: Status): NodeState {
  if (!s.node_running) {
    return {
      key: "stopped",
      label: "Stopped",
      detail: "Node is stopped. Start it to sync blocks, connect to peers, and enable community verification.",
      tone: "faint",
      live: false,
    };
  }
  if (!s.reachable) {
    return {
      key: "starting",
      label: "Starting",
      detail: "Opening the node database and local RPC/REST endpoints.",
      tone: "warn",
      live: true,
    };
  }
  if (s.catching_up) {
    return {
      key: "syncing",
      label: "Syncing",
      detail: "Downloading and verifying blocks. Community verification unlocks once the node catches up.",
      tone: "warn",
      live: true,
    };
  }
  return {
    key: "synced",
    label: "Synced",
    detail: "Connected to the network and synced at the latest chain height.",
    tone: "up",
    live: true,
  };
}

/** Derives the community verification workflow & worker state separately from the node. */
export function verifyStateOf(s: Status): VerifyState {
  if (!s.node_running) {
    return {
      key: "node_stopped",
      label: "Node offline",
      detail: "Start the Nira node first to load registration status and run verification.",
      tone: "faint",
      live: false,
    };
  }
  if (!s.reachable) {
    return {
      key: "node_starting",
      label: "Waiting for node",
      detail: "Waiting for the local node RPC/REST API to become ready.",
      tone: "warn",
      live: false,
    };
  }
  if (s.catching_up) {
    return {
      key: "syncing",
      label: "Waiting for sync",
      detail: "Verification can start automatically once the node finishes catching up.",
      tone: "warn",
      live: false,
    };
  }
  if (s.node_status === "UNBONDING") {
    return {
      key: "unbonding",
      label: "Unbonding",
      detail: "Your bond is unbonding and will be returned automatically after the unbonding block.",
      tone: "faint",
      live: false,
    };
  }
  if (s.node_status === "ACTIVE" && !s.has_onion) {
    return {
      key: "onion",
      label: "Setup required",
      detail: "This registered node's selection secret (onion) is on another computer. Create one here to verify from the next epoch.",
      tone: "warn",
      live: false,
    };
  }
  if (s.node_status === "ACTIVE" && s.worker_running) {
    return {
      key: "verifying",
      label: "Verifying",
      detail: "Checking the chain and submitting proofs for assigned community tasks.",
      tone: "up",
      live: true,
    };
  }
  if (s.node_status === "ACTIVE") {
    return {
      key: "paused",
      label: "Paused",
      detail: "Registered and ready. Start verification to prove assigned tasks and earn rewards.",
      tone: "warn",
      live: false,
    };
  }
  if (BigInt(s.balance || "0") >= BigInt(s.min_bond || "0") && s.min_bond) {
    return {
      key: "unregistered",
      label: "Ready to register",
      detail: "Bond NIRA to register your node and begin receiving verification tasks.",
      tone: "warn",
      live: false,
    };
  }
  return {
    key: "unfunded",
    label: "Needs funding",
    detail: "Send at least the minimum bond to your node's address to register for verification.",
    tone: "warn",
    live: false,
  };
}
