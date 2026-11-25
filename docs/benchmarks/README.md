# Performance baseline (G-TEST-4)

- Date: 2026-11-25. Commit: see `git log` for `docs/benchmarks`.
- **Environment:** one machine, WSL2 Ubuntu 24.04, 28 logical cores, 31 GB RAM, NVMe via the Windows mount. **Every validator of a multi-validator run shares this machine**, so there is no real network latency and the CPU is shared. Treat these numbers as a *regression baseline*, not as production capacity.
- **Config:** `timeout_commit = 1s`, block gas limit 100 M, EVM mempool capacity 4 096 txs, min gas price 1–2 gwei, default pruning.
- **Tool:** `tools/loadgen` (`make bench-load`). It funds fresh eth_secp256k1 accounts, then every account submits signed transfers as fast as the node accepts them. Results are computed from **committed blocks** (CometBFT RPC): included txs, TPS over the measured block range, block-time percentiles. Raw JSON: `docs/benchmarks/raw/`.

## Throughput (100 accounts × 50 transfers = 5 000 txs per run)

| Validators | Mode | Included | TPS | Blocks | Max tx/block | Block time p50 / p95 | Notes |
|---:|---|---:|---:|---:|---:|---|---|
| 1 | native (`MsgSend`) | 4 096 / 5 000 | **707** | 5 | 833 | 1.18 s / 1.21 s | 904 rejected at submit: mempool full (4 096 cap) is backpressure, not loss |
| 1 | EVM (value transfer) | 5 000 | **2 037** | 2 | 4 544 | 1.01 s / 1.01 s | bounded by block gas: 100 M / 21 000 ≈ 4 760 transfers per block |
| 1 | mixed 50/50 | 5 000 | **1 030** | 4 | 2 616 | 1.14 s / 1.29 s | |
| 4 | mixed | 5 000 | **673** | 5 | 2 722 | 1.38 s / 1.64 s | idle block time 1.33 s |
| 16 | mixed | 5 000 | **494** | 5 | 1 542 | 2.18 s / 2.21 s | all 16 nodes on one machine; idle block time 1.33 s |

Against the proposed targets (CHAIN.md):
- **R-N1** (block time ≤ 2 s p50, ≤ 4 s p95): met up to 4 validators. At 16 co-located validators under load, p50 is 2.18 s, just above target; it needs re-measuring on separate machines.
- **R-N4** (≥ 300 TPS of simple transfers): met in every configuration measured.

## Community verification cost (Go benchmarks, `go test ./x/community/keeper -bench .`)

| Operation | Time | Gas |
|---|---:|---:|
| Verify one ICS-23 state proof (`VerifyStateProof`) | ~9 µs | — |
| Full `SubmitTaskProof` handler (checks C1–C12 + writes, excl. tx overhead) | ~45 µs | ~36 600 |

A full epoch at the maximum (`max_proofs_per_epoch = 1024`) costs about 46 ms of execution spread over 600 blocks: negligible next to consensus. Community verification can't slow block production.

## Findings

1. **EVM transfers are gas-bound**, not CPU-bound: a block holds about 4 760 simple transfers. Raising throughput means raising `block.max_gas`, a governance parameter with DoS implications (keep it finite).
2. **Native txs hit the 4 096-tx mempool cap** under burst load; excess is rejected at submission (clients must retry). Tune with the EVM mempool config if needed.
3. **Block time grows with co-located validators** (1.0 → 1.33 s idle at 4 and 16 validators). Separate-machine measurements are required before conclusions about vote-gossip overhead.

## Docker testnet soak (2026-11-26)

`make testnet-up VALIDATORS=8 COMMUNITY=6 && make testnet-test`: 16 containers on one host (8 validators, one behind a sentry, 1 sentry, 1 RPC node, 6 community nodes running the prove loop). Raw log: `raw/docker-testnet-2026-11-26.log`.

| Scenario | Result |
|---|---|
| health | 8/8 validators sign; 16 nodes agree on block hashes |
| sentry | sentried validator's only peer is its sentry; it signs every block |
| community | 6 nodes proved 24 tasks, all rewarded (19.54 NIRA) |
| partition | 6\|2: majority continues, minority catches up; 5\|3: halt, then resume with identical hashes on all 16 nodes |
| **soak, 10 min** | **52 074 txs** (mixed native/EVM, 52 load rounds) in 417 blocks (1.45 s mean block time under load); **no halt, slash, jail or hook-error alerts**; no panics or consensus failures; 0 duplicate-tx ERROR lines |

Memory per container (MiB): ~150–250 at start → ~450–585 at the end of the soak → **~400–510 after 2 idle minutes**. It falls when load stops (val0 485 → 415, rpc0 525 → 444), consistent with caches and heap sized by the workload, not a monotonic leak. A 10-minute run can't rule out a slow leak: the 24 h soak below is still required.

## Not yet measured

Network propagation across real hosts, snapshot-sync duration on a large state, community nodes at scale (hundreds of provers), and long-duration (24 h+) soak (`make testnet-test SCENARIO=soak` with `--soak-minutes 1440` on a dedicated host). Each needs dedicated infrastructure (multiple machines, G-OPS-3).

## Reproduce

```bash
make localnet && nirad start                        # terminal 1
make bench-load                                     # terminal 2: native, evm, mixed on 1 validator
N=4 scripts/genesis/localtest.sh && make bench-load BENCH_CHAIN_ID=nira_47201-77 BENCH_HOME=~/.nira-localtest/val0
go test ./x/community/keeper -run '^$' -bench 'Verify|Submit'
```
