package client

import (
	govclient "github.com/cosmos/cosmos-sdk/x/gov/client"
	"github.com/furya-official/blackfury/x/maker/client/cli"
	"github.com/furya-official/blackfury/x/maker/client/rest"
)

var (
	RegisterBackingProposalHandler    = govclient.NewProposalHandler(cli.NewRegisterBackingProposalCmd, rest.RegisterBackingProposalRESTHandler)
	RegisterCollateralProposalHandler = govclient.NewProposalHandler(cli.NewRegisterCollateralProposalCmd, rest.RegisterCollateralProposalRESTHandler)
	SetBackingProposalHandler         = govclient.NewProposalHandler(cli.NewSetBackingProposalCmd, rest.SetBackingProposalRESTHandler)
	SetCollateralProposalHandler      = govclient.NewProposalHandler(cli.NewSetCollateralProposalCmd, rest.SetCollateralProposalRESTHandler)
	BatchSetBackingProposalHandler    = govclient.NewProposalHandler(cli.NewBatchSetBackingProposalCmd, rest.BatchSetBackingProposalRESTHandler)
	BatchSetCollateralProposalHandler = govclient.NewProposalHandler(cli.NewBatchSetCollateralProposalCmd, rest.BatchSetCollateralProposalRESTHandler)
)
