package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

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
	cmd.AddCommand(OnionInitCmd(), ProveCmd(), RunCmd())
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
