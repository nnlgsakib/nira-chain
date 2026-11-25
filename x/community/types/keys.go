package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name. Its module account escrows bonds,
	// alert deposits and the epoch reward pool (INV-COMM-4).
	ModuleName = "community"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	GovModuleName = "gov"
)

// Store prefixes. Never renumber after a release.
var (
	ParamsKey          = collections.NewPrefix("p_community")
	NodesKey           = collections.NewPrefix(1)
	TotalActiveBondKey = collections.NewPrefix(2)
	EpochKey           = collections.NewPrefix(3)
	EpochWorkKey       = collections.NewPrefix(4)
	TaskDoneKey        = collections.NewPrefix(5)
	AppHashKey         = collections.NewPrefix(6)
	UnbondingQueueKey  = collections.NewPrefix(7)
	AlertsKey          = collections.NewPrefix(8)
	AlertIndexKey      = collections.NewPrefix(9)
	NextAlertIDKey     = collections.NewPrefix(10)
	OpenAlertsKey      = collections.NewPrefix(11)
	OpenAlertCountKey  = collections.NewPrefix(12)
)
