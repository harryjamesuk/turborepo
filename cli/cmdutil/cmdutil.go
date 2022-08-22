package cmdutil

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/spf13/pflag"
	"github.com/vercel/turborepo/cli/internal/client"
	"github.com/vercel/turborepo/cli/internal/config"
	"github.com/vercel/turborepo/cli/internal/fs"
	"github.com/vercel/turborepo/cli/internal/ui"
)

const (
	// _envLogLevel is the environment log level
	_envLogLevel = "TURBO_LOG_LEVEL"
)

type Helper struct {
	// UserConfig *config.TurborepoConfig
	// //UI           cli.Ui              // Should be a function
	// //Logger       func() hclog.Logger // Should be a function
	// //RepoRoot     fs.AbsolutePath
	TurboVersion string
	// ApiClient    *client.ApiClient // TODO: maybe should be a function?

	// for UI
	forceColor bool
	noColor    bool
	// for logging
	verbosity int

	rawRepoRoot string

	clientOpts client.Opts

	remoteConfig config.RemoteConfig
}

func (h *Helper) getUI(flags *pflag.FlagSet) cli.Ui {
	colorMode := ui.GetColorModeFromEnv()
	if flags.Changed("no-color") && h.noColor {
		colorMode = ui.ColorModeSuppressed
	}
	if flags.Changed("color") && h.forceColor {
		colorMode = ui.ColorModeForced
	}
	return ui.BuildColoredUi(colorMode)
}

func (h *Helper) getLogger() (hclog.Logger, error) {
	var level hclog.Level
	switch h.verbosity {
	case 0:
		if v := os.Getenv(_envLogLevel); v != "" {
			level = hclog.LevelFromString(v)
			if level == hclog.NoLevel {
				return nil, fmt.Errorf("%s value %q is not a valid log level", _envLogLevel, v)
			}
		} else {
			level = hclog.NoLevel
		}
	case 1:
		level = hclog.Info
	case 2:
		level = hclog.Debug
	case 3:
		level = hclog.Trace
	default:
		level = hclog.Trace
	}
	// Default output is nowhere unless we enable logging.
	output := ioutil.Discard
	color := hclog.ColorOff
	if level != hclog.NoLevel {
		output = os.Stderr
		color = hclog.AutoColor
	}

	return hclog.New(&hclog.LoggerOptions{
		Name:   "turbo",
		Level:  level,
		Color:  color,
		Output: output,
	}), nil
}

// NewClient returns a new ApiClient instance using the values from
// this Config instance.
func (h *Helper) newClient(logger hclog.Logger) *client.ApiClient {
	apiClient := client.NewClient(
		h.UserConfig.ToRemoteConfig(),
		logger,
		h.TurboVersion,
		h.clientOpts,
	)
	return apiClient
}

func (h *Helper) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&h.forceColor, "color", false, "Force color usage in the terminal")
	flags.BoolVar(&h.noColor, "no-color", false, "Suppress color usage in the terminal")
	flags.CountVarP(&h.verbosity, "verbosity", "v", "verbosity")
	flags.StringVar(&h.rawRepoRoot, "cwd", "", "The directory in which to run turbo")
	client.AddFlags(&h.clientOpts, flags)
	h.remoteConfig.AddFlags(flags)
}

func NewHelper(turboVersion string) *Helper {
	return &Helper{
		TurboVersion: turboVersion,
		// UserConfig:   &config.UserConfig,
		// TurboVersion: config.TurboVersion,
		// ApiClient:    config.NewClient(),
	}
}

func (h *Helper) GetCmdBase(flags *pflag.FlagSet) (*CmdBase, error) {
	ui := h.getUI(flags)
	logger, err := h.getLogger()
	if err != nil {
		return nil, err
	}
	cwd, err := fs.GetCwd()
	if err != nil {
		return nil, err
	}
	repoRoot := fs.ResolveUnknownPath(cwd, h.rawRepoRoot)
	repoRoot, err = repoRoot.EvalSymlinks()
	if err != nil {
		return nil, err
	}
	return &CmdBase{
		UI:        ui,
		Logger:    logger,
		RepoRoot:  repoRoot,
		ApiClient: h.newClient(logger),
	}, nil
}

type CmdBase struct {
	UI        cli.Ui
	Logger    hclog.Logger
	RepoRoot  fs.AbsolutePath
	ApiClient *client.ApiClient
}

// LogError prints an error to the UI and returns a BasicError
func (b *CmdBase) LogError(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	b.Logger.Error("error", err)
	b.UI.Error(fmt.Sprintf("%s%s", ui.ERROR_PREFIX, color.RedString(" %v", err)))
	return err
}
