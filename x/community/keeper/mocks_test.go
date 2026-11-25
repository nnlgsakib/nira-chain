package keeper_test

import (
	"context"
	"fmt"

	"cosmossdk.io/collections"
	"cosmossdk.io/core/address"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// mockBank keeps balances per address (module accounts use their module address).
type mockBank struct {
	bal       map[string]sdk.Coins
	failSends bool // simulate bank failure (INV-COMM-8 isolation test)
}

func newMockBank() *mockBank { return &mockBank{bal: map[string]sdk.Coins{}} }

func modAddr(name string) sdk.AccAddress { return authtypes.NewModuleAddress(name) }

func (b *mockBank) move(from, to sdk.AccAddress, amt sdk.Coins) error {
	if b.failSends {
		return fmt.Errorf("mock bank failure")
	}
	have := b.bal[string(from)]
	if !have.IsAllGTE(amt) {
		return fmt.Errorf("insufficient funds: have %s need %s", have, amt)
	}
	b.bal[string(from)] = have.Sub(amt...)
	b.bal[string(to)] = b.bal[string(to)].Add(amt...)
	return nil
}

func (b *mockBank) SpendableCoins(_ context.Context, a sdk.AccAddress) sdk.Coins {
	return b.bal[string(a)]
}
func (b *mockBank) GetBalance(_ context.Context, a sdk.AccAddress, denom string) sdk.Coin {
	return sdk.NewCoin(denom, b.bal[string(a)].AmountOf(denom))
}
func (b *mockBank) SendCoinsFromAccountToModule(_ context.Context, from sdk.AccAddress, mod string, amt sdk.Coins) error {
	return b.move(from, modAddr(mod), amt)
}
func (b *mockBank) SendCoinsFromModuleToAccount(_ context.Context, mod string, to sdk.AccAddress, amt sdk.Coins) error {
	return b.move(modAddr(mod), to, amt)
}
func (b *mockBank) SendCoinsFromModuleToModule(_ context.Context, from, to string, amt sdk.Coins) error {
	return b.move(modAddr(from), modAddr(to), amt)
}

type mockAuth struct{}

func (mockAuth) GetModuleAddress(name string) sdk.AccAddress { return modAddr(name) }
func (mockAuth) AddressCodec() address.Codec {
	return addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())
}
func (mockAuth) GetAccount(context.Context, sdk.AccAddress) sdk.AccountI { return nil }

// mockDistr: community pool = distribution module balance.
type mockDistr struct{ bank *mockBank }

func (d mockDistr) FundCommunityPool(_ context.Context, amt sdk.Coins, from sdk.AccAddress) error {
	return d.bank.move(from, modAddr("distribution"), amt)
}

type collectionsTriple = collections.Triple[uint64, []byte, uint32]
