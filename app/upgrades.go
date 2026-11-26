package app

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/types/module"
)

// Upgrade is one named software upgrade (G-OPS-1). Each governance
// MsgSoftwareUpgrade plan name must match exactly one entry; the binary that
// contains the entry is the one operators run from the upgrade height on.
type Upgrade struct {
	// Name equals the on-chain upgrade plan name.
	Name string
	// CreateUpgradeHandler returns the state migration run at the plan height.
	CreateUpgradeHandler func(mm *module.Manager, cfg module.Configurator, app *App) upgradetypes.UpgradeHandler
	// StoreUpgrades lists KV stores added / renamed / deleted by this upgrade.
	StoreUpgrades storetypes.StoreUpgrades
}

// Upgrades is the list of upgrades this binary can apply. It must contain ONLY
// the upgrade(s) this release is the target of: x/upgrade halts the chain
// ("BINARY UPDATED BEFORE TRIGGER") if a binary carries the handler of a plan
// that is scheduled but not yet reached. Release flow:
//
//	release N   : Upgrades = {}           → halts at the plan height ("UPGRADE NEEDED")
//	release N+1 : Upgrades = {"vN+1": …} → operators switch binary, handler runs, chain continues
//
// Upgrade entries live in upgrades_<name>.go files that append to this slice;
// the test-network rehearsal entry is only compiled with `-tags rehearsal`.
var Upgrades []Upgrade

// runMigrationsHandler is the standard handler: run module migrations from the
// stored version map to the binary's ConsensusVersion of every module.
func runMigrationsHandler(mm *module.Manager, cfg module.Configurator, _ *App) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		return mm.RunMigrations(ctx, cfg, fromVM)
	}
}

// setupUpgrades registers every handler and, when the node restarts at an
// upgrade height, the store loader for that upgrade's store changes.
// Must run before app.Load.
func (app *App) setupUpgrades() {
	seen := map[string]bool{}
	for _, u := range Upgrades {
		if seen[u.Name] {
			panic(fmt.Sprintf("duplicate upgrade name %q", u.Name))
		}
		seen[u.Name] = true
		app.UpgradeKeeper.SetUpgradeHandler(u.Name, u.CreateUpgradeHandler(app.ModuleManager, app.Configurator(), app))
	}

	info, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(fmt.Errorf("read upgrade info from disk: %w", err))
	}
	if info.Name == "" || app.UpgradeKeeper.IsSkipHeight(info.Height) {
		return
	}
	for _, u := range Upgrades {
		if u.Name == info.Name && (len(u.StoreUpgrades.Added) > 0 || len(u.StoreUpgrades.Renamed) > 0 || len(u.StoreUpgrades.Deleted) > 0) {
			app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(info.Height, &u.StoreUpgrades))
		}
	}
}
