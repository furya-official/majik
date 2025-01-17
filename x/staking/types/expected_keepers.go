package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	vetypes "github.com/furya-official/blackfury/x/ve/types"
)

// NftKeeper defines the expected interface needed to query NFT tokens.
type NftKeeper interface {
	GetOwner(ctx sdk.Context, classID string, nftID string) sdk.AccAddress
}

type VeKeeper interface {
	LockDenom(ctx sdk.Context) string
	GetLockedAmountByUser(ctx sdk.Context, veID uint64) vetypes.LockedBalance
	SlashLockedAmountByUser(ctx sdk.Context, veID uint64, amount sdk.Int)
	SetGetDelegatedAmountByUser(getDelegatedAmount func(ctx sdk.Context, veID uint64) sdk.Int)
}
