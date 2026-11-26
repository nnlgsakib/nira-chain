package policy

import (
	"math/rand"

	sdkmath "cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
	"github.com/cosmos/cosmos-sdk/x/simulation"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"nira-chain/x/policy/types"
)

// GenerateGenesisState creates a randomized GenState of the module.
func (AppModule) GenerateGenesisState(simState *module.SimulationState) {
	accs := make([]string, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = acc.Address.String()
	}
	policyGenesis := types.GenesisState{
		Params: types.DefaultParams(),
	}
	simState.GenState[types.ModuleName] = simState.Cdc.MustMarshalJSON(&policyGenesis)
}

// RegisterStoreDecoder registers a decoder.
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {}

// WeightedOperations: x/policy replaces the SDK's MsgCreateValidator simulation
// (which hard-codes min_self_delegation = 1 and is weighted 0 in app sims) with
// one that respects the chain minimum self-bond (G-STAKE-1), so simulated
// validator sets keep growing and the AfterValidatorCreated hook is exercised.
func (am AppModule) WeightedOperations(simState module.SimulationState) []simtypes.WeightedOperation {
	return []simtypes.WeightedOperation{
		simulation.NewWeightedOperation(100, simulateCreateValidatorWithFloor(simState.TxConfig, am)),
	}
}

func simulateCreateValidatorWithFloor(txGen client.TxConfig, am AppModule) simtypes.Operation {
	return func(r *rand.Rand, app *baseapp.BaseApp, ctx sdk.Context, accs []simtypes.Account, chainID string,
	) (simtypes.OperationMsg, []simtypes.FutureOperation, error) {
		msgType := sdk.MsgTypeURL(&stakingtypes.MsgCreateValidator{})
		simAccount, _ := simtypes.RandomAcc(r, accs)
		valAddr := sdk.ValAddress(simAccount.Address)
		if am.keeper.ValidatorExists(ctx, valAddr) {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "validator already exists"), nil, nil
		}
		params, err := am.keeper.Params.Get(ctx)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "params"), nil, err
		}
		floor := params.MinSelfBond
		spendable := am.bankKeeper.SpendableCoins(ctx, simAccount.Address)
		balance := spendable.AmountOf(sdk.DefaultBondDenom)
		if balance.LT(floor.MulRaw(2)) {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "balance below min self-bond"), nil, nil
		}
		extra, err := simtypes.RandPositiveInt(r, balance.Sub(floor).QuoRaw(2))
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "amount"), nil, nil
		}
		selfBond := sdk.NewCoin(sdk.DefaultBondDenom, floor.Add(extra))
		fees, err := simtypes.RandomFees(r, ctx, spendable.Sub(selfBond))
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "fees"), nil, err
		}
		maxComm := sdkmath.LegacyNewDecWithPrec(int64(simtypes.RandIntBetween(r, 5, 100)), 2)
		comm := stakingtypes.NewCommissionRates(sdkmath.LegacyNewDecWithPrec(5, 2), maxComm, sdkmath.LegacyNewDecWithPrec(1, 2))
		desc := stakingtypes.NewDescription(simtypes.RandStringOfLength(r, 10), "", "", "", "")
		msg, err := stakingtypes.NewMsgCreateValidator(valAddr.String(), simAccount.ConsKey.PubKey(), selfBond, desc, comm, floor)
		if err != nil {
			return simtypes.NoOpMsg(types.ModuleName, msgType, "msg"), nil, err
		}
		return simulation.GenAndDeliverTx(simulation.OperationInput{
			R: r, App: app, TxGen: txGen, Msg: msg, Context: ctx, SimAccount: simAccount,
			AccountKeeper: am.authKeeper, ModuleName: stakingtypes.ModuleName,
		}, fees)
	}
}

// ProposalMsgs returns msgs used for governance proposals for simulations.
func (am AppModule) ProposalMsgs(simState module.SimulationState) []simtypes.WeightedProposalMsg {
	return []simtypes.WeightedProposalMsg{}
}
