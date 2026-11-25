# Running Nira nodes

> There is no public Nira network yet. This guide covers local networks, and joining a network once a genesis file is published (see [`genesis/README.md`](../genesis/README.md)).

## Node roles

| Role | What it runs | Key config | Hardware (proposed, CHAIN.md R-N5) |
|---|---|---|---|
| **Core Validator** | `nirad start` with a consensus key, behind sentry nodes | JSON-RPC **disabled**; `minimum-gas-prices` set; remote signer recommended | 8+ cores, 32 GB, 2 TB NVMe, 1 Gbps, 99.9 % uptime |
| **Full node** | `nirad start`, no validator key | default pruning (keeps the last 362 880 states) | 4 cores, 8 GB, 500 GB SSD |
| **RPC node** | full node with the APIs enabled | `[api] enable`, `[json-rpc] enable`, behind a reverse proxy with rate limits | 8 cores, 16 GB |
| **Archive node** | full node that keeps everything | `pruning = "nothing"` in app.toml | multi-TB |
| **Community Node** | full node + `nirad tx community run` | default pruning (must keep ≥ `apphash_window + epoch_blocks` versions); a funded key with ≥ `min_bond` | 4 cores, 8 GB, 500 GB SSD, 25 Mbps; may go offline |

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

## Joining a network (once a genesis exists)

```bash
nirad init <moniker> --chain-id <chain-id>
cp genesis.json ~/.nira/config/genesis.json
sha256sum ~/.nira/config/genesis.json        # must equal the published genesis.sha256
# config.toml: set persistent_peers / seeds published by the network
# app.toml:    minimum-gas-prices = "<published floor>anira"   (never leave it at 0, see risk K6)
nirad start
```

**State sync** (join without replaying from genesis): CometBFT state sync is supported by the SDK. It needs nodes serving snapshots (`[state-sync] snapshot-interval` in app.toml) and a trusted height and hash in `config.toml [statesync]`. *Not yet tested on Nira* (see docs/STATUS.md).

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
