package oracle_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/staking"
	"github.com/furya-official/blackfury/x/oracle"
	"github.com/furya-official/blackfury/x/oracle/keeper"
	"github.com/stretchr/testify/require"
)

const (
	denom1    = "denom1"
	denom2    = "denom2"
	microUnit = int64(1e6)
)

var (
	stakingAmt = sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)

	randomExchangeRate        = sdk.NewDec(1700)
	anotherRandomExchangeRate = sdk.NewDecWithPrec(4882, 2) // swap rate

	denom1ExchangeRateStr = denom1 + ":" + randomExchangeRate.String()
)

func setupWithSmallVotingPower(t *testing.T) (keeper.TestInput, sdk.Handler) {
	input := keeper.CreateTestInput(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	params.SlashWindow = 100
	params.RewardDistributionWindow = 100
	input.OracleKeeper.SetParams(input.Ctx, params)
	h := oracle.NewHandler(input.OracleKeeper)

	sh := staking.NewHandler(input.StakingKeeper)
	_, err := sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[0], keeper.ValPubKeys[0], sdk.TokensFromConsensusPower(1, sdk.DefaultPowerReduction)))
	require.NoError(t, err)

	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	return input, h
}

func setup(t *testing.T) (keeper.TestInput, sdk.Handler) {
	input := keeper.CreateTestInput(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	params.SlashWindow = 100
	params.RewardDistributionWindow = 100
	input.OracleKeeper.SetParams(input.Ctx, params)
	h := oracle.NewHandler(input.OracleKeeper)

	sh := staking.NewHandler(input.StakingKeeper)

	// Validator created

	_, err := sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[0], keeper.ValPubKeys[0], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[1], keeper.ValPubKeys[1], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[2], keeper.ValPubKeys[2], stakingAmt))
	require.NoError(t, err)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	input.OracleKeeper.SetVoteTarget(input.Ctx, denom1)
	input.OracleKeeper.SetVoteTarget(input.Ctx, denom2)

	return input, h
}

func setupVal5(t *testing.T) (keeper.TestInput, sdk.Handler) {
	input := keeper.CreateTestInput(t)
	params := input.OracleKeeper.GetParams(input.Ctx)
	params.VotePeriod = 1
	params.SlashWindow = 100
	params.RewardDistributionWindow = 100
	input.OracleKeeper.SetParams(input.Ctx, params)
	h := oracle.NewHandler(input.OracleKeeper)

	sh := staking.NewHandler(input.StakingKeeper)

	// Validator created
	_, err := sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[0], keeper.ValPubKeys[0], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[1], keeper.ValPubKeys[1], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[2], keeper.ValPubKeys[2], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[3], keeper.ValPubKeys[3], stakingAmt))
	require.NoError(t, err)
	_, err = sh(input.Ctx, keeper.NewTestMsgCreateValidator(keeper.ValAddrs[4], keeper.ValPubKeys[4], stakingAmt))
	require.NoError(t, err)
	staking.EndBlocker(input.Ctx, input.StakingKeeper)

	input.OracleKeeper.SetVoteTarget(input.Ctx, denom2)

	return input, h
}
