#!/usr/bin/env python3
"""Apply Nira genesis parameters (single source of truth for dev/test networks).

Values are (proposed) — see docs/spec/parameters.md.
"""
import json
import sys

DENOM = "anira"
p = sys.argv[1]
g = json.load(open(p))
a = g["app_state"]

g["consensus"]["params"]["block"]["max_gas"] = "100000000"

a["bank"]["denom_metadata"] = [{
    "description": "Native staking, fee and governance token of Nira Chain",
    "base": DENOM, "display": "nira", "name": "Nira", "symbol": "NIRA",
    "denom_units": [
        {"denom": DENOM, "exponent": 0, "aliases": ["attonira"]},
        {"denom": "nira", "exponent": 18, "aliases": []},
    ],
}]

a["staking"]["params"].update(
    bond_denom=DENOM, max_validators=100, unbonding_time="1814400s",
    min_commission_rate="0.050000000000000000")

a["slashing"]["params"].update(
    signed_blocks_window="10000", min_signed_per_window="0.050000000000000000",
    downtime_jail_duration="600s",
    slash_fraction_double_sign="0.050000000000000000",
    slash_fraction_downtime="0.000100000000000000")

a["mint"]["params"]["mint_denom"] = DENOM

gp = a["gov"]["params"]
gp["min_deposit"] = [{"denom": DENOM, "amount": "10000000000000000000000"}]
gp["expedited_min_deposit"] = [{"denom": DENOM, "amount": "50000000000000000000000"}]
gp["min_initial_deposit_ratio"] = "0.250000000000000000"
gp["voting_period"] = "20s"            # localnet only: fast governance e2e
gp["expedited_voting_period"] = "10s"  # G-GOV-1: no zero-deposit proposal spam

a["evm"]["params"]["evm_denom"] = DENOM
a["evm"]["params"]["active_static_precompiles"] = [
    "0x0000000000000000000000000000000000000100",  # P256
    "0x0000000000000000000000000000000000000400",  # bech32
    "0x0000000000000000000000000000000000000800",  # staking
    "0x0000000000000000000000000000000000000801",  # distribution
    "0x0000000000000000000000000000000000000804",  # bank
    "0x0000000000000000000000000000000000000805",  # gov
    "0x0000000000000000000000000000000000000806",  # slashing
    "0x0000000000000000000000000000000000000900",  # x/community (ADR-0006)
]

# x/community: short epochs for local testing (production defaults: spec §7)
cp = a["community"]["params"]
cp.update(bond_denom=DENOM, epoch_blocks="20", apphash_window=10, target_committee=1000,
          max_proofs_per_epoch=2000, unbonding_blocks="15", alert_ttl_blocks="200")

json.dump(g, open(p, "w"), indent=1)
