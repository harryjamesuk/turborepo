package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vercel/turborepo/cli/cmdutil"
	"github.com/vercel/turborepo/cli/internal/cmd/info"
	"github.com/vercel/turborepo/cli/internal/login"
)

func GetCmd(turboVersion string) *cobra.Command {
	// cfg := &config.Config{
	// 	TurboVersion: turboVersion,
	// }
	cmd := &cobra.Command{
		Use:              "turbo",
		Short:            "Turbo charge your monorepo",
		TraverseChildren: true,
	}
	flags := cmd.PersistentFlags()
	//config.AddUserConfigFlags(&cfg.UserConfig, flags)
	helper := cmdutil.NewHelper(turboVersion)
	helper.AddFlags(flags)
	cmd.AddCommand(login.NewLinkCommand(helper))
	cmd.AddCommand(info.BinCmd(helper))
	return cmd
}
