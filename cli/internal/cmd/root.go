package cmd

import (
	"github.com/spf13/cobra"
	"github.com/vercel/turborepo/cli/internal/config"
)

func GetCmd() *cobra.Command {
	config := &config.Config{}
	cmd := &cobra.Command{
		Use:              "turbo",
		Short:            "Turbo charge your monorepo",
		TraverseChildren: true,
	}

	return cmd
}
