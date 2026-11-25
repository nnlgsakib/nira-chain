// doublesign submits real equivocation evidence for a validator on a TEST
// network: it signs two conflicting prevotes (same height/round, different
// block IDs) with the validator's consensus key and broadcasts
// DuplicateVoteEvidence through CometBFT RPC — the same evidence CometBFT would
// gossip if the validator ran two signing nodes. Used by
// tests/consensus/check_validators.sh to verify slashing + tombstoning (G-CONS-1).
//
//	go run ./tools/doublesign --home ~/.nira-localtest/val15 --rpc http://127.0.0.1:26657
package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/cometbft/cometbft/privval"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/types"
)

func main() {
	home := flag.String("home", "", "validator home holding config/priv_validator_key.json")
	rpc := flag.String("rpc", "http://127.0.0.1:26657", "CometBFT RPC of any node")
	flag.Parse()
	if err := run(*home, *rpc); err != nil {
		fmt.Fprintln(os.Stderr, "doublesign:", err)
		os.Exit(1)
	}
}

func run(home, rpc string) error {
	pv := privval.LoadFilePVEmptyState(filepath.Join(home, "config", "priv_validator_key.json"), "")
	c, err := rpchttp.New(rpc, "/websocket")
	if err != nil {
		return err
	}
	ctx := context.Background()
	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	h := st.SyncInfo.LatestBlockHeight - 1
	blk, err := c.Block(ctx, &h)
	if err != nil {
		return err
	}
	perPage := 100
	vr, err := c.Validators(ctx, &h, nil, &perPage)
	if err != nil {
		return err
	}
	valSet := types.NewValidatorSet(vr.Validators)
	addr := pv.Key.PubKey.Address()
	idx, val := valSet.GetByAddress(addr)
	if val == nil {
		return fmt.Errorf("validator %X not in the set at height %d", addr, h)
	}
	chainID := st.NodeInfo.Network
	vote := func(seed string) (*types.Vote, error) {
		hash := sha256.Sum256([]byte(seed))
		psh := sha256.Sum256([]byte(seed + "/parts"))
		v := &types.Vote{
			Type: cmtproto.PrevoteType, Height: h, Round: 0,
			BlockID:          types.BlockID{Hash: hash[:], PartSetHeader: types.PartSetHeader{Total: 1, Hash: psh[:]}},
			Timestamp:        blk.Block.Time,
			ValidatorAddress: addr, ValidatorIndex: idx,
		}
		sig, err := pv.Key.PrivKey.Sign(types.VoteSignBytes(chainID, v.ToProto()))
		if err != nil {
			return nil, err
		}
		v.Signature = sig
		return v, nil
	}
	v1, err := vote("A")
	if err != nil {
		return err
	}
	v2, err := vote("B")
	if err != nil {
		return err
	}
	ev, err := types.NewDuplicateVoteEvidence(v1, v2, blk.Block.Time, valSet)
	if err != nil {
		return err
	}
	res, err := c.BroadcastEvidence(ctx, ev)
	if err != nil {
		return err
	}
	fmt.Printf("evidence %X for validator %X at height %d submitted\n", res.Hash, addr, h)
	return nil
}
