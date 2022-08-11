package cmdutil

import (
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/vercel/turborepo/cli/internal/client"
	"github.com/vercel/turborepo/cli/internal/config"
	"github.com/vercel/turborepo/cli/internal/fs"
)

const (
	// _envLogLevel is the environment log level
	_envLogLevel = "TURBO_LOG_LEVEL"
)

type Helper struct {
	UserConfig   *config.TurborepoConfig
	UI           cli.Ui              // Should be a function
	Logger       func() hclog.Logger // Should be a function
	RepoRoot     fs.AbsolutePath
	TurboVersion string
	ApiClient    *client.ApiClient // TODO: maybe should be a function?
}

func NewHelper(config *config.Config, ui cli.Ui) *Helper {
	return &Helper{
		UserConfig: &config.UserConfig,
		UI:         ui,
		Logger: func() {
			var level hclog.Level
			switch config.Verbosity {
			case 0:
				if v := os.Getenv(_envLogLevel); v != "" {
					level = hclog.LevelFromString(v)
					if level == hclog.NoLevel {
						return nil, fmt.Errorf("%s value %q is not a valid log level", _envLogLevel, v)
					}
				}
			}
		},
		RepoRoot:     config.Cwd,
		TurboVersion: config.TurboVersion,
		ApiClient:    config.NewClient(),
	}
}
