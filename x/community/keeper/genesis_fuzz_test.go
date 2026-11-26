package keeper_test

import (
	"bytes"
	"testing"

	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"

	module "nira-chain/x/community/module"
	"nira-chain/x/community/types"
)

// G-SEC-3: arbitrary genesis JSON. Whatever passes Validate must import
// without panicking; whatever imports must export a genesis that validates
// and round-trips exactly (import(export(g)) exports the same bytes).
func FuzzGenesisImport(f *testing.F) {
	cdc := moduletestutil.MakeTestEncodingConfig(module.AppModule{}).Codec
	def, err := cdc.MarshalJSON(types.DefaultGenesis())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(def)
	op := sdk.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	anchor := `"AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="`
	params := string(def[bytes.Index(def, []byte(`"params":`))+9 : bytes.Index(def, []byte(`,"nodes"`))])
	f.Add([]byte(`{"params":` + params + `,"nodes":[{"operator":"` + op + `","bond":"1000","status":"NODE_STATUS_ACTIVE","onion_anchor":` + anchor + `}],` +
		`"alerts":[{"id":"1","reporter":"` + op + `","height":"5","deposit":"7","status":"ALERT_STATUS_OPEN","expires_height":"50"}],"next_alert_id":"2",` +
		`"epoch_work":[{"epoch":"3","operator":"` + op + `","units":2}],"task_done":[{"epoch":"3","operator":"` + op + `","task_index":1}],` +
		`"app_hashes":[{"height":"4","app_hash":` + anchor + `}]}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		var gs types.GenesisState
		if cdc.UnmarshalJSON(data, &gs) != nil || gs.Validate() != nil {
			return
		}
		fx := initFixture(t)
		if err := fx.keeper.InitGenesis(fx.ctx, gs); err != nil {
			return // rejected cleanly
		}
		out, err := fx.keeper.ExportGenesis(fx.ctx)
		if err != nil {
			t.Fatalf("export after a successful import failed: %v", err)
		}
		if err := out.Validate(); err != nil {
			t.Fatalf("exported genesis does not validate: %v", err)
		}
		first, _ := cdc.MarshalJSON(out)

		fx2 := initFixture(t)
		if err := fx2.keeper.InitGenesis(fx2.ctx, *out); err != nil {
			t.Fatalf("re-import of exported genesis failed: %v", err)
		}
		out2, err := fx2.keeper.ExportGenesis(fx2.ctx)
		if err != nil {
			t.Fatal(err)
		}
		second, _ := cdc.MarshalJSON(out2)
		if !bytes.Equal(first, second) {
			t.Fatalf("genesis round-trip not stable:\n%s\n%s", first, second)
		}
	})
}
