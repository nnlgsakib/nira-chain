#!/usr/bin/env python3
"""On-chain security monitor for Nira (G-OPS-3, threat-model detection column).

Polls a node's CometBFT RPC and REST API and emits one JSON line per alert:

  halt                 no new block for --halt-after seconds              (TM-01/02/08)
  slash                slashing event: double sign or downtime            (TM-03)
  jailed               a bonded validator became jailed                    (TM-01)
  power_shift          a validator's share of bonded power moved > --power-delta points (TM-05)
  power_concentration  a validator above --max-share of bonded power      (TM-05)
  gov_proposal         a new governance proposal was submitted           (TM-19)
  upgrade_plan         a software-upgrade plan is scheduled              (TM-19)
  comm_hook_error      an x/community block-hook step failed (INV-COMM-8)
  comm_slash           a community node was slashed (G-COMM-1)
  divergence_threshold community nodes reported an app-hash divergence  (TM-11)
  supply_jump          total supply changed by more than --supply-delta per block window

Usage: scripts/monitor.py [--rpc URL] [--rest URL] [--interval 5] [--once]
Pipe the output to your alerting (e.g. `| tee alerts.jsonl`, a webhook forwarder, or Alertmanager).
"""
import argparse
import json
import sys
import time
import urllib.request
from decimal import Decimal


def get(url):
    with urllib.request.urlopen(url, timeout=10) as r:
        return json.load(r)


def alert(kind, **kw):
    print(json.dumps({"ts": int(time.time()), "alert": kind, **kw}), flush=True)


BLOCK_EVENTS = {
    "slash": "slash",
    "comm_hook_error": "comm_hook_error",
    "community_node_slashed": "comm_slash",
    "divergence_threshold": "divergence_threshold",
}


def scan_block(rpc, h):
    res = get(f"{rpc}/block_results?height={h}")["result"]
    events = list(res.get("finalize_block_events") or [])
    for tx in res.get("txs_results") or []:
        events += tx.get("events") or []
    for e in events:
        kind = BLOCK_EVENTS.get(e["type"])
        if kind:
            attrs = {a["key"]: a["value"] for a in e.get("attributes", []) if a.get("key")}
            attrs.pop("mode", None)
            alert(kind, height=h, **attrs)


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--rpc", default="http://127.0.0.1:26657")
    ap.add_argument("--rest", default="http://127.0.0.1:1317")
    ap.add_argument("--interval", type=float, default=5)
    ap.add_argument("--halt-after", type=float, default=30)
    ap.add_argument("--power-delta", type=Decimal, default=Decimal("5"), help="percentage points")
    ap.add_argument("--max-share", type=Decimal, default=Decimal("33.3"), help="percent")
    ap.add_argument("--supply-delta", type=Decimal, default=Decimal("0.001"), help="fraction")
    ap.add_argument("--once", action="store_true", help="one polling round (for tests)")
    a = ap.parse_args()

    last_h = None
    last_change = time.time()
    halted = False
    scanned = None
    shares, jailed, proposals, plan, supply = {}, set(), None, None, None
    while True:
        try:
            h = int(get(f"{a.rpc}/status")["result"]["sync_info"]["latest_block_height"])
            now = time.time()
            if h != last_h:
                last_h, last_change = h, now
                if halted:
                    alert("resumed", height=h)
                halted = False
            elif not halted and now - last_change > a.halt_after:
                halted = True
                alert("halt", height=h, seconds=int(now - last_change))

            # block events since last scan (bounded catch-up)
            start = h if scanned is None else max(scanned + 1, h - 50)
            for bh in range(start, h + 1):
                scan_block(a.rpc, bh)
            scanned = h

            vals = get(f"{a.rest}/cosmos/staking/v1beta1/validators?pagination.limit=500")["validators"]
            bonded = [v for v in vals if v["status"] == "BOND_STATUS_BONDED"]
            total = sum(Decimal(v["tokens"]) for v in bonded) or Decimal(1)
            for v in vals:
                op, name = v["operator_address"], v["description"]["moniker"]
                share = Decimal(v["tokens"]) * 100 / total if v["status"] == "BOND_STATUS_BONDED" else Decimal(0)
                if op in shares and abs(share - shares[op]) > a.power_delta:
                    alert("power_shift", validator=name, operator=op, before=f"{shares[op]:.2f}", after=f"{share:.2f}")
                if share > a.max_share and (op not in shares or shares[op] <= a.max_share):
                    alert("power_concentration", validator=name, operator=op, share=f"{share:.2f}")
                shares[op] = share
                if v["jailed"] and op not in jailed:
                    alert("jailed", validator=name, operator=op)
                    jailed.add(op)
                elif not v["jailed"]:
                    jailed.discard(op)

            props = get(f"{a.rest}/cosmos/gov/v1/proposals?pagination.limit=500&pagination.reverse=true")["proposals"]
            ids = {int(p["id"]) for p in props}
            if proposals is not None:
                for p in props:
                    if int(p["id"]) not in proposals:
                        alert("gov_proposal", id=p["id"], title=p.get("title", ""), status=p["status"],
                              messages=[m.get("@type") for m in p.get("messages", [])])
            proposals = ids

            cp = get(f"{a.rest}/cosmos/upgrade/v1beta1/current_plan").get("plan")
            key = f"{cp['name']}@{cp['height']}" if cp else None
            if key and key != plan:
                alert("upgrade_plan", name=cp["name"], height=cp["height"])
            plan = key

            sup = Decimal(get(f"{a.rest}/cosmos/bank/v1beta1/supply/by_denom?denom=anira")["amount"]["amount"])
            if supply is not None and supply > 0 and abs(sup - supply) / supply > a.supply_delta:
                alert("supply_jump", before=str(supply), after=str(sup))
            supply = sup
        except Exception as e:  # node unreachable counts toward halt detection
            if not halted and time.time() - last_change > a.halt_after:
                halted = True
                alert("halt", height=last_h, error=str(e)[:200])
        if a.once:
            return
        time.sleep(a.interval)


if __name__ == "__main__":
    sys.exit(main())
