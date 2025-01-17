package keeper_test

import (
	abci "github.com/tendermint/tendermint/abci/types"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/furya-official/blackfury/app"
	"github.com/furya-official/blackfury/x/maker/types"
	"github.com/stretchr/testify/suite"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	tmversion "github.com/tendermint/tendermint/proto/tendermint/version"
	"github.com/tendermint/tendermint/version"
	"github.com/tharsis/ethermint/crypto/ethsecp256k1"
	"github.com/tharsis/ethermint/tests"
)

type KeeperTestSuite struct {
	suite.Suite
	ctx         sdk.Context
	app         *app.Blackfury
	queryClient types.QueryClient
	accAddress  sdk.AccAddress
	signer      keyring.Signer
	consAddress sdk.ConsAddress
	bcDenom     string
}

var s *KeeperTestSuite

func TestKeeperTestSuite(t *testing.T) {
	s = new(KeeperTestSuite)
	suite.Run(t, s)
}

func (suite *KeeperTestSuite) SetupTest() {
	// account key
	priv, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	suite.accAddress = sdk.AccAddress(priv.PubKey().Address())
	suite.signer = tests.NewSigner(priv)

	// consensus key
	privCons, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)
	suite.consAddress = sdk.ConsAddress(privCons.PubKey().Address())

	// init app
	suite.app = app.Setup(false)

	// setup context
	suite.ctx = suite.app.BaseApp.NewContext(false, tmproto.Header{
		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		ChainID:         "blackfury_5000-101",
		Height:          1,
		Time:            time.Now().UTC(),
		ProposerAddress: suite.consAddress.Bytes(),
	})

	// setup query helpers
	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, suite.app.MakerKeeper)
	suite.queryClient = types.NewQueryClient(queryHelper)

	// set backing and collateral denom
	suite.bcDenom = "uDAI"
}

func (suite *KeeperTestSuite) Commit() {
	suite.CommitAfter(time.Nanosecond)
}

func (suite *KeeperTestSuite) CommitAfter(t time.Duration) {
	_ = suite.app.Commit()
	header := suite.ctx.BlockHeader()
	header.Height += 1
	header.Time = header.Time.Add(t)
	suite.app.BeginBlock(abci.RequestBeginBlock{
		Header: header,
	})

	// update ctx and query helper
	suite.ctx = suite.app.BaseApp.NewContext(false, header)
	queryHelper := baseapp.NewQueryServerTestHelper(suite.ctx, suite.app.InterfaceRegistry())
	types.RegisterQueryServer(queryHelper, suite.app.MakerKeeper)
}
