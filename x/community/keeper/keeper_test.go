package keeper_test

import (
	"context"
	"crypto/sha256"
	"testing"

	"cosmossdk.io/core/address"
	coreheader "cosmossdk.io/core/header"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store/metrics"
	"cosmossdk.io/store/rootmulti"
	storetypes "cosmossdk.io/store/types"
	dbm "github.com/cosmos/cosmos-db"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	"github.com/stretchr/testify/require"

	"nira-chain/x/community/keeper"
	module "nira-chain/x/community/module"
	"nira-chain/x/community/types"
)

const denom = "anira"

type fixture struct {
	ctx          sdk.Context
	keeper       keeper.Keeper
	msg          types.MsgServer
	query        types.QueryServer
	addressCodec address.Codec
	bank         *mockBank
	gov          string
	chain        *proofChain
}

func nira(n int64) math.Int { return math.NewIntWithDecimal(n, 18) }

// testParams: small epochs and windows so heights stay low; target large so
// every eligible node is selected unless a test lowers it.
func testParams() types.Params {
	p := types.DefaultParams()
	p.EpochBlocks = 10
	p.ApphashWindow = 10
	p.TargetCommittee = 1000
	p.MaxProofsPerEpoch = 2000
	p.SampleStores = []string{"bank", "acc"}
	p.UnbondingBlocks = 5
	p.AlertTtlBlocks = 100
	return p
}

func initFixture(t *testing.T) *fixture {
	t.Helper()
	encCfg := moduletestutil.MakeTestEncodingConfig(module.AppModule{})
	addressCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
	storeKey := storetypes.NewKVStoreKey(types.StoreKey)
	ctx := testutil.DefaultContextWithDB(t, storeKey, storetypes.NewTransientStoreKey("transient_test")).Ctx
	authority := authtypes.NewModuleAddress(types.GovModuleName)
	bank := newMockBank()
	k := keeper.NewKeeper(runtime.NewKVStoreService(storeKey), encCfg.Codec, addressCodec, authority,
		bank, mockAuth{}, mockDistr{bank: bank})
	require.NoError(t, k.Params.Set(ctx, testParams()))
	gov, _ := addressCodec.BytesToString(authority)
	return &fixture{
		ctx: ctx, keeper: k, msg: keeper.NewMsgServerImpl(k), query: keeper.NewQueryServerImpl(k),
		addressCodec: addressCodec, bank: bank, gov: gov, chain: newProofChain(t),
	}
}

// proofChain is a real IAVL multistore with stores "bank" and "acc"; each
// block commits one version, and its commit hash is what the community module
// records as AppHash[height] — exactly as on a live chain.
type proofChain struct {
	t      *testing.T
	ms     *rootmulti.Store
	keys   map[string]*storetypes.KVStoreKey
	hashes map[int64][]byte
}

func newProofChain(t *testing.T) *proofChain {
	ms := rootmulti.NewStore(dbm.NewMemDB(), log.NewNopLogger(), metrics.NewNoOpMetrics())
	keys := map[string]*storetypes.KVStoreKey{}
	for _, n := range []string{"bank", "acc"} {
		keys[n] = storetypes.NewKVStoreKey(n)
		ms.MountStoreWithDB(keys[n], storetypes.StoreTypeIAVL, nil)
	}
	require.NoError(t, ms.LoadLatestVersion())
	return &proofChain{t: t, ms: ms, keys: keys, hashes: map[int64][]byte{}}
}

// commit writes some state and commits version h.
func (c *proofChain) commit(h int64) []byte {
	for n, key := range c.keys {
		s := c.ms.GetKVStore(key)
		for i := 0; i < 5; i++ {
			k := sha256.Sum256([]byte{byte(h), byte(i), n[0]})
			s.Set(k[:], []byte{byte(h), byte(i)})
		}
	}
	id := c.ms.Commit()
	require.Equal(c.t, h, id.Version)
	c.hashes[h] = id.Hash
	return id.Hash
}

// prove returns (value, MerkleProof bytes) for key in store at height.
func (c *proofChain) prove(store string, key []byte, height int64) ([]byte, []byte) {
	res, err := c.ms.Query(&storetypes.RequestQuery{Path: "/" + store + "/key", Data: key, Height: height, Prove: true})
	require.NoError(c.t, err)
	mp, err := commitmenttypes.ConvertProofs(res.ProofOps)
	require.NoError(c.t, err)
	bz, err := mp.Marshal()
	require.NoError(c.t, err)
	return res.Value, bz
}

// advance runs blocks up to height `to`: commit the proof chain, then
// BeginBlock with header.AppHash = commit(h-1) and a deterministic block hash,
// then EndBlock.
func (f *fixture) advance(t *testing.T, to int64) {
	for h := f.ctx.BlockHeight() + 1; h <= to; h++ {
		var appHash []byte
		if h > 1 {
			appHash = f.chain.commit(h - 1)
		}
		bh := sha256.Sum256([]byte{byte(h), byte(h >> 8)})
		f.ctx = f.ctx.WithBlockHeight(h).WithHeaderInfo(coreheader.Info{Height: h, Hash: bh[:], AppHash: appHash}).
			WithEventManager(sdk.NewEventManager())
		require.NoError(t, f.keeper.BeginBlock(f.ctx))
		require.NoError(t, f.keeper.EndBlock(f.ctx))
	}
}

// operator is a test account with funds, a secret onion and helpers.
type operator struct {
	addr   sdk.AccAddress
	bech   string
	secret []byte
	length uint32
}

func (f *fixture) newOperator(t *testing.T, i byte, funds math.Int) *operator {
	addr := sdk.AccAddress(append(make([]byte, 19), i))
	bech, err := f.addressCodec.BytesToString(addr)
	require.NoError(t, err)
	f.bank.bal[string(addr)] = sdk.NewCoins(sdk.NewCoin(denom, funds))
	return &operator{addr: addr, bech: bech, secret: sha256.New().Sum([]byte{i}), length: 1000}
}

func hashN(v []byte, n uint32) []byte {
	for i := uint32(0); i < n; i++ {
		s := sha256.Sum256(v)
		v = s[:]
	}
	return v
}

func (o *operator) tip() []byte { return hashN(o.secret, o.length) }

// layer returns v_e for an onion with base epoch b: H^{K-(e-b+1)}(s).
func (o *operator) layer(e, base uint64) []byte {
	return hashN(o.secret, o.length-uint32(e-base+1))
}

func (f *fixture) register(t *testing.T, o *operator, bond math.Int) *types.MsgRegisterNodeResponse {
	res, err := f.msg.RegisterNode(f.ctx, &types.MsgRegisterNode{
		Operator: o.bech, Moniker: "node", Bond: sdk.NewCoin(denom, bond), OnionTip: o.tip(), OnionLength: o.length})
	require.NoError(t, err)
	return res
}

// proofMsg builds a valid MsgSubmitTaskProof for task i of the current epoch.
func (f *fixture) proofMsg(t *testing.T, o *operator, i uint32) *types.MsgSubmitTaskProof {
	ep, err := f.keeper.Epoch.Get(f.ctx)
	require.NoError(t, err)
	node, err := f.keeper.Nodes.Get(f.ctx, o.addr)
	require.NoError(t, err)
	p, _ := f.keeper.Params.Get(f.ctx)
	task := f.keeper.Tasks(ep, p, o.addr)[i]
	val, proof := f.chain.prove(task.Store, task.Key, task.Height)
	return &types.MsgSubmitTaskProof{
		Operator: o.bech, Epoch: ep.Number, Reveal: o.layer(ep.Number, node.OnionBaseEpoch),
		TaskIndex: i, Store: task.Store, Height: task.Height, Value: val, Proof: proof,
	}
}

func (f *fixture) moduleBalance() math.Int {
	return f.bank.bal[string(modAddr(types.ModuleName))].AmountOf(denom)
}

func (f *fixture) communityPool() math.Int {
	return f.bank.bal[string(modAddr("distribution"))].AmountOf(denom)
}

// checkInvariants asserts INV-COMM-4, -5, -6, -7 and -10 on the current state.
func (f *fixture) checkInvariants(t *testing.T) {
	t.Helper()
	ctx := context.Context(f.ctx)
	p, _ := f.keeper.Params.Get(ctx)
	active, unbonding := math.ZeroInt(), math.ZeroInt()
	require.NoError(t, f.keeper.Nodes.Walk(ctx, nil, func(_ []byte, n types.Node) (bool, error) {
		if n.Status == types.NODE_STATUS_ACTIVE {
			active = active.Add(n.Bond)
		} else {
			unbonding = unbonding.Add(n.Bond)
		}
		return false, nil
	}))
	total, err := f.keeper.TotalActiveBond.Get(ctx)
	if err != nil {
		total = math.ZeroInt()
	}
	require.True(t, total.Equal(active), "INV-COMM-5: total %s != Σ active %s", total, active)

	deposits, open := math.ZeroInt(), uint64(0)
	require.NoError(t, f.keeper.Alerts.Walk(ctx, nil, func(_ uint64, a types.Alert) (bool, error) {
		if a.Status == types.ALERT_STATUS_OPEN {
			deposits = deposits.Add(a.Deposit)
			open++
		}
		return false, nil
	}))
	require.LessOrEqual(t, open, uint64(p.MaxOpenAlerts), "INV-COMM-10")
	budget := math.ZeroInt()
	if ep, err := f.keeper.Epoch.Get(ctx); err == nil {
		budget = ep.Budget
		require.LessOrEqual(t, ep.Proofs, p.MaxProofsPerEpoch, "INV-COMM-6")
	}
	want := active.Add(unbonding).Add(deposits).Add(budget)
	require.True(t, f.moduleBalance().Equal(want), "INV-COMM-4: module %s != %s", f.moduleBalance(), want)

	n := 0
	require.NoError(t, f.keeper.AppHashes.Walk(ctx, nil, func(int64, []byte) (bool, error) { n++; return false, nil }))
	require.LessOrEqual(t, n, int(p.ApphashWindow+uint32(p.EpochBlocks))+2, "INV-COMM-7")
}
