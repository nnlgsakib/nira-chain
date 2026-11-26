package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// DefaultGenesis returns the default genesis state.
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:      DefaultParams(),
		NextAlertId: 1,
	}
}

// Validate performs stateless genesis validation.
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, n := range gs.Nodes {
		if n.Operator == "" || seen[n.Operator] {
			return fmt.Errorf("duplicate or empty node operator %q", n.Operator)
		}
		seen[n.Operator] = true
		if n.Bond.IsNil() || n.Bond.LT(math.ZeroInt()) {
			return fmt.Errorf("node %s: invalid bond", n.Operator)
		}
		if n.Status != NODE_STATUS_ACTIVE && n.Status != NODE_STATUS_UNBONDING {
			return fmt.Errorf("node %s: invalid status", n.Operator)
		}
		if len(n.OnionAnchor) != HashLen {
			return fmt.Errorf("node %s: onion anchor must be %d bytes", n.Operator, HashLen)
		}
	}
	ids := map[uint64]bool{}
	open := 0
	for _, a := range gs.Alerts {
		if a.Id == 0 || a.Id >= gs.NextAlertId || ids[a.Id] {
			return fmt.Errorf("invalid or duplicate alert id %d", a.Id)
		}
		ids[a.Id] = true
		if a.Status == ALERT_STATUS_OPEN {
			open++
		}
	}
	if len(gs.Alerts) > 0 && gs.NextAlertId == 0 {
		return fmt.Errorf("next_alert_id must be set")
	}
	if open > int(gs.Params.MaxOpenAlerts) {
		return fmt.Errorf("open alerts %d > max %d", open, gs.Params.MaxOpenAlerts)
	}
	return nil
}
