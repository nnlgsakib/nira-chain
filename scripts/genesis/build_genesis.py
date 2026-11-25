#!/usr/bin/env python3
"""Nira genesis builder — one reviewable file in, a reproducible genesis out.

Every economic choice and every initial address lives in a network file
(genesis/networks/<network>.toml): total supply, allocation buckets (accounts,
vesting, community pool), initial Core Validators, initial Community Nodes and
all module parameters. Change the file, rebuild, and review the report.

Commands
  check      validate the network file and print the allocation report
  pregenesis build the genesis WITHOUT gentxs (published to validators so they
             can run `nira-chaind genesis gentx` against it)
  finalize   rebuild the pregenesis deterministically, verify and collect the
             validators' gentxs, validate, write genesis.json + sha256 + report

Amounts in the network file are whole/decimal NIRA strings ("1000000",
"0.5"); parameter values may use the suffix " NIRA" to be converted to base
units (anira, 18 decimals). Everything else is passed through verbatim.
"""
from __future__ import annotations

import argparse
import base64
import copy
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import tomllib
from decimal import Decimal, InvalidOperation
from pathlib import Path

DECIMALS = 18
BASE = 10**DECIMALS
HRP = "nira"
EVM_CHAIN_ID = 47201  # protocol constant, app/denom.go
PRECOMPILES = [
    "0x0000000000000000000000000000000000000100",  # P256
    "0x0000000000000000000000000000000000000400",  # bech32
    "0x0000000000000000000000000000000000000800",  # staking
    "0x0000000000000000000000000000000000000801",  # distribution
    "0x0000000000000000000000000000000000000804",  # bank
    "0x0000000000000000000000000000000000000805",  # gov
    "0x0000000000000000000000000000000000000806",  # slashing
    "0x0000000000000000000000000000000000000900",  # x/community (ADR-0006)
]

# ---------------------------------------------------------------- bech32 ---
_CHARSET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"


def _polymod(values):
    gen = [0x3B6A57B2, 0x26508E6D, 0x1EA119FA, 0x3D4233DD, 0x2A1462B3]
    chk = 1
    for v in values:
        b = chk >> 25
        chk = (chk & 0x1FFFFFF) << 5 ^ v
        for i in range(5):
            chk ^= gen[i] if ((b >> i) & 1) else 0
    return chk


def _hrp_expand(hrp):
    return [ord(x) >> 5 for x in hrp] + [0] + [ord(x) & 31 for x in hrp]


def _convertbits(data, frombits, tobits, pad=True):
    acc, bits, ret, maxv = 0, 0, [], (1 << tobits) - 1
    for value in data:
        acc = (acc << frombits) | value
        bits += frombits
        while bits >= tobits:
            bits -= tobits
            ret.append((acc >> bits) & maxv)
    if pad and bits:
        ret.append((acc << (tobits - bits)) & maxv)
    elif not pad and (bits >= frombits or ((acc << (tobits - bits)) & maxv)):
        raise ValueError("invalid bech32 padding")
    return ret


def bech32_encode(hrp: str, data: bytes) -> str:
    d = _convertbits(data, 8, 5)
    poly = _polymod(_hrp_expand(hrp) + d + [0] * 6) ^ 1
    return hrp + "1" + "".join(_CHARSET[x] for x in d + [(poly >> 5 * (5 - i)) & 31 for i in range(6)])


def bech32_decode(s: str) -> tuple[str, bytes]:
    s = s.lower()
    pos = s.rfind("1")
    hrp, data = s[:pos], [_CHARSET.find(c) for c in s[pos + 1:]]
    if pos < 1 or -1 in data or len(data) < 6 or _polymod(_hrp_expand(hrp) + data) != 1:
        raise ValueError(f"invalid bech32 address {s}")
    return hrp, bytes(_convertbits(data[:-6], 5, 8, pad=False))


def addr_bytes(a: str) -> bytes:
    """Accept bech32 (nira1…/niravaloper1…) or 0x hex; same 20 bytes either way."""
    a = a.strip()
    if a.lower().startswith("0x"):
        b = bytes.fromhex(a[2:])
    else:
        hrp, b = bech32_decode(a)
        if hrp not in (HRP, HRP + "valoper"):
            raise ValueError(f"{a}: unexpected prefix {hrp}")
    if len(b) != 20:
        raise ValueError(f"{a}: address must be 20 bytes")
    return b


def acc(b: bytes) -> str:
    return bech32_encode(HRP, b)


def module_address(name: str) -> bytes:
    return hashlib.sha256(name.encode()).digest()[:20]


# ---------------------------------------------------------------- amounts ---
def nira_to_base(v) -> int:
    try:
        d = Decimal(str(v).replace("_", "").strip())
    except InvalidOperation as e:
        raise ValueError(f"invalid NIRA amount {v!r}") from e
    base = d * BASE
    if base != base.to_integral_value() or base < 0:
        raise ValueError(f"NIRA amount {v!r} has more than {DECIMALS} decimals or is negative")
    return int(base)


def fmt(base: int) -> str:
    whole, frac = divmod(base, BASE)
    s = f"{whole:,}"
    return s + (("." + f"{frac:018d}".rstrip("0")) if frac else "")


def convert_params(v):
    """Recursively convert 'X NIRA' strings to base-unit strings."""
    if isinstance(v, dict):
        return {k: convert_params(x) for k, x in v.items()}
    if isinstance(v, list):
        return [convert_params(x) for x in v]
    if isinstance(v, str) and v.endswith(" NIRA"):
        return str(nira_to_base(v[:-5]))
    return v


def deep_merge(dst: dict, src: dict, path=""):
    for k, v in src.items():
        if isinstance(v, dict) and isinstance(dst.get(k), dict):
            deep_merge(dst[k], v, f"{path}.{k}")
        else:
            if k not in dst:
                raise KeyError(f"unknown genesis field {path}.{k}".lstrip("."))
            dst[k] = v


def unix(ts: str) -> int:
    return int(dt.datetime.fromisoformat(ts.replace("Z", "+00:00")).timestamp())


# ----------------------------------------------------------------- config ---
class Network:
    def __init__(self, path: Path):
        self.path = path
        self.raw = tomllib.loads(path.read_text())
        c = self.raw
        self.chain = c.get("chain", {})
        self.chain_id = self.chain.get("chain_id", "")
        self.denom = self.chain.get("denom", "anira")
        self.genesis_time = self.chain.get("genesis_time")
        self.total = nira_to_base(c.get("supply", {}).get("total", "0"))
        self.allocations = c.get("allocation", [])
        self.validators = c.get("validator", [])
        self.nodes = c.get("community_node", [])
        self.params = convert_params(c.get("params", {}))
        self.consensus = convert_params(c.get("consensus", {}))
        # Raw app_state overrides for fields outside module params (e.g. mint.minter)
        self.app_state = convert_params(c.get("app_state", {}))
        self.rules = c.get("rules", {})

    # Every NIRA is accounted for exactly once.
    def ledger(self):
        rows = []
        for a in self.allocations:
            rows.append((a["name"], a.get("kind", "account"), nira_to_base(a["amount"]), a))
        for v in self.validators:
            rows.append((f"validator {v['moniker']}", "validator", nira_to_base(v["balance"]), v))
        for n in self.nodes:
            rows.append((f"community node {n['moniker']} (bond)", "community_bond", nira_to_base(n["bond"]), n))
            if nira_to_base(n.get("balance", "0")):
                rows.append((f"community node {n['moniker']} (liquid)", "account", nira_to_base(n["balance"]), n))
        return rows

    def validate(self) -> tuple[list[str], list[str]]:
        errors, warns = [], []
        e = errors.append
        if not re.fullmatch(rf"nira_{EVM_CHAIN_ID}-\d+", self.chain_id):
            e(f"chain.chain_id must look like nira_{EVM_CHAIN_ID}-<n> (EVM chain ID {EVM_CHAIN_ID} is a protocol constant), got {self.chain_id!r}")
        if self.denom != "anira":
            e("chain.denom must be 'anira' (compiled into the binary)")
        if self.genesis_time:
            try:
                unix(self.genesis_time)
            except ValueError:
                e(f"chain.genesis_time is not RFC3339: {self.genesis_time}")
        if self.total <= 0:
            e("supply.total must be > 0")

        try:
            rows = self.ledger()
        except (KeyError, ValueError) as x:
            return [f"malformed entry: {x}"], warns
        s = sum(r[2] for r in rows)
        if s != self.total:
            e(f"allocations sum to {fmt(s)} NIRA but supply.total is {fmt(self.total)} NIRA (diff {fmt(abs(s - self.total))})")

        seen = {}

        def claim(addr: str, who: str):
            try:
                b = addr_bytes(addr)
            except ValueError as x:
                e(f"{who}: {x}")
                return None
            if b in seen:
                e(f"{who}: address {acc(b)} already used by {seen[b]}")
            seen[b] = who
            return b

        cp_count = 0
        for a in self.allocations:
            kind = a.get("kind", "account")
            who = f"allocation '{a['name']}'"
            if kind == "community_pool":
                cp_count += 1
                if "address" in a:
                    e(f"{who}: community_pool allocations have no address")
                continue
            if kind != "account":
                e(f"{who}: unknown kind {kind!r} (account | community_pool)")
                continue
            if "address" not in a:
                e(f"{who}: address required")
                continue
            claim(a["address"], who)
            vest = a.get("vesting")
            if vest:
                try:
                    amt = nira_to_base(vest.get("amount", a["amount"]))
                    if amt > nira_to_base(a["amount"]):
                        e(f"{who}: vesting amount exceeds allocation")
                    end = unix(vest["end"])
                    start = unix(vest["start"]) if "start" in vest else None
                    if start is not None and start >= end:
                        e(f"{who}: vesting start must be before end")
                except (KeyError, ValueError) as x:
                    e(f"{who}: invalid vesting {x}")
        if cp_count > 1:
            e("at most one community_pool allocation")

        min_vals = int(self.rules.get("min_validators", 4))
        if len(self.validators) < min_vals:
            e(f"{len(self.validators)} validators < rules.min_validators ({min_vals})")
        bonds = []
        for v in self.validators:
            who = f"validator '{v.get('moniker')}'"
            claim(v.get("address", ""), who)
            try:
                b, bal = nira_to_base(v["self_bond"]), nira_to_base(v["balance"])
            except (KeyError, ValueError) as x:
                e(f"{who}: {x}")
                continue
            if b < BASE:
                e(f"{who}: self_bond must be ≥ 1 NIRA")
            if b > bal:
                e(f"{who}: self_bond {fmt(b)} > balance {fmt(bal)}")
            bonds.append((v.get("moniker"), b))
        total_bonded = sum(b for _, b in bonds)
        max_share = Decimal(self.rules.get("max_genesis_power_share", "0.3333"))
        for m, b in bonds:
            share = Decimal(b) / Decimal(total_bonded) if total_bonded else Decimal(0)
            if len(bonds) > 1 and share >= max_share:
                e(f"validator '{m}' holds {share:.2%} of genesis voting power (≥ {max_share:.2%}): one operator could halt or fork the chain")
        cap_bps = int(self.params.get("policy", {}).get("max_validator_power_bps", 1000))
        for m, b in bonds:
            if total_bonded and Decimal(b) / Decimal(total_bonded) * 10000 > cap_bps:
                warns.append(f"validator '{m}' starts above the {cap_bps} bps cap: it cannot receive new delegations once the cap activates")
        maxv = int(self.params.get("staking", {}).get("max_validators", 100))
        if len(self.validators) > maxv:
            e(f"{len(self.validators)} validators > staking.max_validators {maxv}")

        cparams = self.params.get("community", {})
        min_bond = int(cparams.get("min_bond", str(1000 * BASE)))
        max_len = int(cparams.get("max_onion_length", 1_000_000))
        for n in self.nodes:
            who = f"community node '{n.get('moniker')}'"
            claim(n.get("operator", ""), who)
            try:
                if nira_to_base(n["bond"]) < min_bond:
                    e(f"{who}: bond below community.min_bond {fmt(min_bond)} NIRA")
                tip = bytes.fromhex(n["onion_tip"])
                if len(tip) != 32 or tip == bytes(32):
                    e(f"{who}: onion_tip must be 32 non-zero bytes (hex)")
                if not 1 <= int(n["onion_length"]) <= max_len:
                    e(f"{who}: onion_length must be in [1, {max_len}]")
                if len(n.get("moniker", "")) > 64:
                    e(f"{who}: moniker longer than 64 bytes")
            except (KeyError, ValueError) as x:
                e(f"{who}: {x}")
        return errors, warns

    def report(self, genesis_sha: str | None = None, warns=()) -> str:
        rows = self.ledger()
        L = [f"# Genesis report — {self.chain_id}", "",
             f"- Network file: `{self.path}`",
             f"- Total supply: **{fmt(self.total)} NIRA** ({self.total} anira)",
             f"- Genesis time: {self.genesis_time or '(set at build)'}"]
        if genesis_sha:
            L.append(f"- genesis.json sha256: `{genesis_sha}`")
        L += ["", "## Allocations", "", "| Bucket | Kind | NIRA | % of supply | Vesting |", "|---|---|---:|---:|---|"]
        for name, kind, amt, entry in rows:
            v = entry.get("vesting") if kind == "account" else None
            vs = f"{v.get('amount', entry.get('amount'))} NIRA, {v.get('start', 'delayed')} → {v['end']}" if v else ""
            L.append(f"| {name} | {kind} | {fmt(amt)} | {Decimal(amt) / Decimal(self.total):.4%} | {vs} |")
        by_kind = {}
        for _, kind, amt, _ in rows:
            by_kind[kind] = by_kind.get(kind, 0) + amt
        L += ["", "| Kind | NIRA | % |", "|---|---:|---:|"]
        for k, amt in sorted(by_kind.items()):
            L.append(f"| {k} | {fmt(amt)} | {Decimal(amt) / Decimal(self.total):.4%} |")
        bonds = [(v["moniker"], v["address"], nira_to_base(v["self_bond"])) for v in self.validators]
        tb = sum(b for *_, b in bonds) or 1
        L += ["", f"## Core Validators ({len(bonds)})", "", "| Moniker | Operator | Self-bond NIRA | Genesis voting power |", "|---|---|---:|---:|"]
        for m, a, b in bonds:
            L.append(f"| {m} | `{acc(addr_bytes(a))}` | {fmt(b)} | {Decimal(b) / Decimal(tb):.2%} |")
        if bonds:
            # smallest number of validators controlling > 1/3 (halt) and > 2/3 (rewrite)
            sorted_b = sorted((b for *_, b in bonds), reverse=True)
            def needed(frac):
                s = 0
                for i, b in enumerate(sorted_b, 1):
                    s += b
                    if Decimal(s) / Decimal(tb) > frac:
                        return i
                return len(sorted_b)
            L.append(f"\nNakamoto coefficient: {needed(Decimal(1) / 3)} validators to halt (>1/3), {needed(Decimal(2) / 3)} to finalise alone (>2/3).")
        L += ["", f"## Community Nodes ({len(self.nodes)})", "", "| Moniker | Operator | Bond NIRA | Onion length |", "|---|---|---:|---:|"]
        for n in self.nodes:
            L.append(f"| {n['moniker']} | `{acc(addr_bytes(n['operator']))}` | {fmt(nira_to_base(n['bond']))} | {n['onion_length']} |")
        L += ["", "## Parameter overrides", "", "```json", json.dumps(self.params, indent=1), "```"]
        if self.consensus:
            L += ["", "Consensus:", "```json", json.dumps(self.consensus, indent=1), "```"]
        if warns:
            L += ["", "## Warnings", ""] + [f"- {w}" for w in warns]
        return "\n".join(L) + "\n"


# ------------------------------------------------------------------ build ---
def run(cmd: list[str], **kw) -> str:
    r = subprocess.run(cmd, capture_output=True, text=True, **kw)
    if r.returncode != 0:
        raise SystemExit(f"command failed: {' '.join(cmd)}\n{r.stdout}\n{r.stderr}")
    return r.stdout


def build_pregenesis(net: Network, binary: str, home: Path) -> dict:
    shutil.rmtree(home, ignore_errors=True)
    run([binary, "init", "genesis-coordinator", "--chain-id", net.chain_id,
         "--default-denom", net.denom, "--home", str(home)])
    gpath = home / "config" / "genesis.json"

    def add_account(addr: bytes, amount: int, vest: dict | None = None):
        cmd = [binary, "genesis", "add-genesis-account", acc(addr), f"{amount}{net.denom}", "--home", str(home)]
        if vest:
            vamt = nira_to_base(vest.get("amount", 0)) or amount
            cmd += ["--vesting-amount", f"{vamt}{net.denom}", "--vesting-end-time", str(unix(vest["end"]))]
            if "start" in vest:
                cmd += ["--vesting-start-time", str(unix(vest["start"]))]
        run(cmd)

    community_pool = 0
    for a in net.allocations:
        if a.get("kind", "account") == "community_pool":
            community_pool += nira_to_base(a["amount"])
        else:
            add_account(addr_bytes(a["address"]), nira_to_base(a["amount"]), a.get("vesting"))
    for v in net.validators:
        add_account(addr_bytes(v["address"]), nira_to_base(v["balance"]))
    for n in net.nodes:
        if nira_to_base(n.get("balance", "0")):
            add_account(addr_bytes(n["operator"]), nira_to_base(n["balance"]))

    g = json.loads(gpath.read_text())
    s = g["app_state"]
    if net.genesis_time:
        g["genesis_time"] = net.genesis_time

    # Protocol essentials (not economic choices)
    s["bank"]["denom_metadata"] = [{
        "description": "Native staking, fee and governance token of Nira Chain",
        "base": net.denom, "display": "nira", "name": "Nira", "symbol": "NIRA",
        "denom_units": [{"denom": net.denom, "exponent": 0, "aliases": ["attonira"]},
                        {"denom": "nira", "exponent": DECIMALS, "aliases": []}]}]
    s["staking"]["params"]["bond_denom"] = net.denom
    s["mint"]["params"]["mint_denom"] = net.denom
    s["evm"]["params"]["evm_denom"] = net.denom
    s["evm"]["params"]["active_static_precompiles"] = PRECOMPILES
    s["community"]["params"]["bond_denom"] = net.denom
    g["consensus"]["params"]["block"]["max_gas"] = "100000000"

    # Economic / policy parameters from the network file
    for module, p in net.params.items():
        if module not in s:
            raise SystemExit(f"[params.{module}]: no such module in genesis")
        target = s[module]["params"] if "params" in s[module] else s[module]
        try:
            deep_merge(target, copy.deepcopy(p), f"params.{module}")
        except KeyError as x:
            raise SystemExit(f"{x}")
    if net.consensus:
        deep_merge(g["consensus"]["params"], net.consensus, "consensus")
    if net.app_state:
        try:
            deep_merge(s, copy.deepcopy(net.app_state), "app_state")
        except KeyError as x:
            raise SystemExit(f"{x}")

    # Initial Community Nodes: active from epoch 1, bonds escrowed in x/community
    bonds = 0
    nodes = []
    for n in net.nodes:
        b = nira_to_base(n["bond"])
        bonds += b
        nodes.append({
            "operator": acc(addr_bytes(n["operator"])), "moniker": n["moniker"], "bond": str(b),
            "status": "NODE_STATUS_ACTIVE", "registered_epoch": "0", "onion_base_epoch": "1",
            "onion_anchor": base64.b64encode(bytes.fromhex(n["onion_tip"])).decode(),
            "onion_anchor_epoch": "0", "onion_length": int(n["onion_length"]), "units_total": "0",
            "unbond_complete_height": "0"})
    s["community"]["nodes"] = nodes
    s["community"].setdefault("next_alert_id", "1")

    balances = s["bank"]["balances"]

    def module_balance(name: str, amount: int):
        if amount:
            balances.append({"address": acc(module_address(name)),
                             "coins": [{"denom": net.denom, "amount": str(amount)}]})

    module_balance("community", bonds)
    # Protocol reserve → distribution community pool (governance-controlled)
    module_balance("distribution", community_pool)
    s["distribution"]["fee_pool"]["community_pool"] = (
        [{"denom": net.denom, "amount": f"{community_pool}.000000000000000000"}] if community_pool else [])

    balances.sort(key=lambda b: b["address"])
    total = sum(int(c["amount"]) for b in balances for c in b["coins"] if c["denom"] == net.denom)
    if total != net.total:
        raise SystemExit(f"internal: genesis balances {total} != supply {net.total}")
    s["bank"]["supply"] = [{"denom": net.denom, "amount": str(total)}]
    gpath.write_text(json.dumps(g, indent=1))
    return g


def check_gentxs(net: Network, gentx_dir: Path) -> list[Path]:
    files = sorted(gentx_dir.glob("*.json"))
    want = {addr_bytes(v["address"]): v for v in net.validators}
    got = {}
    for f in files:
        tx = json.loads(f.read_text())
        msgs = tx["body"]["messages"]
        if len(msgs) != 1 or not msgs[0]["@type"].endswith("MsgCreateValidator"):
            raise SystemExit(f"{f}: gentx must contain exactly one MsgCreateValidator")
        m = msgs[0]
        b = addr_bytes(m["validator_address"])
        v = want.get(b)
        if v is None:
            raise SystemExit(f"{f}: validator {m['validator_address']} is not in the network file")
        if b in got:
            raise SystemExit(f"{f}: second gentx for {v['moniker']}")
        if m["value"]["denom"] != net.denom or int(m["value"]["amount"]) != nira_to_base(v["self_bond"]):
            raise SystemExit(f"{f}: self-bond {m['value']} != configured {v['self_bond']} NIRA for {v['moniker']}")
        got[b] = f
    missing = [v["moniker"] for b, v in want.items() if b not in got]
    if missing:
        raise SystemExit(f"missing gentxs for: {', '.join(missing)}")
    return files


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("command", choices=["check", "pregenesis", "finalize"])
    ap.add_argument("network", type=Path, help="genesis/networks/<network>.toml")
    ap.add_argument("--out", type=Path, default=Path("build/genesis"), help="output directory")
    ap.add_argument("--gentx-dir", type=Path, help="directory of validators' gentx JSON files (finalize)")
    ap.add_argument("--binary", default=shutil.which("nira-chaind") or "nira-chaind")
    a = ap.parse_args()

    net = Network(a.network)
    errors, warns = net.validate()
    for w in warns:
        print(f"WARN  {w}", file=sys.stderr)
    if errors:
        for x in errors:
            print(f"ERROR {x}", file=sys.stderr)
        raise SystemExit(f"{len(errors)} error(s) in {a.network}")

    a.out.mkdir(parents=True, exist_ok=True)
    if a.command == "check":
        rep = net.report(warns=warns)
        (a.out / "report.md").write_text(rep)
        print(rep)
        return

    with tempfile.TemporaryDirectory() as tmp:
        home = Path(tmp) / "home"
        build_pregenesis(net, a.binary, home)
        gpath = home / "config" / "genesis.json"
        if a.command == "pregenesis":
            run([a.binary, "genesis", "validate", "--home", str(home)])
            shutil.copy(gpath, a.out / "pregenesis.json")
            (a.out / "report.md").write_text(net.report(warns=warns))
            print(f"wrote {a.out / 'pregenesis.json'} — validators run: nira-chaind genesis gentx <key> <self_bond>anira "
                  f"--chain-id {net.chain_id} (with this file as their config/genesis.json)")
            return
        if not a.gentx_dir:
            raise SystemExit("finalize needs --gentx-dir")
        files = check_gentxs(net, a.gentx_dir)
        gdir = home / "config" / "gentx"
        gdir.mkdir(exist_ok=True)
        for f in files:
            shutil.copy(f, gdir / f.name)
        run([a.binary, "genesis", "collect-gentxs", "--home", str(home)])
        run([a.binary, "genesis", "validate", "--home", str(home)])
        out = a.out / "genesis.json"
        shutil.copy(gpath, out)
        sha = hashlib.sha256(out.read_bytes()).hexdigest()
        (a.out / "genesis.sha256").write_text(f"{sha}  genesis.json\n")
        (a.out / "report.md").write_text(net.report(genesis_sha=sha, warns=warns))
        print(f"wrote {out}\nsha256 {sha}\nreport {a.out / 'report.md'}")


if __name__ == "__main__":
    main()
