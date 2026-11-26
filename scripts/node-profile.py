#!/usr/bin/env python3
"""Apply a hardened, role-specific configuration to a Nira node home (G-SPEC-4).

    scripts/node-profile.py <role> [--home ~/.nira] [--sentries id@host:port,...] [--private-ids id,...]

Roles
  validator  behind sentries: PEX off, only persistent peers = its sentries, no public APIs,
             JSON-RPC off, no snapshots, Prometheus on (internal), double-sign check on
  sentry     public P2P, PEX on, validator node IDs kept private (never gossiped),
             persistent peer to the validator, APIs off, generous peer limits
  rpc        full node serving APIs: REST/gRPC/JSON-RPC (eth,net,web3,txpool) on, rate-limit at a proxy
  archive    pruning=nothing, APIs on, snapshots every 1000 blocks for state-sync clients
  community  LEAN full node for x/community proofs (sustainable on consumer hardware): verifies
             every block but keeps only the last N state versions and N blocks, where
             N = max(2000, 2 × (apphash_window + epoch_blocks)) from genesis (or --keep-recent);
             no tx index; modest peer limits; APIs local-only (the prove loop queries 127.0.0.1).
             Blocks actually kept = max(N, consensus evidence max_age_num_blocks), because
             CometBFT keeps evidence-relevant blocks (100 000 ≈ 2.3 days at 2 s): still a fixed
             window. Disk then tracks the size of the current state, not the age of the chain.

The script only edits keys it knows; it prints every change. Re-run safely.
"""
import argparse
import re
import sys
from pathlib import Path

COMMON_CFG = {
    # p2p hygiene for every role
    ("p2p", "addr_book_strict"): "true",
    ("p2p", "max_packet_msg_payload_size"): "1024",
    ("p2p", "send_rate"): "20480000",
    ("p2p", "recv_rate"): "20480000",
    ("instrumentation", "prometheus"): "true",
    ("instrumentation", "prometheus_listen_addr"): '"127.0.0.1:26660"',
    ("rpc", "pprof_laddr"): '""',
}

PROFILES = {
    "validator": {
        "cfg": {
            ("p2p", "pex"): "false",
            ("p2p", "max_num_inbound_peers"): "10",   # only its sentries connect (unconditional)
            ("p2p", "max_num_outbound_peers"): "10",
            ("p2p", "addr_book_strict"): "false",
            ("consensus", "double_sign_check_height"): "10",
            ("rpc", "laddr"): '"tcp://127.0.0.1:26657"',
        },
        "app": {
            ("api", "enable"): "false",
            ("grpc", "enable"): "false",
            ("json-rpc", "enable"): "false",
            ("state-sync", "snapshot-interval"): "0",
            (None, "pruning"): '"default"',
        },
    },
    "sentry": {
        "cfg": {
            ("p2p", "pex"): "true",
            ("p2p", "max_num_inbound_peers"): "80",
            ("p2p", "max_num_outbound_peers"): "20",
            ("rpc", "laddr"): '"tcp://127.0.0.1:26657"',
        },
        "app": {
            ("api", "enable"): "false",
            ("json-rpc", "enable"): "false",
            (None, "pruning"): '"default"',
        },
    },
    "rpc": {
        "cfg": {
            ("p2p", "pex"): "true",
            ("p2p", "max_num_inbound_peers"): "40",
            ("p2p", "max_num_outbound_peers"): "10",
            ("rpc", "max_open_connections"): "900",
            ("rpc", "max_subscription_clients"): "100",
        },
        "app": {
            ("api", "enable"): "true",
            ("grpc", "enable"): "true",
            ("json-rpc", "enable"): "true",
            ("json-rpc", "api"): '"eth,net,web3,txpool"',
            ("json-rpc", "allow-insecure-unlock"): "false",
            ("json-rpc", "allow-unprotected-txs"): "false",
            ("json-rpc", "gas-cap"): "25000000",
            ("json-rpc", "logs-cap"): "10000",
            ("json-rpc", "block-range-cap"): "10000",
            (None, "pruning"): '"default"',
        },
    },
    "archive": {
        "cfg": {("p2p", "pex"): "true"},
        "app": {
            (None, "pruning"): '"nothing"',
            ("api", "enable"): "true",
            ("grpc", "enable"): "true",
            ("json-rpc", "enable"): "true",
            ("state-sync", "snapshot-interval"): "1000",
            ("state-sync", "snapshot-keep-recent"): "2",
        },
    },
    "community": {
        "cfg": {
            ("p2p", "pex"): "true",
            ("p2p", "max_num_inbound_peers"): "20",
            ("p2p", "max_num_outbound_peers"): "10",
            ("rpc", "laddr"): '"tcp://127.0.0.1:26657"',
            ("tx_index", "indexer"): '"null"',   # tx search is an RPC/archive-node job
        },
        "app": {
            # proofs are only ever for heights inside apphash_window + epoch_blocks, so keep a
            # small multiple of that instead of "default" (362 880 versions ≈ 8 days, plus
            # every block forever). The two KEEP values are filled in by lean_keep().
            (None, "pruning"): '"custom"',
            (None, "pruning-keep-recent"): "KEEP",
            (None, "pruning-interval"): '"100"',
            (None, "min-retain-blocks"): "KEEP_INT",
            ("api", "enable"): "true",
            ("api", "address"): '"tcp://127.0.0.1:1317"',
            ("grpc", "address"): '"127.0.0.1:9090"',
            ("json-rpc", "enable"): "false",
        },
    },
}


def set_key(text: str, section, key: str, value: str):
    """Set `key = value` inside [section] (or at top level when section is None)."""
    lines = text.split("\n")
    cur = None
    for i, line in enumerate(lines):
        m = re.match(r"^\[([^\]]+)\]\s*$", line)
        if m:
            cur = m.group(1)
            continue
        if cur == section and re.match(rf"^{re.escape(key)}\s*=", line):
            old = line.split("=", 1)[1].strip()
            if old != value:
                lines[i] = f"{key} = {value}"
                return "\n".join(lines), f"[{section or 'root'}] {key}: {old} → {value}"
            return text, None
    raise KeyError(f"[{section}] {key} not found")


def apply(path: Path, changes: dict) -> list:
    text, log = path.read_text(), []
    for (section, key), value in changes.items():
        text, msg = set_key(text, section, key, value)
        if msg:
            log.append(msg)
    path.write_text(text)
    return log


LEAN_FLOOR = 2000


def lean_keep(home: Path, override: int | None) -> int:
    """Versions/blocks a community node must keep: 2 × (apphash_window + epoch_blocks) from
    genesis, at least LEAN_FLOOR. Proof heights never fall outside apphash_window +
    epoch_blocks (docs/spec/community.md), so this leaves a 2× margin."""
    if override:
        return override
    import json
    try:
        p = json.load(open(home / "config" / "genesis.json"))["app_state"]["community"]["params"]
        need = 2 * (int(p["apphash_window"]) + int(p["epoch_blocks"]))
    except (OSError, KeyError, ValueError):
        need = 0
    return max(LEAN_FLOOR, need)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("role", choices=sorted(PROFILES))
    ap.add_argument("--home", type=Path, default=Path.home() / ".nira")
    ap.add_argument("--sentries", default="", help="validator: id@host:port of its sentries")
    ap.add_argument("--private-ids", default="", help="sentry: node IDs never gossiped (the validator)")
    ap.add_argument("--validator-peer", default="", help="sentry: id@host:port of the validator")
    ap.add_argument("--keep-recent", type=int, default=None,
                    help="community: state versions and blocks to keep (default max(2000, 2×(apphash_window+epoch_blocks)))")
    a = ap.parse_args()

    prof = PROFILES[a.role]
    cfg = dict(COMMON_CFG)
    cfg.update(prof["cfg"])
    if a.role == "validator":
        if not a.sentries:
            sys.exit("validator profile requires --sentries")
        cfg[("p2p", "persistent_peers")] = f'"{a.sentries}"'
        ids = ",".join(x.split("@")[0] for x in a.sentries.split(",") if x)
        cfg[("p2p", "unconditional_peer_ids")] = f'"{ids}"'
    if a.role == "sentry":
        if a.private_ids:
            cfg[("p2p", "private_peer_ids")] = f'"{a.private_ids}"'
            cfg[("p2p", "unconditional_peer_ids")] = f'"{a.private_ids}"'
        if a.validator_peer:
            cfg[("p2p", "persistent_peers")] = f'"{a.validator_peer}"'

    def section_cfg(changes):  # config.toml uses [p2p] etc.; root keys have section None
        return changes

    app_changes = dict(prof["app"])
    if a.role == "community":
        keep = lean_keep(a.home, a.keep_recent)
        app_changes = {k: (f'"{keep}"' if v == "KEEP" else str(keep) if v == "KEEP_INT" else v) for k, v in app_changes.items()}
    log = apply(a.home / "config" / "config.toml", section_cfg(cfg))
    log += apply(a.home / "config" / "app.toml", app_changes)
    print(f"profile '{a.role}' applied to {a.home}:")
    for line in log or ["(already applied)"]:
        print("  " + line)


if __name__ == "__main__":
    main()
