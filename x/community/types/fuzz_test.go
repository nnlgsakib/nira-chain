package types_test

import (
	"crypto/sha256"
	"testing"

	"cosmossdk.io/math"

	"nira-chain/x/community/types"
)

// G-SEC-3: attacker-controlled bytes must never panic the verifier, and a
// random proof must never verify against a random root.
func FuzzVerifyStateProof(f *testing.F) {
	f.Add(make([]byte, 32), "acc", []byte("key"), []byte{}, []byte{0x0a, 0x00})
	f.Add([]byte{1}, "evm", []byte{}, []byte("v"), []byte{0x0a, 0x02, 0x0a, 0x00, 0x0a, 0x00})
	f.Fuzz(func(t *testing.T, root []byte, store string, key, value, proof []byte) {
		if err := types.VerifyStateProof(root, store, key, value, proof); err == nil {
			t.Fatalf("random proof verified: root=%x store=%q key=%x value=%x proof=%x", root, store, key, value, proof)
		}
	})
}

func FuzzVerifyReveal(f *testing.F) {
	tip := sha256.Sum256(make([]byte, 32))
	f.Add(make([]byte, 32), tip[:], uint64(0), uint64(1), uint64(1), uint32(10), uint32(100))
	f.Fuzz(func(t *testing.T, reveal, anchor []byte, anchorEpoch, epoch, base uint64, length, maxD uint32) {
		if maxD > 10_000 { // keep the fuzzer fast; the chain bounds this by param + gas
			maxD = 10_000
		}
		err := types.VerifyReveal(reveal, anchor, anchorEpoch, epoch, base, length, maxD)
		if err == nil {
			// soundness: an accepted reveal must hash forward to the anchor
			v := reveal
			for i := uint64(0); i < epoch-anchorEpoch; i++ {
				s := sha256.Sum256(v)
				v = s[:]
			}
			if string(v) != string(anchor) {
				t.Fatalf("accepted reveal does not hash to anchor")
			}
		}
	})
}

func FuzzDeriveTaskAndSelection(f *testing.F) {
	f.Add([]byte("seed"), []byte("operator-20-bytes!!!"), uint32(0), int64(100), uint32(10), uint64(5), uint64(10), uint32(64))
	f.Fuzz(func(t *testing.T, seed, op []byte, i uint32, start int64, window uint32, bond, eligible uint64, target uint32) {
		stores := []string{"acc", "evm", "distribution", "slashing"}
		task, ok := types.DeriveTask(seed, op, i, start, window, stores)
		if ok {
			if task.Height >= start || task.Height < start-1-int64(window) || task.Height < 1 {
				t.Fatalf("task height %d outside window for start %d window %d", task.Height, start, window)
			}
			if len(task.Key) != 32 {
				t.Fatalf("key length %d", len(task.Key))
			}
		}
		// selection must never panic and must be monotone in bond
		tk := types.Ticket(seed, seed, op)
		lo := types.IsSelected(tk, math.NewIntFromUint64(bond), math.NewIntFromUint64(eligible), target)
		hi := types.IsSelected(tk, math.NewIntFromUint64(bond).AddRaw(1), math.NewIntFromUint64(eligible), target)
		if lo && !hi {
			t.Fatalf("selection not monotone in bond")
		}
	})
}
