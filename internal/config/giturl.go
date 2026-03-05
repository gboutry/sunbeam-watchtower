package config

import "fmt"

// CloneURL returns the git clone URL for this code config.
// If GitURL is set, it is returned directly. Otherwise the URL is derived
// from the forge type, host/owner and project name.
func (c CodeConfig) CloneURL() (string, error) {
	if c.GitURL != "" {
		return c.GitURL, nil
	}

	switch c.Forge {
	case "github":
		return fmt.Sprintf("https://github.com/%s/%s.git", c.Owner, c.Project), nil
	case "gerrit":
		return fmt.Sprintf("%s/%s", c.Host, c.Project), nil
	case "launchpad":
		return fmt.Sprintf("https://git.launchpad.net/%s", c.Project), nil
	default:
		return "", fmt.Errorf("cannot derive clone URL for forge %q", c.Forge)
	}
}

// CommitURL returns a web URL pointing to a specific commit.
func (c CodeConfig) CommitURL(sha string) string {
	switch c.Forge {
	case "github":
		return fmt.Sprintf("https://github.com/%s/%s/commit/%s", c.Owner, c.Project, sha)
	case "gerrit":
		return fmt.Sprintf("%s/gitweb?p=%s.git;a=commit;h=%s", c.Host, c.Project, sha)
	case "launchpad":
		return fmt.Sprintf("https://git.launchpad.net/%s/commit/?id=%s", c.Project, sha)
	default:
		return ""
	}
}
