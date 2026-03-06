package cli

import (
	"bufio"
	"fmt"

	"github.com/spf13/cobra"
)

func newAuthCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	cmd.AddCommand(newAuthLoginCmd(opts))
	cmd.AddCommand(newAuthStatusCmd(opts))
	cmd.AddCommand(newAuthLogoutCmd(opts))
	return cmd
}

func newAuthLoginCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Launchpad (interactive browser flow)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(opts.Out, "Starting Launchpad OAuth flow...")
			result, err := opts.Client.AuthLaunchpadBegin(cmd.Context())
			if err != nil {
				return err
			}

			fmt.Fprintf(opts.Out, "\nOpen this URL in your browser to authorize:\n\n  %s\n\n", result.AuthorizeURL)
			fmt.Fprint(opts.Out, "Press Enter after authorizing in the browser...")
			if _, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n'); err != nil {
				return fmt.Errorf("waiting for authorization confirmation: %w", err)
			}

			finalized, err := opts.Client.AuthLaunchpadFinalize(cmd.Context(), result.FlowID)
			if err != nil {
				return err
			}

			if finalized.Launchpad.Authenticated {
				fmt.Fprintf(
					opts.Out,
					"Authenticated as: %s (%s)\n",
					finalized.Launchpad.DisplayName,
					finalized.Launchpad.Username,
				)
			}
			if finalized.Launchpad.CredentialsPath != "" {
				fmt.Fprintf(opts.Out, "Credentials saved to %s\n", finalized.Launchpad.CredentialsPath)
			}
			if finalized.Launchpad.Error != "" {
				fmt.Fprintf(opts.Out, "Credentials saved, but verification failed: %s\n", finalized.Launchpad.Error)
			}
			return nil
		},
	}
}

func newAuthStatusCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := opts.Client.AuthStatus(cmd.Context())
			if err != nil {
				return err
			}
			if !status.Launchpad.Authenticated {
				if status.Launchpad.Error != "" {
					fmt.Fprintf(opts.Out, "Credentials found but invalid: %s\n", status.Launchpad.Error)
					return nil
				}
				fmt.Fprintln(opts.Out, "Not authenticated. Run 'watchtower auth login' to authenticate.")
				return nil
			}

			fmt.Fprintf(opts.Out, "Authenticated as: %s (%s)\n", status.Launchpad.DisplayName, status.Launchpad.Username)
			if status.Launchpad.Source != "" {
				fmt.Fprintf(opts.Out, "Source: %s\n", status.Launchpad.Source)
			}
			return nil
		},
	}
}

func newAuthLogoutCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear persisted Launchpad credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := opts.Client.AuthLaunchpadLogout(cmd.Context())
			if err != nil {
				return err
			}
			if !result.Cleared {
				fmt.Fprintln(opts.Out, "No persisted Launchpad credentials were found.")
				return nil
			}
			if result.CredentialsPath != "" {
				fmt.Fprintf(opts.Out, "Removed Launchpad credentials from %s\n", result.CredentialsPath)
				return nil
			}
			fmt.Fprintln(opts.Out, "Removed persisted Launchpad credentials.")
			return nil
		},
	}
}
