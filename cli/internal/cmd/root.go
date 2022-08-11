package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vercel/turborepo/cli/internal/client"
	"github.com/vercel/turborepo/cli/internal/config"
	"github.com/vercel/turborepo/cli/internal/fs"
)

func GetCmd() *cobra.Command {
	cfg := &config.Config{}
	repoRoot, err := fs.GetCwd()
	if err != nil {
		// If we cannot get the cwd, bail early
		panic(err)
	}
	cmd := &cobra.Command{
		Use:              "turbo",
		Short:            "Turbo charge your monorepo",
		TraverseChildren: true,
	}
	flags := cmd.PersistentFlags()
	fs.AbsolutePathVar(flags, &cfg.Cwd, "cwd", repoRoot, "Specify the directory to run turbo in", repoRoot.ToString())
	client.AddFlags(&cfg.ClientOpts, flags)
	config.AddUserConfigFlags(&cfg.UserConfig, flags)
	flags.CountVarP(&cfg.Verbosity, "verbosity", "v", "verbosity")
	return cmd
}
