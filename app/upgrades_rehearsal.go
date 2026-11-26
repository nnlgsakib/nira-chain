//go:build rehearsal

package app

// The "rehearsal" upgrade is compiled only into the rehearsal binary
// (`go build -tags rehearsal`). It runs every module's migrations so a test
// network exercises the real release path: the normal binary halts at the plan
// height, operators switch to this binary, the handler runs, blocks continue.
// See tests/upgrade/rehearsal.sh.
func init() {
	Upgrades = append(Upgrades, Upgrade{
		Name:                 "rehearsal",
		CreateUpgradeHandler: runMigrationsHandler,
	})
}
