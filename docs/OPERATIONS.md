# Running Nira nodes

> There is no public Nira network yet. This guide covers local networks, and joining a network once a genesis file is published (see [`genesis/README.md`](../genesis/README.md)).

## Node roles

| Role | What it runs | Key config | Hardware (proposed, CHAIN.md R-N5) |
|---|---|---|---|
| **Core Validator** | `nirad start` with a consensus key, behind sentry nodes | JSON-RPC **disabled**; `minimum-gas-prices` set; remote signer recommended | 8+ cores, 32 GB, 2 TB NVMe, 1 Gbps, 99.9 % uptime |
| **Full node** | `nirad start`, no validator key | default pruning (keeps the last 362 880 states) | 4 cores, 8 GB, 500 GB SSD |
| **RPC node** | full node with the APIs enabled | `[api] enable`, `[json-rpc] enable`, `api = "eth,net,web3,txpool"`, behind a reverse proxy with rate limits | 8 cores, 16 GB |
| **Archive node** | full node that keeps everything | `pruning = "nothing"` in app.toml | multi-TB |
| **Community Node** | full node + `nirad tx community run` | default pruning (must keep ≥ `apphash_window + epoch_blocks` versions); a funded key with ≥ `min_bond` | 4 cores, 8 GB, 500 GB SSD, 25 Mbps; may go offline |

## Hardened role profiles

`scripts/node-profile.py <validator|sentry|rpc|archive|community> --home ~/.nira [...]` applies the per-role configuration from [`docs/spec/network.md`](spec/network.md): validator behind sentries, sentries with a private validator ID, API exposure, pruning, snapshots, Prometheus on localhost. It's idempotent and prints every change.

## Files and ports

Home directory: `~/.nira` (override with `--home`).

| File | Holds |
|---|---|
| `config/genesis.json` | the network's genesis. Check its sha256 against the published one |
| `config/config.toml` | CometBFT: p2p, consensus timeouts, peers, state sync |
| `config/app.toml` | app: gas prices, pruning, APIs, JSON-RPC, EVM, snapshots |
| `config/priv_validator_key.json` | **validator consensus key: keep it secret and never run two nodes with it (double sign = 5 % slash + tombstone)** |
| `config/node_key.json` | p2p identity |
| `community/onion.json` | community node onion secret (0600): never share it |

| Port | Service | Expose publicly? |
|---|---|---|
| 26656 | P2P | yes |
| 26657 | CometBFT RPC | RPC nodes only, behind a proxy |
| 1317 | Cosmos REST (`/cosmos/...`, `/nira/...`) | RPC nodes only |
| 9090 | gRPC | RPC nodes only |
| 8545 / 8546 | Ethereum JSON-RPC / WebSocket | RPC nodes only, never on validators |
| 26660 | Prometheus metrics (`prometheus = true` in config.toml) | internal only |

## Local networks

```bash
make localnet && nirad start        # 1 validator, ~/.nira, keys: validator, dev1, dev2 (test keyring)
make localtest                      # 4 validators through the genesis ceremony, ~/.nira-localtest/val{0..3}
make localtest-stop
```

## Docker testnet (multi-node, on one machine)

```bash
make testnet-up VALIDATORS=8 COMMUNITY=6   # builds nira:local from source, runs the genesis ceremony in a container
make testnet-test                          # health, sentry, community, partition, soak (or SCENARIO=…)
python3 deploy/testnet/testnet.py status   # height + peers of every node
make testnet-down                          # removes containers and the chain data volume
```

| Container | Role |
|---|---|
| `val0`…`val{N-2}` | validators on the public mesh |
| `val{N-1}` + `sentry0` | validator behind a sentry (validator profile; its ID is private on the sentry) |
| `rpc0` | RPC node, exposed on the host: CometBFT `127.0.0.1:36657`, REST `31317`, JSON-RPC `38545` / WS `38546` |
| `cn0`…`cn{M-1}` | community nodes: full node + `nirad tx community run` (genesis-registered, restart-safe) |

Scenarios: `partition` cuts nodes off the Docker network (`docker network disconnect`); `soak` runs `tools/loadgen` against `rpc0` with `scripts/monitor.py` watching for halts, slashing, jailing and hook errors, and reports per-container memory.

## Joining a network (once a genesis exists)

```bash
nirad init <moniker> --chain-id <chain-id>
cp genesis.json ~/.nira/config/genesis.json
sha256sum ~/.nira/config/genesis.json        # must equal the published genesis.sha256
# config.toml: set persistent_peers / seeds published by the network
# app.toml:    minimum-gas-prices = "<published floor>anira"   (never leave it at 0, see risk K6)
nirad start
```

**State sync** (join without replaying from genesis): CometBFT state sync is supported by the SDK. It needs nodes serving snapshots (`[state-sync] snapshot-interval` in app.toml, e.g. 1000 on mainnet; the test network uses 20), and in `config.toml [statesync]`: `enable = true`, two or more `rpc_servers`, and a `trust_height` / `trust_hash` taken from a block you trust (obtain it from at least two independent sources). Tested by `make sync-test`.

## Becoming a Core Validator (after launch)

```bash
nirad keys add operator                       # eth_secp256k1; back up the mnemonic offline
nirad comet show-validator > pubkey.json
nirad tx staking create-validator validator.json --from operator --chain-id <chain-id> --gas auto --gas-prices <floor>anira
```

The active set is the top `max_validators` by bonded stake. A validator can't receive delegations that would push it above `max_validator_power_bps` of total stake once the cap is active (ADR-0004). Downtime: jailed after missing more than 95 % of `signed_blocks_window` blocks; unjail with `nirad tx slashing unjail`.

## Running a Community Node

```bash
nirad keys add cnode                                       # fund it with ≥ min_bond + fees
nirad tx community onion-init                              # prints the onion tip; secret stays local
nirad tx community register-node <moniker> 1000000000000000000000anira <tip> 100000 --from cnode --gas auto --gas-prices <floor>anira
nirad tx community run --from cnode --gas auto --gas-prices <floor>anira   # loop; safe to stop/restart
nirad q community node $(nirad keys show cnode -a)          # status, units proven
nirad tx community deregister-node --from cnode            # bond returns after unbonding_blocks
```

- Offline time is never penalised; you simply aren't paid for epochs you miss.
- After more than `max_reveal_distance` epochs offline, run `nirad tx community rotate-onion <new tip> <length>`. Create the new tip with `onion-init --overwrite`. It's valid from the next epoch.
- The same operations are available from Solidity and Ethereum wallets through the precompile at `0x0000000000000000000000000000000000000900` (`precompiles/community/ICommunity.sol`).

## Software upgrades

1. A release that needs a state migration adds one entry to `app.Upgrades` (`app/upgrades_<name>.go`), with the plan name, handler and store changes. **Only the target release carries the entry**: x/upgrade halts a node whose binary already contains the handler of a plan that hasn't been reached yet.
2. Governance passes `MsgSoftwareUpgrade{name, height}`.
3. At `height` every node on the old binary halts with `UPGRADE "<name>" NEEDED at height: <h>`. Nothing is finalised past it.
4. Operators stop the old binary and start the new one (manually, or with Cosmovisor). The handler runs at `<h>` and blocks continue.

Rehearse every release on a test network first: `make upgrade-rehearsal` performs exactly this flow with a `-tags rehearsal` binary, and asserts that the halt height, handler height, state and EVM are preserved.

## Monitoring and alerting

- **Metrics:** set `[instrumentation] prometheus = true` (the role profiles do, bound to 127.0.0.1:26660) and app telemetry in `app.toml [telemetry]`. `deploy/monitoring/` contains a Prometheus scrape config, alert rules (chain halted, block time, missed blocks, byzantine validators, low peers, syncing, mempool near full) and a docker-compose stack (Prometheus, Alertmanager, Grafana). The YAML is syntax-checked, but `promtool` wasn't run.
- **On-chain security monitor:** `scripts/monitor.py --rpc … --rest …` emits one JSON line per alert: halt, slashing, jailing, voting-power shifts or concentration, new governance proposals, upgrade plans, `comm_hook_error`, community slashing, divergence threshold, supply jumps. Pipe it to your alert channel. Verified live on a localnet (proposal, upgrade plan and halt detected).

## Example systemd unit

```ini
# /etc/systemd/system/nirad.service
[Unit]
Description=Nira node
After=network-online.target

[Service]
User=nira
ExecStart=/usr/local/bin/nirad start --home /home/nira/.nira
Restart=on-failure
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

## Security checklist for operators

- Validators: sentry topology, consensus key in a remote signer/HSM (e.g. TMKMS or Horcrux), JSON-RPC off, firewall everything except P2P.
- Never copy `priv_validator_key.json` to a second running node.
- Use `--keyring-backend os` or `file` for real funds; `test` is for development only.
- Verify the genesis sha256 and binary checksums before starting.
