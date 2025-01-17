package keeper

import (
	"context"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	blackfury "github.com/furya-official/blackfury/types"
	"github.com/furya-official/blackfury/x/maker/types"
	oracletypes "github.com/furya-official/blackfury/x/oracle/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns an implementation of the MsgServer interface
// for the provided Keeper.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (m msgServer) MintBySwap(c context.Context, msg *types.MsgMintBySwap) (*types.MsgMintBySwapResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	backingIn, furyIn, mintOut, mintFee, err := m.Keeper.calculateMintBySwapOut(ctx, msg.BackingInMax, msg.FuryInMax, msg.FullBacking)
	if err != nil {
		return nil, err
	}
	mintTotal := mintOut.Add(mintFee)

	if mintOut.IsLT(msg.MintOutMin) {
		return nil, sdkerrors.Wrapf(types.ErrOverSlippage, "mint out: %s", mintOut)
	}

	totalBacking, poolBacking, err := m.Keeper.getBacking(ctx, msg.BackingInMax.Denom)
	if err != nil {
		return nil, err
	}

	poolBacking.MerMinted = poolBacking.MerMinted.Add(mintTotal)
	poolBacking.Backing = poolBacking.Backing.Add(backingIn)
	poolBacking.FuryBurned = poolBacking.FuryBurned.Add(furyIn)

	totalBacking.MerMinted = totalBacking.MerMinted.Add(mintTotal)
	totalBacking.FuryBurned = totalBacking.FuryBurned.Add(furyIn)

	m.Keeper.SetPoolBacking(ctx, poolBacking)
	m.Keeper.SetTotalBacking(ctx, totalBacking)

	// take backing and fury coin
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(backingIn, furyIn))
	if err != nil {
		return nil, err
	}
	// burn fury
	if furyIn.IsPositive() {
		err = m.Keeper.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(furyIn))
		if err != nil {
			return nil, err
		}
	}

	// mint black stablecoin
	err = m.Keeper.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(mintTotal))
	if err != nil {
		return nil, err
	}
	// send black to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(mintOut))
	if err != nil {
		return nil, err
	}
	// send black fee to oracle
	if mintFee.IsPositive() {
		err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(mintFee))
		if err != nil {
			return nil, err
		}
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeMintBySwap,
			sdk.NewAttribute(types.AttributeKeyCoinIn, sdk.NewCoins(backingIn, furyIn).String()),
			sdk.NewAttribute(types.AttributeKeyCoinOut, mintOut.String()),
			sdk.NewAttribute(types.AttributeKeyFee, mintFee.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgMintBySwapResponse{
		BackingIn: backingIn,
		FuryIn:    furyIn,
		MintOut:   mintOut,
		MintFee:   mintFee,
	}, nil
}

func (m msgServer) BurnBySwap(c context.Context, msg *types.MsgBurnBySwap) (*types.MsgBurnBySwapResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	backingOut, furyOut, burnFee, err := m.Keeper.calculateBurnBySwapOut(ctx, msg.BurnIn, msg.BackingOutMin.Denom)
	if err != nil {
		return nil, err
	}
	burnActual := msg.BurnIn.Sub(burnFee)

	if backingOut.IsLT(msg.BackingOutMin) {
		return nil, sdkerrors.Wrapf(types.ErrOverSlippage, "backing out: %s", backingOut)
	}
	if furyOut.IsLT(msg.FuryOutMin) {
		return nil, sdkerrors.Wrapf(types.ErrOverSlippage, "fury out: %s", furyOut)
	}

	totalBacking, poolBacking, err := m.Keeper.getBacking(ctx, msg.BackingOutMin.Denom)
	if err != nil {
		return nil, err
	}

	poolBacking.Backing = poolBacking.Backing.Sub(backingOut)
	// allow FuryBurned to be negative which means minted fury
	// here use Int.Sub() to bypass Coin.Sub() negativeness check
	poolBacking.FuryBurned.Amount = poolBacking.FuryBurned.Amount.Sub(furyOut.Amount)
	totalBacking.FuryBurned.Amount = totalBacking.FuryBurned.Amount.Sub(furyOut.Amount)
	// allow MerMinted to be negative which means burned black
	poolBacking.MerMinted.Amount = poolBacking.MerMinted.Amount.Sub(burnActual.Amount)
	totalBacking.MerMinted.Amount = totalBacking.MerMinted.Amount.Sub(burnActual.Amount)

	m.Keeper.SetPoolBacking(ctx, poolBacking)
	m.Keeper.SetTotalBacking(ctx, totalBacking)

	// take black stablecoin
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(msg.BurnIn))
	if err != nil {
		return nil, err
	}
	// burn black
	err = m.Keeper.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(burnActual))
	if err != nil {
		return nil, err
	}
	// send black fee to oracle
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(burnFee))
	if err != nil {
		return nil, err
	}

	// mint fury
	err = m.Keeper.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(furyOut))
	if err != nil {
		return nil, err
	}
	// send backing and fury to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(backingOut, furyOut))
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeBurnBySwap,
			sdk.NewAttribute(types.AttributeKeyCoinIn, msg.BurnIn.String()),
			sdk.NewAttribute(types.AttributeKeyCoinOut, sdk.NewCoins(backingOut, furyOut).String()),
			sdk.NewAttribute(types.AttributeKeyFee, burnFee.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgBurnBySwapResponse{
		BurnFee:    burnFee,
		BackingOut: backingOut,
		FuryOut:    furyOut,
	}, nil
}

func (m msgServer) BuyBacking(c context.Context, msg *types.MsgBuyBacking) (*types.MsgBuyBackingResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	backingOut, buybackFee, err := m.Keeper.calculateBuyBackingOut(ctx, msg.FuryIn, msg.BackingOutMin.Denom)
	if err != nil {
		return nil, err
	}

	if backingOut.IsLT(msg.BackingOutMin) {
		return nil, sdkerrors.Wrapf(types.ErrOverSlippage, "backing out: %s", backingOut)
	}

	totalBacking, poolBacking, err := m.Keeper.getBacking(ctx, msg.BackingOutMin.Denom)
	if err != nil {
		return nil, err
	}

	poolBacking.Backing = poolBacking.Backing.Sub(backingOut).Sub(buybackFee)
	poolBacking.FuryBurned = poolBacking.FuryBurned.Add(msg.FuryIn)
	totalBacking.FuryBurned = totalBacking.FuryBurned.Add(msg.FuryIn)

	m.Keeper.SetPoolBacking(ctx, poolBacking)
	m.Keeper.SetTotalBacking(ctx, totalBacking)

	// take fury-in
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(msg.FuryIn))
	if err != nil {
		return nil, err
	}
	// burn fury
	err = m.Keeper.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(msg.FuryIn))
	if err != nil {
		return nil, err
	}

	// send backing to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(backingOut))
	if err != nil {
		return nil, err
	}
	// send fee to oracle
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(buybackFee))
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeBuyBacking,
			sdk.NewAttribute(types.AttributeKeyCoinIn, msg.FuryIn.String()),
			sdk.NewAttribute(types.AttributeKeyCoinOut, backingOut.String()),
			sdk.NewAttribute(types.AttributeKeyFee, buybackFee.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgBuyBackingResponse{
		BackingOut: backingOut,
		BuybackFee: buybackFee,
	}, nil
}

func (m msgServer) SellBacking(c context.Context, msg *types.MsgSellBacking) (*types.MsgSellBackingResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	furyOut, rebackFee, err := m.Keeper.calculateSellBackingOut(ctx, msg.BackingIn)
	if err != nil {
		return nil, err
	}
	furyMint := furyOut.Add(rebackFee)

	if furyOut.IsLT(msg.FuryOutMin) {
		return nil, sdkerrors.Wrapf(types.ErrOverSlippage, "fury out: %s", furyOut)
	}

	totalBacking, poolBacking, err := m.Keeper.getBacking(ctx, msg.BackingIn.Denom)
	if err != nil {
		return nil, err
	}

	poolBacking.Backing = poolBacking.Backing.Add(msg.BackingIn)

	// allow FuryBurned to be negative which means minted fury
	// here use Int.Sub() to bypass Coin.Sub() negativeness check
	poolBacking.FuryBurned.Amount = poolBacking.FuryBurned.Amount.Sub(furyMint.Amount)
	totalBacking.FuryBurned.Amount = totalBacking.FuryBurned.Amount.Sub(furyMint.Amount)

	m.Keeper.SetPoolBacking(ctx, poolBacking)
	m.Keeper.SetTotalBacking(ctx, totalBacking)

	// take backing-in
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(msg.BackingIn))
	if err != nil {
		return nil, err
	}

	// mint fury
	err = m.Keeper.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(furyMint))
	if err != nil {
		return nil, err
	}
	// send fury to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(furyOut))
	if err != nil {
		return nil, err
	}
	// send fee to oracle
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(rebackFee))
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeSellBacking,
			sdk.NewAttribute(types.AttributeKeyCoinIn, msg.BackingIn.String()),
			sdk.NewAttribute(types.AttributeKeyCoinOut, furyOut.String()),
			sdk.NewAttribute(types.AttributeKeyFee, rebackFee.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgSellBackingResponse{
		FuryOut:   furyOut,
		RebackFee: rebackFee,
	}, nil
}

func (m msgServer) MintByCollateral(c context.Context, msg *types.MsgMintByCollateral) (*types.MsgMintByCollateralResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	mintFee, totalColl, poolColl, accColl, err := m.Keeper.calculateMintByCollateral(ctx, sender, msg.CollateralDenom, msg.MintOut)
	if err != nil {
		return nil, err
	}
	mintTotal := msg.MintOut.Add(mintFee)

	m.Keeper.SetAccountCollateral(ctx, sender, accColl)
	m.Keeper.SetPoolCollateral(ctx, poolColl)
	m.Keeper.SetTotalCollateral(ctx, totalColl)

	// mint black
	err = m.Keeper.bankKeeper.MintCoins(ctx, types.ModuleName, sdk.NewCoins(mintTotal))
	if err != nil {
		return nil, err
	}
	// send black to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(msg.MintOut))
	if err != nil {
		return nil, err
	}
	// send mint fee to oracle
	if mintFee.IsPositive() {
		err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(mintFee))
		if err != nil {
			return nil, err
		}
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeMintByCollateral,
			sdk.NewAttribute(types.AttributeKeyCoinOut, msg.MintOut.String()),
			sdk.NewAttribute(types.AttributeKeyFee, mintFee.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgMintByCollateralResponse{
		MintFee: mintFee,
	}, nil
}

func (m msgServer) BurnByCollateral(c context.Context, msg *types.MsgBurnByCollateral) (*types.MsgBurnByCollateralResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)
	sender, _, err := getSenderReceiver(msg.Sender, "")
	if err != nil {
		return nil, err
	}

	collateralDenom := msg.CollateralDenom

	collateralParams, err := m.Keeper.getAvailableCollateralParams(ctx, collateralDenom)
	if err != nil {
		return nil, err
	}

	totalColl, poolColl, accColl, err := m.Keeper.getCollateral(ctx, sender, collateralDenom)
	if err != nil {
		return nil, err
	}

	settleInterestFee(ctx, &accColl, &poolColl, &totalColl, *collateralParams.InterestFee)

	// compute burn-in, repay interest first
	if !accColl.MerDebt.IsPositive() {
		return nil, sdkerrors.Wrapf(types.ErrAccountNoDebt, "account has no debt for %s collateral", collateralDenom)
	}
	repayIn := sdk.NewCoin(msg.RepayInMax.Denom, sdk.MinInt(accColl.MerDebt.Amount, msg.RepayInMax.Amount))
	repayInterest := sdk.NewCoin(msg.RepayInMax.Denom, sdk.MinInt(accColl.LastInterest.Amount, repayIn.Amount))
	burn := repayIn.Sub(repayInterest)

	// update debt
	accColl.LastInterest = accColl.LastInterest.Sub(repayInterest)
	accColl.MerDebt = accColl.MerDebt.Sub(repayIn)
	poolColl.MerDebt = poolColl.MerDebt.Sub(repayIn)
	totalColl.MerDebt = totalColl.MerDebt.Sub(repayIn)

	// eventually update collateral
	m.Keeper.SetAccountCollateral(ctx, sender, accColl)
	m.Keeper.SetPoolCollateral(ctx, poolColl)
	m.Keeper.SetTotalCollateral(ctx, totalColl)

	// take black
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(repayIn))
	if err != nil {
		return nil, err
	}
	// burn black
	if burn.IsPositive() {
		err = m.Keeper.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(burn))
		if err != nil {
			return nil, err
		}
	}
	// send fee to oracle
	if repayInterest.IsPositive() {
		err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(repayInterest))
		if err != nil {
			return nil, err
		}
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeBurnByCollateral,
			sdk.NewAttribute(types.AttributeKeyCoinIn, repayIn.String()),
			sdk.NewAttribute(types.AttributeKeyFee, repayInterest.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgBurnByCollateralResponse{
		RepayIn: repayIn,
	}, nil
}

func (m msgServer) DepositCollateral(c context.Context, msg *types.MsgDepositCollateral) (*types.MsgDepositCollateralResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	collateralDenom := msg.CollateralIn.Denom

	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	collateralParams, err := m.Keeper.getAvailableCollateralParams(ctx, collateralDenom)
	if err != nil {
		return nil, err
	}

	totalColl, poolColl, accColl, err := m.Keeper.getCollateral(ctx, receiver, collateralDenom, true)
	if err != nil {
		return nil, err
	}

	settleInterestFee(ctx, &accColl, &poolColl, &totalColl, *collateralParams.InterestFee)

	accColl.Collateral = accColl.Collateral.Add(msg.CollateralIn)
	poolColl.Collateral = poolColl.Collateral.Add(msg.CollateralIn)
	accColl.FuryCollateralized = accColl.FuryCollateralized.Add(msg.FuryIn)
	poolColl.FuryCollateralized = poolColl.FuryCollateralized.Add(msg.FuryIn)
	totalColl.FuryCollateralized = totalColl.FuryCollateralized.Add(msg.FuryIn)

	if collateralParams.MaxCollateral != nil && poolColl.Collateral.Amount.GT(*collateralParams.MaxCollateral) {
		return nil, sdkerrors.Wrap(types.ErrCollateralCeiling, "")
	}

	m.Keeper.SetAccountCollateral(ctx, receiver, accColl)
	m.Keeper.SetPoolCollateral(ctx, poolColl)
	m.Keeper.SetTotalCollateral(ctx, totalColl)

	// take collateral from sender
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(msg.CollateralIn, msg.FuryIn))
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeDepositCollateral,
			sdk.NewAttribute(types.AttributeKeyCoinIn, sdk.NewCoins(msg.CollateralIn, msg.FuryIn).String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgDepositCollateralResponse{}, nil
}

func (m msgServer) RedeemCollateral(c context.Context, msg *types.MsgRedeemCollateral) (*types.MsgRedeemCollateralResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	collateralDenom := msg.CollateralOut.Denom

	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}

	collateralParams, err := m.Keeper.getAvailableCollateralParams(ctx, collateralDenom)
	if err != nil {
		return nil, err
	}

	totalColl, poolColl, accColl, err := m.Keeper.getCollateral(ctx, sender, collateralDenom)
	if err != nil {
		return nil, err
	}

	settleInterestFee(ctx, &accColl, &poolColl, &totalColl, *collateralParams.InterestFee)

	// update collateral
	accColl.Collateral = accColl.Collateral.Sub(msg.CollateralOut)
	poolColl.Collateral = poolColl.Collateral.Sub(msg.CollateralOut)
	accColl.FuryCollateralized = accColl.FuryCollateralized.Sub(msg.FuryOut)
	poolColl.FuryCollateralized = poolColl.FuryCollateralized.Sub(msg.FuryOut)
	totalColl.FuryCollateralized = totalColl.FuryCollateralized.Sub(msg.FuryOut)

	_, maxDebtInUSD, err := m.Keeper.maxLoanToValueForAccount(ctx, &accColl, &collateralParams)
	if err != nil {
		return nil, err
	}

	if accColl.MerDebt.Amount.ToDec().Mul(blackfury.MicroFUSDTarget).GT(maxDebtInUSD) {
		return nil, sdkerrors.Wrapf(types.ErrAccountInsufficientCollateral, "account collateral insufficient: %s", collateralDenom)
	}

	// eventually persist collateral
	m.Keeper.SetAccountCollateral(ctx, sender, accColl)
	m.Keeper.SetPoolCollateral(ctx, poolColl)
	m.Keeper.SetTotalCollateral(ctx, totalColl)

	// send collateral to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(msg.CollateralOut, msg.FuryOut))
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeRedeemCollateral,
			sdk.NewAttribute(types.AttributeKeyCoinOut, sdk.NewCoins(msg.CollateralOut, msg.FuryOut).String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgRedeemCollateralResponse{}, nil
}

func (m msgServer) LiquidateCollateral(c context.Context, msg *types.MsgLiquidateCollateral) (*types.MsgLiquidateCollateralResponse, error) {
	ctx := sdk.UnwrapSDKContext(c)

	collateralDenom := msg.Collateral.Denom

	sender, receiver, err := getSenderReceiver(msg.Sender, msg.To)
	if err != nil {
		return nil, err
	}
	debtor, err := sdk.AccAddressFromBech32(msg.Debtor)
	if err != nil {
		return nil, err
	}

	collateralParams, err := m.Keeper.getAvailableCollateralParams(ctx, collateralDenom)
	if err != nil {
		return nil, err
	}

	totalColl, poolColl, accColl, err := m.Keeper.getCollateral(ctx, debtor, collateralDenom)
	if err != nil {
		return nil, err
	}

	settleInterestFee(ctx, &accColl, &poolColl, &totalColl, *collateralParams.InterestFee)

	// get prices in usd
	collateralPrice, err := m.Keeper.oracleKeeper.GetExchangeRate(ctx, collateralDenom)
	if err != nil {
		return nil, err
	}

	// check whether undercollateralized
	liquidationValue := accColl.Collateral.Amount.ToDec().Mul(collateralPrice).Mul(*collateralParams.LiquidationThreshold)
	if accColl.MerDebt.Amount.ToDec().Mul(blackfury.MicroFUSDTarget).LT(liquidationValue) {
		return nil, sdkerrors.Wrap(types.ErrNotUndercollateralized, "")
	}

	if msg.Collateral.Amount.GT(accColl.Collateral.Amount) {
		return nil, sdkerrors.Wrap(types.ErrCollateralCoinInsufficient, "")
	}

	liquidationFee := msg.Collateral.Amount.ToDec().Mul(*collateralParams.LiquidationFee)
	commissionFee := sdk.NewCoin(collateralDenom, liquidationFee.Mul(m.Keeper.LiquidationCommissionFee(ctx)).TruncateInt())
	collateralOut := msg.Collateral.Sub(commissionFee)
	repayIn := sdk.NewCoin(blackfury.MicroFUSDDenom, msg.Collateral.Amount.ToDec().Sub(liquidationFee).Mul(collateralPrice).Quo(blackfury.MicroFUSDTarget).TruncateInt())

	if msg.RepayInMax.IsLT(repayIn) {
		return nil, sdkerrors.Wrap(types.ErrMerSlippage, "")
	}

	// repay for debtor as much as possible, and repay interest first
	repayDebt := sdk.NewCoin(blackfury.MicroFUSDDenom, sdk.MinInt(accColl.MerDebt.Amount, repayIn.Amount))
	merRefund := repayIn.Sub(repayDebt)

	repayInterest := sdk.NewCoin(blackfury.MicroFUSDDenom, sdk.MinInt(accColl.LastInterest.Amount, repayDebt.Amount))
	accColl.LastInterest = accColl.LastInterest.Sub(repayInterest)

	accColl.MerDebt = accColl.MerDebt.Sub(repayDebt)
	poolColl.MerDebt = poolColl.MerDebt.Sub(repayDebt)
	totalColl.MerDebt = totalColl.MerDebt.Sub(repayDebt)
	accColl.Collateral = accColl.Collateral.Sub(msg.Collateral)
	poolColl.Collateral = poolColl.Collateral.Sub(msg.Collateral)

	// eventually persist collateral
	m.Keeper.SetAccountCollateral(ctx, debtor, accColl)
	m.Keeper.SetPoolCollateral(ctx, poolColl)
	m.Keeper.SetTotalCollateral(ctx, totalColl)

	// take black from sender
	err = m.Keeper.bankKeeper.SendCoinsFromAccountToModule(ctx, sender, types.ModuleName, sdk.NewCoins(repayIn))
	if err != nil {
		return nil, err
	}
	// burn black debt
	err = m.Keeper.bankKeeper.BurnCoins(ctx, types.ModuleName, sdk.NewCoins(repayDebt))
	if err != nil {
		return nil, err
	}
	// send excess black to debtor
	if merRefund.IsPositive() {
		err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, debtor, sdk.NewCoins(merRefund))
		if err != nil {
			return nil, err
		}
	}

	// send collateral to receiver
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, receiver, sdk.NewCoins(collateralOut))
	if err != nil {
		return nil, err
	}
	// send liquidation commission fee to oracle
	err = m.Keeper.bankKeeper.SendCoinsFromModuleToModule(ctx, types.ModuleName, oracletypes.ModuleName, sdk.NewCoins(commissionFee))
	if err != nil {
		return nil, err
	}

	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(types.EventTypeLiquidateCollateral,
			sdk.NewAttribute(types.AttributeKeyCoinIn, repayIn.String()),
			sdk.NewAttribute(types.AttributeKeyCoinOut, collateralOut.String()),
			sdk.NewAttribute(types.AttributeKeyFee, commissionFee.String()),
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
		),
	})

	return &types.MsgLiquidateCollateralResponse{
		RepayIn:       repayIn,
		CollateralOut: collateralOut,
	}, nil
}

func (k Keeper) getBacking(ctx sdk.Context, denom string) (total types.TotalBacking, pool types.PoolBacking, err error) {
	total, found := k.GetTotalBacking(ctx)
	if !found {
		err = sdkerrors.Wrapf(types.ErrBackingCoinNotFound, "backing coin denomination not found: %s", denom)
		return
	}
	pool, found = k.GetPoolBacking(ctx, denom)
	if !found {
		err = sdkerrors.Wrapf(types.ErrBackingCoinNotFound, "backing coin denomination not found: %s", denom)
		return
	}
	return
}

func (k Keeper) getCollateral(ctx sdk.Context, account sdk.AccAddress, denom string, allowNewAccount ...bool) (total types.TotalCollateral, pool types.PoolCollateral, acc types.AccountCollateral, err error) {
	total, found := k.GetTotalCollateral(ctx)
	if !found {
		err = sdkerrors.Wrapf(types.ErrCollateralCoinNotFound, "collateral coin denomination not found: %s", denom)
		return
	}
	pool, found = k.GetPoolCollateral(ctx, denom)
	if !found {
		err = sdkerrors.Wrapf(types.ErrCollateralCoinNotFound, "collateral coin denomination not found: %s", denom)
		return
	}
	acc, found = k.GetAccountCollateral(ctx, account, denom)
	if !found {
		if len(allowNewAccount) > 0 && allowNewAccount[0] {
			acc = types.AccountCollateral{
				Account:             account.String(),
				Collateral:          sdk.NewCoin(denom, sdk.ZeroInt()),
				MerDebt:             sdk.NewCoin(blackfury.MicroFUSDDenom, sdk.ZeroInt()),
				FuryCollateralized:  sdk.NewCoin(blackfury.AttoFuryDenom, sdk.ZeroInt()),
				LastInterest:        sdk.NewCoin(blackfury.MicroFUSDDenom, sdk.ZeroInt()),
				LastSettlementBlock: ctx.BlockHeight(),
			}
		} else {
			err = sdkerrors.Wrapf(types.ErrAccountNoCollateral, "account has no collateral: %s", denom)
			return
		}
	}
	return
}

func settleInterestFee(ctx sdk.Context, acc *types.AccountCollateral, pool *types.PoolCollateral, total *types.TotalCollateral, apr sdk.Dec) {
	period := ctx.BlockHeight() - acc.LastSettlementBlock
	if period == 0 {
		// short circuit
		return
	}

	// principal debt, excluding interest debt
	principalDebt := acc.MerDebt.Sub(acc.LastInterest)
	interestOfPeriod := principalDebt.Amount.ToDec().Mul(apr).MulInt64(period).QuoInt64(int64(blackfury.BlocksPerYear)).RoundInt()

	// update remaining interest accumulation
	acc.LastInterest = acc.LastInterest.AddAmount(interestOfPeriod)
	// update debt
	acc.MerDebt = acc.MerDebt.AddAmount(interestOfPeriod)
	pool.MerDebt = pool.MerDebt.AddAmount(interestOfPeriod)
	total.MerDebt = total.MerDebt.AddAmount(interestOfPeriod)
	// update settlement block
	acc.LastSettlementBlock = ctx.BlockHeight()
}

func (k Keeper) maxLoanToValueForAccount(ctx sdk.Context, acc *types.AccountCollateral, collateralParams *types.CollateralRiskParams) (availableLTV, maxDebtInUSD sdk.Dec, err error) {
	collateralPrice, err := k.oracleKeeper.GetExchangeRate(ctx, acc.Collateral.Denom)
	if err != nil {
		return
	}
	furyPrice, err := k.oracleKeeper.GetExchangeRate(ctx, blackfury.AttoFuryDenom)
	if err != nil {
		return
	}

	collateralInUSD := acc.Collateral.Amount.ToDec().Mul(collateralPrice)
	collateralizedFuryInUSD := acc.FuryCollateralized.Amount.ToDec().Mul(furyPrice)
	if !collateralInUSD.IsPositive() {
		return sdk.ZeroDec(), sdk.ZeroDec(), nil
	}

	catalyticRatio := sdk.MinDec(collateralizedFuryInUSD.Quo(collateralInUSD), *collateralParams.CatalyticFuryRatio)
	// actualCatalyticRatio / maxCatalyticRatio = (availableLTV - basicLTV) / (maxLTV - basicLTV)
	availableLTV = collateralParams.LoanToValue.Sub(*collateralParams.BasicLoanToValue).Mul(catalyticRatio).Quo(*collateralParams.CatalyticFuryRatio).Add(*collateralParams.BasicLoanToValue)
	maxDebtInUSD = collateralInUSD.Mul(availableLTV)

	return
}

func getSenderReceiver(senderStr, toStr string) (sender sdk.AccAddress, receiver sdk.AccAddress, err error) {
	sender, err = sdk.AccAddressFromBech32(senderStr)
	if err != nil {
		return
	}
	receiver = sender
	if len(toStr) > 0 {
		// user specifies receiver
		receiver, err = sdk.AccAddressFromBech32(toStr)
		if err != nil {
			return
		}
	}
	return
}
