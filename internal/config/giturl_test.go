package config

import "testing"

func TestCodeConfig_CloneURL(t *testing.T) {
	tests := []struct {
		name    string
		cfg     CodeConfig
		want    string
		wantErr bool
	}{
		{
			name: "explicit git_url",
			cfg:  CodeConfig{Forge: "github", Owner: "org", Project: "repo", GitURL: "https://custom.example.com/repo.git"},
			want: "https://custom.example.com/repo.git",
		},
		{
			name: "github derived",
			cfg:  CodeConfig{Forge: "github", Owner: "openstack", Project: "nova"},
			want: "https://github.com/openstack/nova.git",
		},
		{
			name: "gerrit derived",
			cfg:  CodeConfig{Forge: "gerrit", Host: "https://review.opendev.org", Project: "openstack/nova"},
			want: "https://review.opendev.org/openstack/nova",
		},
		{
			name: "launchpad derived",
			cfg:  CodeConfig{Forge: "launchpad", Project: "sunbeam"},
			want: "https://git.launchpad.net/sunbeam",
		},
		{
			name:    "unknown forge",
			cfg:     CodeConfig{Forge: "gitlab"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.CloneURL()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("CloneURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodeConfig_CommitURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  CodeConfig
		sha  string
		want string
	}{
		{
			name: "github",
			cfg:  CodeConfig{Forge: "github", Owner: "org", Project: "repo"},
			sha:  "abc123",
			want: "https://github.com/org/repo/commit/abc123",
		},
		{
			name: "gerrit",
			cfg:  CodeConfig{Forge: "gerrit", Host: "https://review.opendev.org", Project: "openstack/nova"},
			sha:  "abc123",
			want: "https://review.opendev.org/gitweb?p=openstack/nova.git;a=commit;h=abc123",
		},
		{
			name: "launchpad",
			cfg:  CodeConfig{Forge: "launchpad", Project: "sunbeam"},
			sha:  "abc123",
			want: "https://git.launchpad.net/sunbeam/commit/?id=abc123",
		},
		{
			name: "unknown",
			cfg:  CodeConfig{Forge: "gitlab"},
			sha:  "abc123",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.CommitURL(tt.sha)
			if got != tt.want {
				t.Errorf("CommitURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
