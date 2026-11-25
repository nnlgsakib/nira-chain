# Key inventory

Nira has **no protocol admin key**. Every module authority is the x/gov module account, and the circuit breaker has no authorised accounts (`account_permissions: []`). The keys below belong to participants.

| Key | Holder | Type | Can do | Custody (required for mainnet) | Rotation / revocation | Compromise detection |
|---|---|---|---|---|---|---|
| Validator consensus key (`priv_validator_key.json`) | each Core Validator | ed25519 | sign blocks and votes with the validator's power | **remote signer** (TMKMS / Horcrux with HSM or YubiHSM); never two active signers | create a new validator (SDK 0.53); consensus-key rotation arrives with SDK 0.55 | equivocation evidence, missed blocks, `tombstoned` flag |
| Validator operator key | each Core Validator | eth_secp256k1 | edit validator, withdraw commission, unbond self-bond, unjail | hardware wallet; offline signing | move self-bond to a new validator | commission/description changes, unexpected unbonding |
| Node key (`node_key.json`) | every node | ed25519 | P2P identity only | file on the node | regenerate freely | — |
| Community node operator key | each community node | eth_secp256k1 | register, prove, deregister, report alerts; holds the bond | hot key on the node (needed every epoch); fund it with the bond plus a small fee buffer only | deregister and re-register with a new key | unexpected deregistration or alerts |
| Community onion secret (`community/onion.json`) | each community node | 32-byte secret | predict the node's own selections (no funds) | file 0600, never transmitted | `tx community rotate-onion` (valid next epoch) | — (low impact) |
| Genesis allocation keys (treasury, vesting, programme accounts) | owners named in the network file | eth_secp256k1 | spend allocations | hardware wallets / multisig; addresses collected via a signed, out-of-band channel | n/a (allocations move by tx) | balance monitors |
| Release signing key | maintainers | GPG / sigstore | sign binaries and checksums | hardware token; CI without long-lived secrets | revoke + re-sign | checksum mismatch reports |
| Dev mnemonics in `scripts/localnet.sh` | public | eth_secp256k1 | **test networks only** | none (published) | n/a | never fund them on a public network |

Rules (quality-security):
- Multisig signers verify calldata on an independent device; never trust the UI that prepared the transaction.
- No key is ever pasted into chat, CI logs or tickets.
- Every mainnet validator documents its signer setup before genesis (checked in the readiness review).
