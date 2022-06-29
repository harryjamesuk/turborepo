package cmdutil

import (
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/vercel/turborepo/cli/internal/config"
	"github.com/vercel/turborepo/cli/internal/fs"
)

type Helper struct {
	//Config   *config.Config
	UserConfig *config.TurborepoConfig
	UI         cli.Ui
	Logger     hclog.Logger
	RepoRoot   fs.AbsolutePath
}
