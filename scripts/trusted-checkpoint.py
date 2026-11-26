#!/usr/bin/env python3
"""Obtain a trusted checkpoint from several independent RPC servers (G-CONS-3, TM-04).

A node that syncs from snapshots, or that was offline longer than the unbonding
period (21 days), must not trust a single server: a long-range attacker could
show it a fake history. This tool asks every given server for the block at a
recent height, requires them ALL to agree on the hash and on the validator set,
and only then prints (or writes) the state-sync trust settings.

  scripts/trusted-checkpoint.py --rpc http://a:26657 --rpc http://b:26657 [--rpc ...]
        [--offset 1000] [--max-age-hours 168] [--write ~/.nira/config/config.toml]

Exit codes: 0 agreed · 2 servers disagree (possible attack: do not sync) · 3 unusable input.
"""
import argparse
import json
import re
import sys
import time
import urllib.request
from datetime import datetime, timezone


def get(url):
    with urllib.request.urlopen(url, timeout=10) as r:
        return json.load(r)["result"]


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--rpc", action="append", required=True, help="CometBFT RPC URL (repeat; use ≥ 2 independent operators)")
    ap.add_argument("--offset", type=int, default=1000, help="blocks below the lowest reported head")
    ap.add_argument("--max-age-hours", type=float, default=168, help="refuse checkpoints older than this (≤ unbonding period)")
    ap.add_argument("--write", help="config.toml to update ([statesync] enable, rpc_servers, trust_height, trust_hash)")
    a = ap.parse_args()
    if len(a.rpc) < 2:
        sys.exit("need at least 2 independent --rpc servers")

    status = {}
    for u in a.rpc:
        try:
            status[u] = get(f"{u}/status")
        except Exception as e:
            print(f"unreachable: {u}: {e}", file=sys.stderr)
    if len(status) < 2:
        sys.exit(3)
    chains = {s["node_info"]["network"] for s in status.values()}
    if len(chains) != 1:
        print(f"DISAGREE: servers report different chain IDs {chains}", file=sys.stderr)
        sys.exit(2)
    heads = [int(s["sync_info"]["latest_block_height"]) for s in status.values()]
    earliest = max(int(s["sync_info"]["earliest_block_height"]) for s in status.values())
    h = max(earliest, min(heads) - a.offset)

    views = {}
    for u in status:
        blk = get(f"{u}/block?height={h}")
        vals = get(f"{u}/validators?height={h}&per_page=100")
        vset = sorted((v["address"], v["voting_power"]) for v in vals["validators"])
        views[u] = (blk["block_id"]["hash"], blk["block"]["header"]["time"], blk["block"]["header"]["validators_hash"], tuple(vset))
    distinct = {v[:3] for v in views.values()}
    if len(distinct) != 1 or len({v[3] for v in views.values()}) != 1:
        print(f"DISAGREE at height {h}: servers return different blocks or validator sets — DO NOT SYNC", file=sys.stderr)
        for u, v in views.items():
            print(f"  {u}: hash {v[0]} validators_hash {v[2]}", file=sys.stderr)
        sys.exit(2)
    block_hash, block_time, _, vset = next(iter(views.values()))
    t = datetime.fromisoformat(re.sub(r"\.(\d{6})\d*", r".\1", block_time).replace("Z", "+00:00"))
    age_h = (datetime.now(timezone.utc) - t).total_seconds() / 3600
    if age_h > a.max_age_hours:
        print(f"checkpoint at {h} is {age_h:.0f} h old (> {a.max_age_hours} h): use servers that are synced", file=sys.stderr)
        sys.exit(3)
    out = {"chain_id": chains.pop(), "trust_height": h, "trust_hash": block_hash, "block_time": block_time,
           "age_hours": round(age_h, 2), "servers_agreeing": len(views), "validators": len(vset)}
    print(json.dumps(out, indent=1))

    if a.write:
        cfg = open(a.write).read()
        head, sep, body = cfg.partition("[statesync]")
        if not sep:
            sys.exit(f"{a.write}: no [statesync] section")
        for k, v in {"enable": "true", "rpc_servers": f'"{",".join(status)}"', "trust_height": str(h),
                     "trust_hash": f'"{block_hash}"', "trust_period": '"168h0m0s"'}.items():
            body = re.sub(rf"^{k} = .*", f"{k} = {v}", body, count=1, flags=re.M)
        open(a.write, "w").write(head + sep + body)
        print(f"wrote [statesync] to {a.write}", file=sys.stderr)


if __name__ == "__main__":
    main()
