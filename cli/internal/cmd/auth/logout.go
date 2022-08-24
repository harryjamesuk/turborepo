package auth

import (
	"github.com/vercel/turborepo/cli/cmdutil"
	"github.com/vercel/turborepo/cli/internal/util"

	"github.com/spf13/cobra"
)

// LogoutCmd returns the Cobra logout command
func LogoutCmd(helper *cmdutil.Helper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout of your Vercel account",
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := helper.GetCmdBase(cmd.Flags())
			if err != nil {
				return err
			}
			if err := base.UserConfig.Delete(); err != nil {
				return base.LogError("could not logout. Something went wrong: %w", err)
			}

			base.UI.Info(util.Sprintf("${GREY}>>> Logged out${RESET}"))

			return nil
		},
	}

	return cmd
}
