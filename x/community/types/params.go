package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// DefaultBondDenom is the chain's base denom (app.BaseDenom).
const DefaultBondDenom = "anira"

func nira(n int64) math.Int { return math.NewIntWithDecimal(n, 18) }

// DefaultParams returns the (proposed) defaults of docs/spec/community.md §7.
func DefaultParams() Params {
	return Params{
		BondDenom:         DefaultBondDenom,
		MinBond:           nira(1_000),
		EpochBlocks:       600,
		TargetCommittee:   64,
		TasksPerSelection: 2,
		MaxProofsPerEpoch: 1024,
		ApphashWindow:     100,
		// Stores whose entries always carry values: ICS-23 cannot prove non-membership
		// next to an empty-value leaf (collections KeySets) or in an empty tree.
		// Measured with tools/proofprobe (CHAIN.md K8).
		SampleStores:      []string{"acc", "evm", "distribution", "slashing"},
		MaxProofBytes:     16_384,
		ProofGasPerByte:   20,
		MaxOnionLength:    1_000_000,
		MaxRevealDistance: 4_096,
		RewardShareBps:    1_500,
		UnbondingBlocks:   100_800,
		MaxUnbondPerBlock: 100,
		AlertDeposit:      nira(100),
		AlertTtlBlocks:    100_800,
		MaxOpenAlerts:     256,
		AlertThresholdBps: 3_334,
	}
}

func inRange[T ~uint32 | ~uint64](name string, v, lo, hi T) error {
	if v < lo || v > hi {
		return fmt.Errorf("%s must be in [%d, %d], got %d", name, lo, hi, v)
	}
	return nil
}

func intInRange(name string, v, lo, hi math.Int) error {
	if v.IsNil() || v.LT(lo) || v.GT(hi) {
		return fmt.Errorf("%s must be in [%s, %s], got %s", name, lo, hi, v)
	}
	return nil
}

// Validate checks every parameter against the bounds of spec §7.
func (p Params) Validate() error {
	if p.BondDenom == "" {
		return fmt.Errorf("bond_denom must be set")
	}
	checks := []error{
		intInRange("min_bond", p.MinBond, nira(1), nira(1_000_000_000)),
		inRange("epoch_blocks", p.EpochBlocks, 10, 100_000),
		inRange("target_committee", p.TargetCommittee, 1, 10_000),
		inRange("tasks_per_selection", p.TasksPerSelection, 1, 16),
		inRange("max_proofs_per_epoch", p.MaxProofsPerEpoch, 1, 10_000),
		inRange("apphash_window", p.ApphashWindow, 10, 10_000),
		inRange("max_proof_bytes", p.MaxProofBytes, 1_024, 65_536),
		inRange("proof_gas_per_byte", p.ProofGasPerByte, 1, 1_000),
		inRange("max_onion_length", p.MaxOnionLength, 1, 10_000_000),
		inRange("max_reveal_distance", p.MaxRevealDistance, 1, 100_000),
		inRange("reward_share_bps", p.RewardShareBps, 0, 3_000),
		inRange("unbonding_blocks", p.UnbondingBlocks, 1, 2_000_000),
		inRange("max_unbond_per_block", p.MaxUnbondPerBlock, 1, 10_000),
		intInRange("alert_deposit", p.AlertDeposit, nira(1), nira(1_000_000)),
		inRange("alert_ttl_blocks", p.AlertTtlBlocks, 100, 2_000_000),
		inRange("max_open_alerts", p.MaxOpenAlerts, 1, 10_000),
		inRange("alert_threshold_bps", p.AlertThresholdBps, 1, 10_000),
	}
	for _, err := range checks {
		if err != nil {
			return err
		}
	}
	if uint64(p.TasksPerSelection)*uint64(p.TargetCommittee) > uint64(p.MaxProofsPerEpoch) {
		return fmt.Errorf("tasks_per_selection × target_committee (%d) must be ≤ max_proofs_per_epoch (%d)",
			uint64(p.TasksPerSelection)*uint64(p.TargetCommittee), p.MaxProofsPerEpoch)
	}
	if n := len(p.SampleStores); n < 1 || n > 16 {
		return fmt.Errorf("sample_stores must have 1..16 entries, got %d", n)
	}
	seen := map[string]bool{}
	for _, s := range p.SampleStores {
		if s == "" || seen[s] {
			return fmt.Errorf("sample_stores entries must be unique and non-empty")
		}
		seen[s] = true
	}
	return nil
}
