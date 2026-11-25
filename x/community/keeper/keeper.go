package keeper

import (
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	corestore "cosmossdk.io/core/store"
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"nira-chain/x/community/types"
)

// Keeper of the community participation layer (docs/spec/community.md).
type Keeper struct {
	storeService corestore.KVStoreService
	cdc          codec.Codec
	addressCodec address.Codec
	// authority executes MsgUpdateParams and MsgResolveAlert (x/gov module account).
	authority []byte

	bankKeeper  types.BankKeeper
	authKeeper  types.AuthKeeper
	distrKeeper types.DistributionKeeper

	Schema          collections.Schema
	Params          collections.Item[types.Params]
	Nodes           collections.Map[[]byte, types.Node]
	TotalActiveBond collections.Item[math.Int]
	Epoch           collections.Item[types.Epoch]
	// (epoch, operator) → proven units
	EpochWork collections.Map[collections.Pair[uint64, []byte], uint32]
	// (epoch, operator, task index)
	TaskDone collections.KeySet[collections.Triple[uint64, []byte, uint32]]
	// height → app hash committed at that height (state root after block `height`)
	AppHashes collections.Map[int64, []byte]
	// (complete height, operator)
	UnbondingQueue collections.KeySet[collections.Pair[int64, []byte]]
	Alerts         collections.Map[uint64, types.Alert]
	// (height, reporter) → alert id
	AlertIndex  collections.Map[collections.Pair[int64, []byte], uint64]
	NextAlertID collections.Sequence
	// (expires height, id) for OPEN alerts
	OpenAlerts     collections.KeySet[collections.Pair[int64, uint64]]
	OpenAlertCount collections.Item[uint64]
}

func NewKeeper(
	storeService corestore.KVStoreService,
	cdc codec.Codec,
	addressCodec address.Codec,
	authority []byte,
	bankKeeper types.BankKeeper,
	authKeeper types.AuthKeeper,
	distrKeeper types.DistributionKeeper,
) Keeper {
	if _, err := addressCodec.BytesToString(authority); err != nil {
		panic(fmt.Sprintf("invalid authority address %s: %s", authority, err))
	}
	sb := collections.NewSchemaBuilder(storeService)
	k := Keeper{
		storeService: storeService,
		cdc:          cdc,
		addressCodec: addressCodec,
		authority:    authority,
		bankKeeper:   bankKeeper,
		authKeeper:   authKeeper,
		distrKeeper:  distrKeeper,

		Params:          collections.NewItem(sb, types.ParamsKey, "params", codec.CollValue[types.Params](cdc)),
		Nodes:           collections.NewMap(sb, types.NodesKey, "nodes", collections.BytesKey, codec.CollValue[types.Node](cdc)),
		TotalActiveBond: collections.NewItem(sb, types.TotalActiveBondKey, "total_active_bond", sdk.IntValue),
		Epoch:           collections.NewItem(sb, types.EpochKey, "epoch", codec.CollValue[types.Epoch](cdc)),
		EpochWork: collections.NewMap(sb, types.EpochWorkKey, "epoch_work",
			collections.PairKeyCodec(collections.Uint64Key, collections.BytesKey), collections.Uint32Value),
		TaskDone: collections.NewKeySet(sb, types.TaskDoneKey, "task_done",
			collections.TripleKeyCodec(collections.Uint64Key, collections.BytesKey, collections.Uint32Key)),
		AppHashes: collections.NewMap(sb, types.AppHashKey, "app_hashes", collections.Int64Key, collections.BytesValue),
		UnbondingQueue: collections.NewKeySet(sb, types.UnbondingQueueKey, "unbonding_queue",
			collections.PairKeyCodec(collections.Int64Key, collections.BytesKey)),
		Alerts: collections.NewMap(sb, types.AlertsKey, "alerts", collections.Uint64Key, codec.CollValue[types.Alert](cdc)),
		AlertIndex: collections.NewMap(sb, types.AlertIndexKey, "alert_index",
			collections.PairKeyCodec(collections.Int64Key, collections.BytesKey), collections.Uint64Value),
		NextAlertID: collections.NewSequence(sb, types.NextAlertIDKey, "next_alert_id"),
		OpenAlerts: collections.NewKeySet(sb, types.OpenAlertsKey, "open_alerts",
			collections.PairKeyCodec(collections.Int64Key, collections.Uint64Key)),
		OpenAlertCount: collections.NewItem(sb, types.OpenAlertCountKey, "open_alert_count", collections.Uint64Value),
	}
	schema, err := sb.Build()
	if err != nil {
		panic(err)
	}
	k.Schema = schema
	return k
}

// GetAuthority returns the module's authority.
func (k Keeper) GetAuthority() []byte { return k.authority }

// ModuleAddress is the escrow account (bonds, deposits, reward pool).
func (k Keeper) ModuleAddress() sdk.AccAddress {
	return k.authKeeper.GetModuleAddress(types.ModuleName)
}
