package cli

import (
	"fmt"

	lp "github.com/gboutry/sunbeam-watchtower/pkg/launchpad/v1"
	"github.com/spf13/cobra"
)

func newAuthCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	cmd.AddCommand(newAuthLoginCmd(opts))
	cmd.AddCommand(newAuthStatusCmd(opts))
	return cmd
}

func newAuthLoginCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Launchpad (interactive browser flow)",
		RunE: func(cmd *cobra.Command, args []string) error {
			consumerKey := lp.ConsumerKey()

			fmt.Fprintln(opts.Out, "Starting Launchpad OAuth flow...")
			rt, err := lp.ObtainRequestToken(consumerKey)
			if err != nil {
				return fmt.Errorf("requesting token: %w", err)
			}

			authURL := rt.AuthorizeURL()
			fmt.Fprintf(opts.Out, "\nOpen this URL in your browser to authorize:\n\n  %s\n\n", authURL)
			fmt.Fprint(opts.Out, "Press Enter after authorizing in the browser...")

			// Wait for user confirmation
			var input string
			_, _ = fmt.Scanln(&input)

			creds, err := lp.ExchangeAccessToken(consumerKey, rt)
			if err != nil {
				return fmt.Errorf("exchanging token: %w", err)
			}

			if err := lp.SaveCredentials(creds); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}

			fmt.Fprintf(opts.Out, "Authenticated! Credentials saved to %s\n", lp.CredentialsPath())
			return nil
		},
	}
}

func newAuthStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := lp.LoadCredentials()
			if err != nil {
				return fmt.Errorf("loading credentials: %w", err)
			}
			if creds == nil {
				fmt.Fprintln(opts.Out, "Not authenticated. Run 'watchtower auth login' to authenticate.")
				return nil
			}

			// Verify by calling /~ (current user)
			client := lp.NewClient(creds, opts.Logger)
			me, err := client.Me(cmd.Context())
			if err != nil {
				fmt.Fprintf(opts.Out, "Credentials found but invalid: %v\n", err)
				return nil
			}

			fmt.Fprintf(opts.Out, "Authenticated as: %s (%s)\n", me.DisplayName, me.Name)
			return nil
		},
	}
}
