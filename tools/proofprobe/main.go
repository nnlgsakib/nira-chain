// proofprobe measures, per store, how often a random-key state proof from a
// live node verifies against the committed app hash (diagnostic tool).
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	rpcclient "github.com/cometbft/cometbft/rpc/client"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"

	"nira-chain/x/community/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
)

func main() {
	c, err := rpchttp.New("http://127.0.0.1:26657", "/websocket")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	st, err := c.Status(ctx)
	if err != nil {
		panic(err)
	}
	h := st.SyncInfo.LatestBlockHeight - 2
	next := h + 1
	hdr, err := c.Header(ctx, &next)
	if err != nil {
		panic(err)
	}
	appHash := hdr.Header.AppHash
	stores := os.Args[1:]
	for _, s := range stores {
		okN, failN, mem := 0, 0, 0
		var lastErr error
		for i := 0; i < 200; i++ {
			key := make([]byte, 32)
			rand.Read(key)
			res, err := c.ABCIQueryWithOptions(ctx, "/store/"+s+"/key", key, rpcOpts(h))
			if err != nil {
				panic(err)
			}
			mp, err := commitmenttypes.ConvertProofs(res.Response.ProofOps)
			if err != nil {
				failN++
				lastErr = err
				continue
			}
			bz, _ := mp.Marshal()
			if len(res.Response.Value) > 0 {
				mem++
			}
			if err := types.VerifyStateProof(appHash, s, key, res.Response.Value, bz); err != nil {
				failN++
				lastErr = err
			} else {
				okN++
			}
		}
		fmt.Printf("%-12s ok=%d fail=%d membership=%d lastErr=%v\n", s, okN, failN, mem, lastErr)
	}
}

func rpcOpts(h int64) rpcclient.ABCIQueryOptions { return rpcclient.ABCIQueryOptions{Height: h, Prove: true} }
