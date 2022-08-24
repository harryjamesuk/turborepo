package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vercel/turborepo/cli/cmdutil"
	"github.com/vercel/turborepo/cli/internal/cmd/auth"
	"github.com/vercel/turborepo/cli/internal/cmd/info"
	"github.com/vercel/turborepo/cli/internal/daemon"
	"github.com/vercel/turborepo/cli/internal/login"
	"github.com/vercel/turborepo/cli/internal/signals"
)

func GetCmd(turboVersion string, signalWatcher *signals.Watcher) *cobra.Command {
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
	cmd.AddCommand(auth.UnlinkCmd(helper))
	cmd.AddCommand(info.BinCmd(helper))
	cmd.AddCommand(daemon.GetCmd(helper, signalWatcher))
	return cmd
}
