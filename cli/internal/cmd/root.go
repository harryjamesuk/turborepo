package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vercel/turborepo/cli/cmdutil"
	"github.com/vercel/turborepo/cli/internal/cmd/auth"
	"github.com/vercel/turborepo/cli/internal/cmd/info"
	"github.com/vercel/turborepo/cli/internal/login"
)

func GetCmd(turboVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:              "turbo",
		Short:            "Turbo charge your monorepo",
		TraverseChildren: true,
	}
	flags := cmd.PersistentFlags()
	helper := cmdutil.NewHelper(turboVersion)
	helper.AddFlags(flags)
	cmd.AddCommand(login.NewLinkCommand(helper))
	cmd.AddCommand(login.NewLoginCommand(helper))
	cmd.AddCommand(auth.LogoutCmd(helper))
	cmd.AddCommand(info.BinCmd(helper))
	return cmd
}
