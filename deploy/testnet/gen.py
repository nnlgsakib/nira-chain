#!/usr/bin/env python3
"""Generate a Docker testnet inside the nira image (writes /data).

    python3 /opt/nira/gen.py --validators 8 --community 6 --chain-id nira_93000-88

Nodes (container hostname = directory under /data):
  val0..valN-2   validators on the public mesh
  valN-1         validator behind sentry0 (validator profile: PEX off, private ID)
  sentry0        sentry for valN-1 (sentry profile, private_peer_ids)
  rpc0           RPC node (APIs + JSON-RPC exposed to the host)
  cn0..cnM-1     community nodes (community profile, genesis-registered, run the prove loop)

Runs the real ceremony: keys → network file → pregenesis → gentx per validator →
finalize (build_genesis.py verifies every gentx) → identical genesis in every home.
"""
import argparse
import json
import re
import shutil
import subprocess
from pathlib import Path

D = Path("/data")
NIRA = 10**18
DEV1_MNEMONIC = ("copper push brief egg scan entry inform record adjust fossil boss egg comic alien "
                 "upon aspect dry avoid interest fury window hint race symptom")


def sh(*args, input=None):
    r = subprocess.run(args, capture_output=True, text=True, input=input)
    if r.returncode:
        raise SystemExit(f"{' '.join(args)}\n{r.stdout}\n{r.stderr}")
    return r.stdout


def kr(home):
    return ["--keyring-backend", "test", "--home", str(home)]


def addr(name, home):
    return sh("nirad", "keys", "show", name, "-a", *kr(home)).strip()


def node_id(home):
    return sh("nirad", "comet", "show-node-id", "--home", str(home)).strip()


def set_toml(path, section, key, value):
    text, cur, out, done = Path(path).read_text().split("\n"), None, [], False
    for line in text:
        m = re.match(r"^\[([^\]]+)\]\s*$", line)
        if m:
            cur = m.group(1)
        elif cur == section and re.match(rf"^{re.escape(key)}\s*=", line) and not done:
            line, done = f"{key} = {value}", True
        out.append(line)
    if not done:
        raise SystemExit(f"{path}: [{section}] {key} not found")
    Path(path).write_text("\n".join(out))


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--validators", type=int, default=4)
    ap.add_argument("--community", type=int, default=4)
    ap.add_argument("--chain-id", default="nira_93000-88")
    a = ap.parse_args()
    if a.validators < 4:
        raise SystemExit("need ≥ 4 validators")
    for p in D.iterdir():
        shutil.rmtree(p) if p.is_dir() else p.unlink()

    vals = [f"val{i}" for i in range(a.validators)]
    sentried = vals[-1]
    cns = [f"cn{i}" for i in range(a.community)]
    nodes = vals + ["sentry0", "rpc0"] + cns
    for n in nodes:
        sh("nirad", "init", n, "--chain-id", a.chain_id, "--default-denom", "anira", "--home", str(D / n))
    for v in vals:
        sh("nirad", "keys", "add", "val", "--algo", "eth_secp256k1", *kr(D / v))
    sh("nirad", "keys", "add", "dev1", "--recover", "--algo", "eth_secp256k1", *kr(D / "rpc0"), input=DEV1_MNEMONIC + "\n")
    tips = {}
    for c in cns:
        sh("nirad", "keys", "add", "cnode", "--algo", "eth_secp256k1", *kr(D / c))
        out = sh("nirad", "tx", "community", "onion-init", "--length", "100000", "--home", str(D / c))
        tips[c] = re.search(r"onion_tip: ([0-9a-f]{64})", out).group(1)

    # network file (the same format used for mainnet, genesis/networks/*.toml)
    total = 100_000_000 + 50_000_000 + a.validators * 1_010_000 + a.community * 1_100
    t = [f'[chain]\nchain_id = "{a.chain_id}"\ndenom = "anira"\n[rules]\nmin_validators = 4\nmax_genesis_power_share = "0.3333"',
         f'[supply]\ntotal = "{total}"',
         '[[allocation]]\nname = "Protocol reserve"\nkind = "community_pool"\namount = "50000000"',
         f'[[allocation]]\nname = "dev1 (load generator funder)"\naddress = "{addr("dev1", D / "rpc0")}"\namount = "100000000"']
    for v in vals:
        t.append(f'[[validator]]\nmoniker = "{v}"\naddress = "{addr("val", D / v)}"\nbalance = "1010000"\nself_bond = "1000000"')
    for c in cns:
        t.append(f'[[community_node]]\nmoniker = "{c}"\noperator = "{addr("cnode", D / c)}"\nbond = "1000"\n'
                 f'balance = "100"\nonion_tip = "{tips[c]}"\nonion_length = 100000')
    t.append('[params.feemarket]\nmin_gas_price = "1000000000.000000000000000000"')
    t.append('[params.community]\nepoch_blocks = "20"\napphash_window = 10\ntarget_committee = 1000\n'
             'max_proofs_per_epoch = 2000\nunbonding_blocks = "15"\nalert_ttl_blocks = "200"')
    t.append('[params.slashing]\nsigned_blocks_window = "100"\ndowntime_jail_duration = "60s"')
    t.append('[params.gov]\nvoting_period = "30s"\nexpedited_voting_period = "15s"')
    build = D / "build"
    build.mkdir()
    (build / "network.toml").write_text("\n".join(t) + "\n")

    bg = ["python3", "/opt/nira/build_genesis.py"]
    sh(*bg, "pregenesis", str(build / "network.toml"), "--out", str(build), "--binary", "nirad")
    (build / "gentx").mkdir()
    for v in vals:
        shutil.copy(build / "pregenesis.json", D / v / "config" / "genesis.json")
        sh("nirad", "genesis", "gentx", "val", f"{1_000_000 * NIRA}anira", "--chain-id", a.chain_id, *kr(D / v),
           "--moniker", v, "--min-self-delegation", str(100_000 * NIRA), "--commission-rate", "0.10",
           "--commission-max-rate", "0.20", "--commission-max-change-rate", "0.01",
           "--output-document", str(build / "gentx" / f"{v}.json"))
    sh(*bg, "finalize", str(build / "network.toml"), "--out", str(build), "--gentx-dir", str(build / "gentx"), "--binary", "nirad")

    ids = {n: node_id(D / n) for n in nodes}
    peer = lambda n: f"{ids[n]}@{n}:26656"
    public_vals = [v for v in vals if v != sentried]
    for n in nodes:
        home = D / n
        shutil.copy(build / "genesis.json", home / "config" / "genesis.json")
        cfg, app = home / "config" / "config.toml", home / "config" / "app.toml"
        prof = ["python3", "/opt/nira/node-profile.py"]
        if n == sentried:
            sh(*prof, "validator", "--home", str(home), "--sentries", peer("sentry0"))
        elif n == "sentry0":
            sh(*prof, "sentry", "--home", str(home), "--private-ids", ids[sentried], "--validator-peer", peer(sentried))
        elif n == "rpc0":
            sh(*prof, "rpc", "--home", str(home))
        elif n.startswith("cn"):
            sh(*prof, "community", "--home", str(home))
        # peers: everyone except the sentried validator dials the public validators + sentry
        if n != sentried:
            peers = [peer(v) for v in public_vals + ["sentry0"] if v != n]
            if n == "sentry0":
                peers.append(peer(sentried))
            set_toml(cfg, "p2p", "persistent_peers", f'"{",".join(peers)}"')
        set_toml(cfg, "p2p", "addr_book_strict", "false")  # docker private addresses
        set_toml(cfg, "consensus", "timeout_commit", '"1s"')
        set_toml(cfg, "rpc", "laddr", '"tcp://0.0.0.0:26657"')
        set_toml(cfg, "instrumentation", "prometheus", "true")
        set_toml(cfg, "instrumentation", "prometheus_listen_addr", '"0.0.0.0:26660"')
        set_toml(app, None, "minimum-gas-prices", '"1000000000anira"')
        if n in ("rpc0", "val0"):
            set_toml(app, "api", "enable", "true")
            set_toml(app, "api", "address", '"tcp://0.0.0.0:1317"')
            set_toml(app, "json-rpc", "enable", "true")
            set_toml(app, "json-rpc", "address", '"0.0.0.0:8545"')
            set_toml(app, "json-rpc", "ws-address", '"0.0.0.0:8546"')
            set_toml(app, "json-rpc", "api", '"eth,net,web3,txpool"')
        if n.startswith("cn"):  # prove loop talks to its own node
            set_toml(app, "api", "enable", "true")
            set_toml(app, "grpc", "enable", "true")
    json.dump({"validators": vals, "sentried": sentried, "community": cns, "ids": ids, "chain_id": a.chain_id},
              open(D / "testnet.json", "w"), indent=1)
    print(json.dumps({"validators": len(vals), "community": len(cns), "chain_id": a.chain_id}))


if __name__ == "__main__":
    main()
