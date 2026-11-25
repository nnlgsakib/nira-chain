package app

import (
	"cosmossdk.io/core/appmodule"
	storetypes "cosmossdk.io/store/types"
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	"github.com/cosmos/cosmos-sdk/types/module"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/spf13/cast"

	"github.com/cosmos/cosmos-sdk/codec/legacy"
	evmcryptocodec "github.com/cosmos/evm/crypto/codec"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/ethereum/eip712"
	evmmempool "github.com/cosmos/evm/mempool"
	precompiletypes "github.com/cosmos/evm/precompiles/types"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/cosmos/evm/utils"
	erc20 "github.com/cosmos/evm/x/erc20"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/feemarket"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
)

func init() {
	// manually update the power reduction by replacing micro (u) -> atto (a) evmos
	sdk.DefaultPowerReduction = utils.AttoPowerReduction

	// set default EVM denom
	evmtypes.DefaultEVMDenom = BaseDenom
	evmtypes.DefaultEVMDisplayDenom = DisplayDenom
	evmtypes.DefaultEVMExtendedDenom = BaseDenom
}

// registerEVMModules register EVM keepers and non dependency inject modules.
func (app *App) registerEVMModules(appOpts servertypes.AppOptions) error {
	// chain config
	chainID := GetEVMChainID(appOpts)

	// set up non depinject support modules store keys
	if err := app.RegisterStores(
		storetypes.NewKVStoreKey(evmtypes.StoreKey),
		storetypes.NewKVStoreKey(feemarkettypes.StoreKey),
		storetypes.NewKVStoreKey(erc20types.StoreKey),
		storetypes.NewTransientStoreKey(evmtypes.TransientKey),
		storetypes.NewTransientStoreKey(feemarkettypes.TransientKey),
	); err != nil {
		return err
	}

	// set up EVM keeper
	tracer := cast.ToString(appOpts.Get(srvflags.EVMTracer))

	app.FeeMarketKeeper = feemarketkeeper.NewKeeper(
		app.appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.GetKey(feemarkettypes.StoreKey),
		app.UnsafeFindStoreKey(feemarkettypes.TransientKey),
	)

	app.EVMKeeper = evmkeeper.NewKeeper(
		app.appCodec,
		app.GetKey(evmtypes.StoreKey),
		app.UnsafeFindStoreKey(evmtypes.TransientKey),
		app.GetStoreKeysMap(),
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AuthKeeper,
		app.BankKeeper,
		app.StakingKeeper,
		app.FeeMarketKeeper,
		&app.ConsensusParamsKeeper,
		&app.Erc20Keeper,
		chainID,
		tracer,
	).WithStaticPrecompiles(
		precompiletypes.DefaultStaticPrecompiles(
			*app.StakingKeeper,
			app.DistrKeeper,
			app.BankKeeper,
			&app.Erc20Keeper,
			&app.TransferKeeper,
			app.IBCKeeper.ChannelKeeper,
			*app.GovKeeper,
			app.SlashingKeeper,
			app.appCodec,
		),
	).WithDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{
		Denom:         BaseDenom,
		ExtendedDenom: BaseDenom,
		DisplayDenom:  DisplayDenom,
		Decimals:      evmtypes.EighteenDecimals.Uint32(),
	})

	app.Erc20Keeper = erc20keeper.NewKeeper(
		app.GetKey(erc20types.StoreKey),
		app.appCodec,
		authtypes.NewModuleAddress(govtypes.ModuleName),
		app.AuthKeeper,
		app.BankKeeper,
		app.EVMKeeper,
		app.StakingKeeper,
		&app.TransferKeeper,
	)

	// register evm modules
	if err := app.RegisterModules(
		vm.NewAppModule(app.EVMKeeper, app.AuthKeeper, app.BankKeeper, app.AuthKeeper.AddressCodec()),
		feemarket.NewAppModule(app.FeeMarketKeeper),
		erc20.NewAppModule(app.Erc20Keeper, app.AuthKeeper),
	); err != nil {
		return err
	}

	return nil
}

// setEVMMempool sets the EVM priority nonce mempool
// it is required for the ethereum json rpc server to work
func (app *App) setEVMMempool() {
	if evmtypes.GetChainConfig() != nil {
		mempoolConfig := &evmmempool.EVMMempoolConfig{
			AnteHandler:   app.BaseApp.AnteHandler(),
			BlockGasLimit: 100_000_000,
		}

		evmMempool := evmmempool.NewExperimentalEVMMempool(app.CreateQueryContext, app.Logger(), app.EVMKeeper, app.FeeMarketKeeper, app.txConfig, app.clientCtx, mempoolConfig, 4096)
		app.EVMMempool = evmMempool

		app.SetMempool(evmMempool)
		checkTxHandler := evmmempool.NewCheckTxHandler(evmMempool)
		app.SetCheckTxHandler(checkTxHandler)

		abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, app)
		abciProposalHandler.SetSignerExtractionAdapter(evmmempool.NewEthSignerExtractionAdapter(sdkmempool.NewDefaultSignerExtractionAdapter()))
		app.SetPrepareProposal(abciProposalHandler.PrepareProposalHandler())
	}
}

// RegisterPendingTxListener a function that registers a listener for pending transactions.
func (app *App) RegisterPendingTxListener(listener func(common.Hash)) {
	app.pendingTxListeners = append(app.pendingTxListeners, listener)
}

// SetClientCtx a function that sets the client context on the app, required by EVM module implementation.
func (app *App) SetClientCtx(ctx client.Context) {
	app.clientCtx = ctx
}

// GetMempool returns the mempool of the app.
// It is required by the EVM application interface.
func (app *App) GetMempool() sdkmempool.ExtMempool {
	return app.EVMMempool
}

// GetEVMChainID returns the EIP-155 chain ID used by the EVM.
//
// It is a protocol constant (ADR-0005, CHAIN.md Q2): wallets and EIP-155
// signatures bind to it, so every node must use the same value and it must
// never change after launch. It can be overridden only through the
// `evm.evm-chain-id` app option for local test networks.
func GetEVMChainID(appOpts servertypes.AppOptions) uint64 {
	if appOpts != nil {
		if v := cast.ToUint64(appOpts.Get(srvflags.EVMChainID)); v != 0 {
			return v
		}
	}
	return EVMChainID
}

// RegisterEVM Since the EVM modules don't support dependency injection,
// we need to manually register the modules on the client side.
// This needs to be removed after EVM supports App Wiring.
func RegisterEVM(cdc codec.Codec, interfaceRegistry codectypes.InterfaceRegistry) map[string]appmodule.AppModule {
	modules := map[string]appmodule.AppModule{
		evmtypes.ModuleName:       vm.NewAppModule(nil, authkeeper.AccountKeeper{}, nil, interfaceRegistry.SigningContext().AddressCodec()),
		erc20types.ModuleName:     erc20.NewAppModule(erc20keeper.Keeper{}, authkeeper.AccountKeeper{}),
		feemarkettypes.ModuleName: feemarket.NewAppModule(feemarketkeeper.Keeper{}),
	}

	for _, m := range modules { // audit:ok client-side interface registration, order-independent
		if mr, ok := m.(module.AppModuleBasic); ok {
			mr.RegisterInterfaces(cdc.InterfaceRegistry())
		}
	}

	return modules
}

// ProvideMsgEthereumTxCustomGetSigner provides a custom signer for the MsgEthereumTx message.
func ProvideMsgEthereumTxCustomGetSigner() signing.CustomGetSigner {
	return evmtypes.MsgEthereumTxCustomGetSigner
}

// RegisterEVMCrypto registers the eth_secp256k1 key types with the amino codecs
// and interface registry, and configures EIP-712 encoding. It must run on both
// the node (tx decoding, genesis) and the CLI (keyring) so that one
// eth_secp256k1 key works for Cosmos and Ethereum transactions (R-F11).
//
// The depinject-built amino codec already contains the SDK crypto types, so
// only the eth_secp256k1 concrete types are added (re-registering panics).
func RegisterEVMCrypto(amino *codec.LegacyAmino, registry codectypes.InterfaceRegistry) {
	for _, cdc := range []*codec.LegacyAmino{amino, legacy.Cdc} {
		registerEthKeys(cdc)
	}
	evmcryptocodec.RegisterInterfaces(registry)
	eip712.SetEncodingConfig(amino, registry, EVMChainID)
}

func registerEthKeys(cdc *codec.LegacyAmino) {
	defer func() { _ = recover() }() // already registered on this codec
	cdc.RegisterConcrete(&ethsecp256k1.PubKey{}, ethsecp256k1.PubKeyName, nil)
	cdc.RegisterConcrete(&ethsecp256k1.PrivKey{}, ethsecp256k1.PrivKeyName, nil)
}
