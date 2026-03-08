package cli

import (
	"bufio"
	"context"
	"fmt"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
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
			styler := newOutputStylerForOptions(opts, opts.Out, opts.Output)
			fmt.Fprintln(opts.Out, "Starting Launchpad OAuth flow...")
			workflow := opts.Frontend().Auth()
			login, err := workflow.LoginLaunchpad(cmd.Context(), func(ctx context.Context, begin *dto.LaunchpadAuthBeginResult) error {
				fmt.Fprintf(opts.Out, "\n%s\n\n  %s\n\n%s", styler.Section("Open this URL in your browser to authorize:"), styler.DetailValue("URL", begin.AuthorizeURL), styler.Key("Press Enter after authorizing in the browser..."))
				if _, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n'); err != nil {
					return fmt.Errorf("waiting for authorization confirmation: %w", err)
				}
				return nil
			})
			if err != nil {
				return err
			}

			finalized := login.Finalized
			if finalized.Launchpad.Authenticated {
				fmt.Fprintf(opts.Out, "%s %s (%s)\n", styler.Key("Authenticated as:"), finalized.Launchpad.DisplayName, finalized.Launchpad.Username)
			}
			if finalized.Launchpad.CredentialsPath != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Credentials saved to"), finalized.Launchpad.CredentialsPath)
			}
			if finalized.Launchpad.Error != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Credentials saved, but verification failed:"), finalized.Launchpad.Error)
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
			workflow := opts.Frontend().Auth()
			styler := newOutputStylerForOptions(opts, opts.Out, opts.Output)
			status, err := workflow.Status(cmd.Context())
			if err != nil {
				return err
			}
			if !status.Launchpad.Authenticated {
				if status.Launchpad.Error != "" {
					fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Credentials found but invalid:"), status.Launchpad.Error)
					return nil
				}
				fmt.Fprintf(opts.Out, "%s Run 'watchtower auth login' to authenticate.\n", styler.Warning("Not authenticated."))
				return nil
			}

			fmt.Fprintf(opts.Out, "%s %s (%s)\n", styler.Key("Authenticated as:"), status.Launchpad.DisplayName, status.Launchpad.Username)
			if status.Launchpad.Source != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Source:"), status.Launchpad.Source)
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
			workflow := opts.Frontend().Auth()
			styler := newOutputStylerForOptions(opts, opts.Out, opts.Output)
			result, err := workflow.LogoutLaunchpad(cmd.Context())
			if err != nil {
				return err
			}
			if !result.Cleared {
				fmt.Fprintln(opts.Out, styler.Placeholder("No persisted Launchpad credentials were found."))
				return nil
			}
			if result.CredentialsPath != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Removed Launchpad credentials from"), result.CredentialsPath)
				return nil
			}
			fmt.Fprintf(opts.Out, "%s persisted Launchpad credentials.\n", styler.Action("Removed"))
			return nil
		},
	}
}
