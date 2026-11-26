package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"cosmossdk.io/math"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	"github.com/spf13/cobra"

	"nira-chain/x/community/types"
)

const (
	flagLength    = "length"
	flagOverwrite = "overwrite"
	flagInterval  = "interval"
)

// GetTxCmd returns the custom community commands; AutoCLI adds the rest.
func GetTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Community node transactions",
		DisableFlagParsing:         false,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}
	cmd.AddCommand(OnionInitCmd(), ProveCmd(), RunCmd(), StatusCmd())
	return cmd
}

// OnionInitCmd creates the local onion secret and prints the tip to register with.
func OnionInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "onion-init",
		Short: "Create the local onion secret (<home>/community/onion.json) and print the tip",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx := client.GetClientContextFromCmd(cmd)
			length, _ := cmd.Flags().GetUint32(flagLength)
			overwrite, _ := cmd.Flags().GetBool(flagOverwrite)
			o, err := newOnion(length)
			if err != nil {
				return err
			}
			if err := saveOnion(clientCtx.HomeDir, o, overwrite); err != nil {
				return err
			}
			return clientCtx.PrintString(fmt.Sprintf("onion_tip: %s\nonion_length: %d\nfile: %s\n",
				hex.EncodeToString(o.Tip()), o.Length, onionPath(clientCtx.HomeDir)))
		},
	}
	cmd.Flags().Uint32(flagLength, 100_000, "number of epochs the onion covers")
	cmd.Flags().Bool(flagOverwrite, false, "replace an existing onion file")
	flags.AddKeyringFlags(cmd.Flags())
	return cmd
}

// ProveCmd: if this node is selected in the current epoch, build and submit
// proofs for all remaining tasks. Exits cleanly when not selected.
func ProveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prove",
		Short: "Check selection for the current epoch and submit state proofs for assigned tasks",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			n, err := proveOnce(cmd, clientCtx)
			if err != nil {
				return err
			}
			return clientCtx.PrintString(fmt.Sprintf("submitted %d proof(s)\n", n))
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// RunCmd loops `prove` every interval — the community node's work loop.
// It is safe to stop and restart at any time: all state is on chain or in the onion file.
func RunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Community node work loop: prove assigned tasks every epoch (safe to stop/restart)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			interval, _ := cmd.Flags().GetDuration(flagInterval)
			for {
				clientCtx, err := client.GetClientTxContext(cmd)
				if err != nil {
					return err
				}
				n, err := proveOnce(cmd, clientCtx)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "prove: %v\n", err)
				} else if n > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "submitted %d proof(s)\n", n)
				}
				select {
				case <-cmd.Context().Done():
					return nil
				case <-time.After(interval):
				}
			}
		},
	}
	cmd.Flags().Duration(flagInterval, 30*time.Second, "polling interval")
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

func proveOnce(cmd *cobra.Command, clientCtx client.Context) (int, error) {
	ctx := context.Background()
	q := types.NewQueryClient(clientCtx)
	op := clientCtx.GetFromAddress()
	opStr := op.String()
	onion, err := loadOnion(clientCtx.HomeDir)
	if err != nil {
		return 0, err
	}
	params, err := q.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		return 0, err
	}
	epRes, err := q.CurrentEpoch(ctx, &types.QueryCurrentEpochRequest{})
	if err != nil {
		return 0, err
	}
	nodeRes, err := q.Node(ctx, &types.QueryNodeRequest{Operator: opStr})
	if err != nil {
		return 0, fmt.Errorf("node %s: %w", opStr, err)
	}
	ep, node := epRes.Epoch, nodeRes.Node
	if node.Status != types.NODE_STATUS_ACTIVE || node.RegisteredEpoch >= ep.Number || ep.Number < node.OnionBaseEpoch {
		return 0, nil // not eligible this epoch
	}
	reveal, err := onion.Layer(ep.Number, node.OnionBaseEpoch)
	if err != nil {
		return 0, err
	}
	if !types.IsSelected(types.Ticket(ep.Seed, reveal, op), node.Bond, ep.EligibleBond, params.Params.TargetCommittee) {
		return 0, nil // not selected: nothing to do, no penalty
	}
	tasks, err := q.Tasks(ctx, &types.QueryTasksRequest{Operator: opStr})
	if err != nil {
		return 0, err
	}
	var msgs []sdk.Msg
	for i, t := range tasks.Tasks {
		if uint32(i) < nodeRes.UnitsThisEpoch {
			continue // best-effort skip of already proven tasks (chain rejects duplicates anyway)
		}
		res, err := clientCtx.QueryABCI(abci.RequestQuery{
			Path: fmt.Sprintf("/store/%s/key", t.Store), Data: t.Key, Height: t.Height, Prove: true})
		if err != nil {
			return 0, fmt.Errorf("query proof %s@%d (node must keep ≥ apphash_window+epoch_blocks versions): %w", t.Store, t.Height, err)
		}
		mp, err := commitmenttypes.ConvertProofs(res.ProofOps)
		if err != nil {
			return 0, err
		}
		proofBz, err := mp.Marshal()
		if err != nil {
			return 0, err
		}
		msgs = append(msgs, &types.MsgSubmitTaskProof{
			Operator: opStr, Epoch: ep.Number, Reveal: reveal, TaskIndex: t.Index,
			Store: t.Store, Height: t.Height, Value: res.Value, Proof: proofBz,
		})
	}
	if len(msgs) == 0 {
		return 0, nil
	}
	// One tx carries all proofs of the epoch: one fee, one sequence, atomic.
	if err := tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msgs...); err != nil {
		return 0, err
	}
	return len(msgs), nil
}

// StatusCmd prints everything an operator needs to know about its node:
// registration, eligibility and (privately, from the local onion) selection
// in the current epoch, proven work, rewards, slashes, and actions required.
func StatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show this community node's state, current-epoch selection, rewards and required actions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}
			st, err := communityStatus(cmd.Context(), clientCtx)
			if err != nil {
				return err
			}
			return clientCtx.PrintObjectLegacy(st)
		},
	}
	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NodeStatus is the operator-facing summary printed by `status`.
type NodeStatus struct {
	Operator          string   `json:"operator"`
	Status            string   `json:"status"`
	Bond              string   `json:"bond"`
	Epoch             uint64   `json:"epoch"`
	NextEpochHeight   int64    `json:"next_epoch_height"`
	Eligible          bool     `json:"eligible_this_epoch"`
	Selected          bool     `json:"selected_this_epoch"`
	TasksProven       uint32   `json:"tasks_proven_this_epoch"`
	TasksAssigned     int      `json:"tasks_assigned_this_epoch"`
	UnitsTotal        uint64   `json:"units_total"`
	RewardsTotal      string   `json:"rewards_total"`
	SlashedTotal      string   `json:"slashed_total"`
	OnionEpochsLeft   uint64   `json:"onion_epochs_left"`
	RevealDistance    uint64   `json:"reveal_distance"`
	MaxRevealDistance uint32   `json:"max_reveal_distance"`
	SelectionChance   string   `json:"selection_chance_per_epoch"`
	Actions           []string `json:"actions_required"`
}

func communityStatus(ctx context.Context, clientCtx client.Context) (NodeStatus, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	q := types.NewQueryClient(clientCtx)
	op := clientCtx.GetFromAddress()
	st := NodeStatus{Operator: op.String(), Actions: []string{}}
	params, err := q.Params(ctx, &types.QueryParamsRequest{})
	if err != nil {
		return st, err
	}
	epRes, err := q.CurrentEpoch(ctx, &types.QueryCurrentEpochRequest{})
	if err != nil {
		return st, err
	}
	ep, p := epRes.Epoch, params.Params
	st.Epoch, st.NextEpochHeight = ep.Number, epRes.NextEpochHeight
	nodeRes, err := q.Node(ctx, &types.QueryNodeRequest{Operator: st.Operator})
	if err != nil {
		st.Status = "NOT_REGISTERED"
		st.Actions = append(st.Actions, fmt.Sprintf("register: tx community register-node <moniker> %s%s <onion-tip> <length>", p.MinBond, p.BondDenom))
		return st, nil
	}
	n := nodeRes.Node
	st.Status, st.Bond, st.UnitsTotal = n.Status.String(), n.Bond.String()+p.BondDenom, n.UnitsTotal
	st.RewardsTotal, st.SlashedTotal = intStr(n.RewardsTotal)+p.BondDenom, intStr(n.SlashedTotal)+p.BondDenom
	st.TasksProven, st.MaxRevealDistance = nodeRes.UnitsThisEpoch, p.MaxRevealDistance
	if ep.EligibleBond.IsPositive() {
		chance := new(big.Float).Quo(new(big.Float).SetInt(n.Bond.MulRaw(int64(p.TargetCommittee)).BigInt()), new(big.Float).SetInt(ep.EligibleBond.BigInt()))
		if chance.Cmp(big.NewFloat(1)) > 0 {
			chance = big.NewFloat(1)
		}
		f, _ := chance.Float64()
		st.SelectionChance = fmt.Sprintf("%.2f%%", f*100)
	}
	if n.Status != types.NODE_STATUS_ACTIVE {
		if n.Status == types.NODE_STATUS_UNBONDING {
			st.Actions = append(st.Actions, fmt.Sprintf("none: bond is returned at height %d", n.UnbondCompleteHeight))
		}
		return st, nil
	}
	st.RevealDistance = types.RevealDistance(n.OnionAnchorEpoch, ep.Number)
	st.Eligible = n.RegisteredEpoch < ep.Number && ep.Number >= n.OnionBaseEpoch
	if used := ep.Number - n.OnionBaseEpoch + 1; ep.Number >= n.OnionBaseEpoch && uint64(n.OnionLength) >= used {
		st.OnionEpochsLeft = uint64(n.OnionLength) - used
	}
	if st.RevealDistance > uint64(p.MaxRevealDistance) || (ep.Number >= n.OnionBaseEpoch && st.OnionEpochsLeft == 0) {
		st.Actions = append(st.Actions, "rotate the onion: tx community onion-init --overwrite, then tx community rotate-onion <tip> <length>")
	} else if st.RevealDistance > uint64(p.MaxRevealDistance)*9/10 || (st.OnionEpochsLeft > 0 && st.OnionEpochsLeft < 100) {
		st.Actions = append(st.Actions, "rotate the onion soon (reveal distance or onion length nearly exhausted)")
	}
	if st.Eligible {
		onion, err := loadOnion(clientCtx.HomeDir)
		if err != nil {
			st.Actions = append(st.Actions, "onion secret missing in this home: run status/run on the machine that registered the node")
			return st, nil
		}
		if reveal, err := onion.Layer(ep.Number, n.OnionBaseEpoch); err == nil {
			st.Selected = types.IsSelected(types.Ticket(ep.Seed, reveal, op), n.Bond, ep.EligibleBond, p.TargetCommittee)
		}
		if st.Selected {
			st.TasksAssigned = int(p.TasksPerSelection)
			if st.TasksProven < p.TasksPerSelection {
				st.Actions = append(st.Actions, "prove now: tx community prove (or keep `tx community run` running)")
			}
		}
	}
	return st, nil
}

func intStr(i math.Int) string {
	if i.IsNil() {
		return "0"
	}
	return i.String()
}
