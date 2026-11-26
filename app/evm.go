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

	"fmt"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/client/flags"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"

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
	gethvm "github.com/ethereum/go-ethereum/core/vm"

	communityprecompile "nira-chain/precompiles/community"
	communitykeeper "nira-chain/x/community/keeper"
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
	// chain config: EIP-155 chain ID derived from the Cosmos chain-id
	chainID, err := GetEVMChainID(appOpts)
	if err != nil {
		return err
	}
	eip712.SetEncodingConfig(app.legacyAmino, app.interfaceRegistry, chainID)

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
		app.staticPrecompiles(),
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

// staticPrecompiles returns the cosmos/evm default precompiles plus Nira's
// x/community precompile at 0x…0900 (ADR-0006). Precompiles are only callable
// when listed in evm params active_static_precompiles.
func (app *App) staticPrecompiles() map[common.Address]gethvm.PrecompiledContract {
	pcs := precompiletypes.DefaultStaticPrecompiles(
		*app.StakingKeeper,
		app.DistrKeeper,
		app.BankKeeper,
		&app.Erc20Keeper,
		&app.TransferKeeper,
		app.IBCKeeper.ChannelKeeper,
		*app.GovKeeper,
		app.SlashingKeeper,
		app.appCodec,
	)
	cp := communityprecompile.NewPrecompile(
		communitykeeper.NewMsgServerImpl(app.CommunityKeeper),
		communitykeeper.NewQueryServerImpl(app.CommunityKeeper),
		app.BankKeeper,
		app.AuthKeeper.AddressCodec(),
	)
	pcs[cp.Address()] = cp
	return pcs
}

// setEVMMempool sets the EVM priority nonce mempool
// it is required for the ethereum json rpc server to work
func (app *App) setEVMMempool() {
	if evmtypes.GetChainConfig() != nil {
		mempoolConfig := &evmmempool.EVMMempoolConfig{
			AnteHandler:   app.BaseApp.AnteHandler(),
			BlockGasLimit: 100_000_000,
		}

		evmMempool := evmmempool.NewExperimentalEVMMempool(app.CreateQueryContext, newMempoolLogger(app.Logger()), app.EVMKeeper, app.FeeMarketKeeper, app.txConfig, app.clientCtx, mempoolConfig, 4096)
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
// It is derived from the Cosmos chain-id (nira_<evm-chain-id>-<revision>), so
// it is fixed by genesis and identical on every node: mainnet 91000, testnet
// 92000, local 93000. The `evm.evm-chain-id` app option may be left at 0
// (derive); if an operator sets it, it must equal the derived value, otherwise
// the node refuses to start rather than sign for the wrong network.
func GetEVMChainID(appOpts servertypes.AppOptions) (uint64, error) {
	var configured uint64
	if appOpts != nil {
		configured = cast.ToUint64(appOpts.Get(srvflags.EVMChainID))
	}
	cosmosID := cosmosChainID(appOpts)
	if cosmosID == "" { // no genesis available (unit tests, simulation)
		if configured != 0 {
			return configured, nil
		}
		return EVMChainID, nil
	}
	derived, err := ParseEVMChainID(cosmosID)
	if err != nil {
		return 0, err
	}
	if configured != 0 && configured != derived {
		return 0, fmt.Errorf("app.toml evm.evm-chain-id = %d but chain-id %s means EVM chain ID %d; set it to 0 or %d",
			configured, cosmosID, derived, derived)
	}
	return derived, nil
}

// cosmosChainID resolves the chain-id the same way the SDK server does:
// the --chain-id flag, else the genesis file in the node home.
func cosmosChainID(appOpts servertypes.AppOptions) string {
	if appOpts == nil {
		return ""
	}
	if id := cast.ToString(appOpts.Get(flags.FlagChainID)); id != "" {
		return id
	}
	home := cast.ToString(appOpts.Get(flags.FlagHome))
	if home == "" {
		return ""
	}
	gen, _ := appOpts.Get("genesis_file").(string)
	if gen == "" {
		gen = filepath.Join("config", "genesis.json")
	}
	f, err := os.Open(filepath.Join(home, gen))
	if err != nil {
		return ""
	}
	defer f.Close()
	id, err := genutiltypes.ParseChainIDFromGenesis(f)
	if err != nil {
		return ""
	}
	return id
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
