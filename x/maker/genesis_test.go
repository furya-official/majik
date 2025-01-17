package maker_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/suite"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"

	"github.com/furya-official/blackfury/app"
	"github.com/furya-official/blackfury/x/maker"
	"github.com/furya-official/blackfury/x/maker/types"
)

type GenesisTestSuite struct {
	suite.Suite
	ctx sdk.Context
	app *app.Blackfury
}

func (suite *GenesisTestSuite) SetupTest() {
	suite.app = app.Setup(false)
	suite.ctx = suite.app.BaseApp.NewContext(false, tmproto.Header{})
}

func TestGenesisTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisTestSuite))
}

func (suite *GenesisTestSuite) TestMakerInitGenesis() {
	app := suite.app
	makerKeeper := app.MakerKeeper

	suite.Require().NotPanics(func() {
		maker.InitGenesis(suite.ctx, makerKeeper, *types.DefaultGenesis())
	})

	backingRatio := makerKeeper.GetBackingRatio(suite.ctx)
	params := makerKeeper.GetParams(suite.ctx)

	suite.Require().Equal(sdk.OneDec(), backingRatio)
	suite.Require().Equal(types.DefaultParams(), params)
}

func (suite *GenesisTestSuite) TestMakerExportGenesis() {
	app := suite.app
	makerKeeper := app.MakerKeeper

	suite.Require().NotPanics(func() {
		maker.InitGenesis(suite.ctx, makerKeeper, *types.DefaultGenesis())
	})

	genesisExported := maker.ExportGenesis(suite.ctx, makerKeeper)
	suite.Require().Equal(sdk.OneDec(), genesisExported.BackingRatio)
	suite.Require().Equal(types.DefaultParams(), genesisExported.Params)
}
