package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"

	frontend "github.com/gboutry/sunbeam-watchtower/internal/adapter/primary/frontend"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	"github.com/spf13/cobra"
)

func newAuthCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}
	cmd.AddCommand(newAuthStatusCmd(opts))
	cmd.AddCommand(newAuthLaunchpadCmd(opts))
	cmd.AddCommand(newAuthGitHubCmd(opts))
	return cmd
}

func newAuthLaunchpadCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launchpad",
		Short: "Manage Launchpad authentication",
	}
	cmd.AddCommand(newAuthLaunchpadLoginCmd(opts))
	cmd.AddCommand(newAuthLaunchpadLogoutCmd(opts))
	return cmd
}

func newAuthGitHubCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "github",
		Short: "Manage GitHub authentication",
	}
	cmd.AddCommand(newAuthGitHubLoginCmd(opts))
	cmd.AddCommand(newAuthGitHubLogoutCmd(opts))
	return cmd
}

func newAuthLaunchpadLoginCmd(opts *Options) *cobra.Command {
	return withActionID(&cobra.Command{
		Use:   "login",
		Short: "Authenticate with Launchpad",
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
	}, frontend.ActionAuthLaunchpadBegin)
}

func newAuthGitHubLoginCmd(opts *Options) *cobra.Command {
	return withActionID(&cobra.Command{
		Use:   "login",
		Short: "Authenticate with GitHub",
		RunE: func(cmd *cobra.Command, args []string) error {
			styler := newOutputStylerForOptions(opts, opts.Out, opts.Output)
			fmt.Fprintln(opts.Out, "Starting GitHub device flow...")
			workflow := opts.Frontend().Auth()
			login, err := workflow.LoginGitHub(cmd.Context(), func(ctx context.Context, begin *dto.GitHubAuthBeginResult) error {
				fmt.Fprintf(opts.Out, "\n%s\n\n  %s\n  %s\n\n%s\n", styler.Section("Open this URL in your browser and enter the code:"), styler.DetailValue("URL", begin.VerificationURI), styler.DetailValue("Code", begin.UserCode), styler.Key("Waiting for GitHub authorization..."))
				return nil
			})
			if err != nil {
				return err
			}

			finalized := login.Finalized
			if finalized.GitHub.Authenticated {
				fmt.Fprintf(opts.Out, "%s %s (%s)\n", styler.Key("Authenticated as:"), finalized.GitHub.DisplayName, finalized.GitHub.Username)
			}
			if finalized.GitHub.CredentialsPath != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Credentials saved to"), finalized.GitHub.CredentialsPath)
			}
			if finalized.GitHub.Error != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Credentials saved, but verification failed:"), finalized.GitHub.Error)
			}
			return nil
		},
	}, frontend.ActionAuthGitHubBegin)
}

func newAuthStatusCmd(opts *Options) *cobra.Command {
	return withActionID(&cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		RunE: func(cmd *cobra.Command, args []string) error {
			workflow := opts.Frontend().Auth()
			styler := newOutputStylerForOptions(opts, opts.Out, opts.Output)
			status, err := workflow.Status(cmd.Context())
			if err != nil {
				return err
			}
			writeAuthStatusSection(opts.Out, styler, "Launchpad", status.Launchpad.Authenticated, status.Launchpad.DisplayName, status.Launchpad.Username, status.Launchpad.Source, status.Launchpad.CredentialsPath, status.Launchpad.Error)
			fmt.Fprintln(opts.Out)
			writeAuthStatusSection(opts.Out, styler, "GitHub", status.GitHub.Authenticated, status.GitHub.DisplayName, status.GitHub.Username, status.GitHub.Source, status.GitHub.CredentialsPath, status.GitHub.Error)
			return nil
		},
	}, frontend.ActionAuthStatus)
}

func writeAuthStatusSection(out io.Writer, styler *outputStyler, provider string, authenticated bool, displayName string, username string, source string, credentialsPath string, statusError string) {
	fmt.Fprintf(out, "%s\n", styler.Section(provider))
	if !authenticated {
		if statusError != "" {
			fmt.Fprintf(out, "%s %s\n", styler.Key("Credentials found but invalid:"), statusError)
			return
		}
		fmt.Fprintln(out, styler.Warning("Not authenticated."))
		return
	}
	fmt.Fprintf(out, "%s %s (%s)\n", styler.Key("Authenticated as:"), displayName, username)
	if source != "" {
		fmt.Fprintf(out, "%s %s\n", styler.Key("Source:"), source)
	}
	if credentialsPath != "" {
		fmt.Fprintf(out, "%s %s\n", styler.Key("Credentials path:"), credentialsPath)
	}
}

func newAuthLaunchpadLogoutCmd(opts *Options) *cobra.Command {
	return withActionID(&cobra.Command{
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
	}, frontend.ActionAuthLaunchpadLogout)
}

func newAuthGitHubLogoutCmd(opts *Options) *cobra.Command {
	return withActionID(&cobra.Command{
		Use:   "logout",
		Short: "Clear persisted GitHub credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			workflow := opts.Frontend().Auth()
			styler := newOutputStylerForOptions(opts, opts.Out, opts.Output)
			result, err := workflow.LogoutGitHub(cmd.Context())
			if err != nil {
				return err
			}
			if !result.Cleared {
				fmt.Fprintln(opts.Out, styler.Placeholder("No persisted GitHub credentials were found."))
				return nil
			}
			if result.CredentialsPath != "" {
				fmt.Fprintf(opts.Out, "%s %s\n", styler.Key("Removed GitHub credentials from"), result.CredentialsPath)
				return nil
			}
			fmt.Fprintf(opts.Out, "%s persisted GitHub credentials.\n", styler.Action("Removed"))
			return nil
		},
	}, frontend.ActionAuthGitHubLogout)
}
