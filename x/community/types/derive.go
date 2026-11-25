package types

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/big"

	"cosmossdk.io/math"

	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	commitmenttypesv2 "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types/v2"
)

// Deterministic derivations of docs/spec/community.md §5. Pure functions only.

const HashLen = 32

// RevealGasPerHash is charged per SHA-256 iteration when verifying an onion
// reveal: the EVM SHA-256 precompile price for a 32-byte input (60 + 12·1).
// Without it a reveal at max_reveal_distance is unmetered CPU (gap G-SEC-1).
const RevealGasPerHash uint64 = 72

var (
	domainSeed   = []byte("nira/comm/seed")
	domainTicket = []byte("nira/comm/ticket")
	domainHeight = []byte("nira/comm/height")
	domainStore  = []byte("nira/comm/store")
	domainKey    = []byte("nira/comm/key")
	two256       = new(big.Int).Lsh(big.NewInt(1), 256)
)

func h(parts ...[]byte) []byte {
	d := sha256.New()
	for _, p := range parts {
		d.Write(p)
	}
	return d.Sum(nil)
}

func be8(v uint64) []byte { b := make([]byte, 8); binary.BigEndian.PutUint64(b, v); return b }
func be4(v uint32) []byte { b := make([]byte, 4); binary.BigEndian.PutUint32(b, v); return b }

// NextSeed: seed_e = H("nira/comm/seed" ‖ seed_{e-1} ‖ block_hash ‖ be8(e)).
func NextSeed(prev, blockHash []byte, epoch uint64) []byte {
	if len(prev) == 0 {
		prev = make([]byte, HashLen)
	}
	return h(domainSeed, prev, blockHash, be8(epoch))
}

// RevealDistance returns d = epoch - anchorEpoch (0 if epoch ≤ anchorEpoch),
// so callers can charge RevealGasPerHash·d before calling VerifyReveal.
func RevealDistance(anchorEpoch, epoch uint64) uint64 {
	if epoch <= anchorEpoch {
		return 0
	}
	return epoch - anchorEpoch
}

// VerifyReveal checks H^d(reveal) == anchor with d = epoch - anchorEpoch,
// 1 ≤ d ≤ maxDistance, and that epoch is within the onion length.
func VerifyReveal(reveal, anchor []byte, anchorEpoch, epoch, baseEpoch uint64, onionLength, maxDistance uint32) error {
	if len(reveal) != HashLen || len(anchor) != HashLen {
		return fmt.Errorf("reveal and anchor must be %d bytes", HashLen)
	}
	if epoch < baseEpoch || epoch <= anchorEpoch {
		return fmt.Errorf("epoch %d not after anchor epoch %d / before base %d", epoch, anchorEpoch, baseEpoch)
	}
	if epoch-baseEpoch+1 > uint64(onionLength) {
		return fmt.Errorf("onion exhausted at epoch %d", epoch)
	}
	d := epoch - anchorEpoch
	if d > uint64(maxDistance) {
		return fmt.Errorf("reveal distance %d > max %d: rotate the onion", d, maxDistance)
	}
	v := reveal
	for i := uint64(0); i < d; i++ {
		v = h(v)
	}
	if !bytes.Equal(v, anchor) {
		return fmt.Errorf("H^%d(reveal) != anchor", d)
	}
	return nil
}

// Ticket = H("nira/comm/ticket" ‖ seed ‖ v_e ‖ operator).
func Ticket(seed, reveal, operator []byte) []byte {
	return h(domainTicket, seed, reveal, operator)
}

// IsSelected: t · eligible < 2^256 · target · bond (exact integer arithmetic).
func IsSelected(ticket []byte, bond, eligible math.Int, target uint32) bool {
	if !eligible.IsPositive() || !bond.IsPositive() {
		return false
	}
	lhs := new(big.Int).Mul(new(big.Int).SetBytes(ticket), eligible.BigInt())
	rhs := new(big.Int).Mul(two256, new(big.Int).Mul(big.NewInt(int64(target)), bond.BigInt()))
	return lhs.Cmp(rhs) < 0
}

// TaskSpec is the derived assignment for one task index.
type TaskSpec struct {
	Index  uint32
	Store  string
	Height int64
	Key    []byte
}

// DeriveTask returns task i for operator in the epoch starting at startHeight.
// Returns ok=false if no app-hash history is available yet (startHeight ≤ 1).
func DeriveTask(seed, operator []byte, i uint32, startHeight int64, window uint32, stores []string) (TaskSpec, bool) {
	if startHeight <= 1 || len(stores) == 0 || window == 0 {
		return TaskSpec{}, false // window = 0 found by FuzzDeriveTaskAndSelection (division by zero)
	}
	w := uint64(window)
	if avail := uint64(startHeight - 1); avail < w {
		w = avail
	}
	x := append(append(append([]byte{}, seed...), operator...), be4(i)...)
	off := binary.BigEndian.Uint64(h(domainHeight, x)[:8]) % w
	idx := binary.BigEndian.Uint64(h(domainStore, x)[:8]) % uint64(len(stores))
	return TaskSpec{
		Index:  i,
		Store:  stores[idx],
		Height: startHeight - 1 - int64(off),
		Key:    h(domainKey, x),
	}, true
}

// VerifyStateProof checks an ICS-23 MerkleProof of (store, key) at the given
// app hash: membership with value if len(value) > 0, else non-membership.
func VerifyStateProof(appHash []byte, store string, key, value, proofBz []byte) error {
	var proof commitmenttypes.MerkleProof
	if err := proof.Unmarshal(proofBz); err != nil {
		return fmt.Errorf("decode MerkleProof: %w", err)
	}
	if len(proof.Proofs) != 2 {
		return fmt.Errorf("expected 2 chained proofs (store, multistore), got %d", len(proof.Proofs))
	}
	root := commitmenttypes.NewMerkleRoot(appHash)
	path := commitmenttypesv2.NewMerklePath([]byte(store), key)
	specs := commitmenttypes.GetSDKSpecs()
	if len(value) > 0 {
		return proof.VerifyMembership(specs, root, path, value)
	}
	return proof.VerifyNonMembership(specs, root, path)
}
