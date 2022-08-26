package cmdutil

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/fatih/color"
	"github.com/hashicorp/go-hclog"
	"github.com/mattn/go-isatty"
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

// isCI returns true if running in a CI/CD environment
func isCI() bool {
	return !isatty.IsTerminal(os.Stdout.Fd()) || os.Getenv("CI") != ""
}

// Helper is a struct used to hold configuration values passed via flag, env vars,
// config files, etc. It is not intended for direct use by turbo commands, it drives
// the creation of CmdBase, which is then used by the commands themselves.
type Helper struct {
	turboVersion string

	// for UI
	forceColor bool
	noColor    bool
	// for logging
	verbosity int

	rawRepoRoot string

	clientOpts client.Opts

	// UserConfigPath is the path to where we expect to find
	// a user-specific config file, if one is present. Public
	// to allow overrides in tests
	UserConfigPath fs.AbsolutePath
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

func (h *Helper) AddFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&h.forceColor, "color", false, "Force color usage in the terminal")
	flags.BoolVar(&h.noColor, "no-color", false, "Suppress color usage in the terminal")
	flags.CountVarP(&h.verbosity, "verbosity", "v", "verbosity")
	flags.StringVar(&h.rawRepoRoot, "cwd", "", "The directory in which to run turbo")
	client.AddFlags(&h.clientOpts, flags)
	config.AddRepoConfigFlags(flags)
	config.AddUserConfigFlags(flags)
}

func NewHelper(turboVersion string) *Helper {
	return &Helper{
		turboVersion:   turboVersion,
		UserConfigPath: config.DefaultUserConfigPath(),
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
	repoConfig, err := config.ReadRepoConfigFile(config.GetRepoConfigPath(repoRoot), flags)
	if err != nil {
		return nil, err
	}
	userConfig, err := config.LoadUserConfigFile(h.UserConfigPath, flags)
	if err != nil {
		return nil, err
	}
	remoteConfig := repoConfig.GetRemoteConfig(userConfig.Token())
	if remoteConfig.Token == "" && isCI() {
		vercelArtifactsToken := os.Getenv("VERCEL_ARTIFACTS_TOKEN")
		vercelArtifactsOwner := os.Getenv("VERCEL_ARTIFACTS_OWNER")
		if vercelArtifactsToken != "" {
			remoteConfig.Token = vercelArtifactsToken
		}
		if vercelArtifactsOwner != "" {
			remoteConfig.TeamID = vercelArtifactsOwner
		}
	}
	apiClient := client.NewClient(
		remoteConfig,
		logger,
		h.turboVersion,
		h.clientOpts,
	)
	return &CmdBase{
		UI:           ui,
		Logger:       logger,
		RepoRoot:     repoRoot,
		APIClient:    apiClient,
		RepoConfig:   repoConfig,
		UserConfig:   userConfig,
		RemoteConfig: remoteConfig,
		TurboVersion: h.turboVersion,
	}, nil
}

// CmdBase encompasses everything common to all turbo commands.
type CmdBase struct {
	UI           cli.Ui
	Logger       hclog.Logger
	RepoRoot     fs.AbsolutePath
	APIClient    *client.ApiClient
	RepoConfig   *config.RepoConfig
	UserConfig   *config.UserConfig
	RemoteConfig client.RemoteConfig
	TurboVersion string
}

// LogError prints an error to the UI and returns a BasicError
func (b *CmdBase) LogError(format string, args ...interface{}) error {
	err := fmt.Errorf(format, args...)
	b.Logger.Error("error", err)
	b.UI.Error(fmt.Sprintf("%s%s", ui.ERROR_PREFIX, color.RedString(" %v", err)))
	return err
}
