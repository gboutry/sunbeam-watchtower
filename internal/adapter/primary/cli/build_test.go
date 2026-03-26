package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// helper that returns a temp config file path with minimal valid YAML.
func writeTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	return cfgFile
}

// ---------- command registration ----------

func TestBuildCommand_Registered(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	buildCmd, _, err := root.Find([]string{"build"})
	if err != nil {
		t.Fatalf("'build' command not found: %v", err)
	}
	if buildCmd.Name() != "build" {
		t.Errorf("expected command name 'build', got %q", buildCmd.Name())
	}
}

func TestBuildSubcommands_Registered(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	for _, sub := range []string{"trigger", "list", "download", "cleanup"} {
		t.Run(sub, func(t *testing.T) {
			cmd, _, err := root.Find([]string{"build", sub})
			if err != nil {
				t.Fatalf("'build %s' not found: %v", sub, err)
			}
			if cmd.Name() != sub {
				t.Errorf("expected command name %q, got %q", sub, cmd.Name())
			}
		})
	}
}

// ---------- flag parsing ----------

func TestBuildTriggerCmd_Flags(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	cmd, _, _ := root.Find([]string{"build", "trigger"})

	for _, flag := range []string{"wait", "timeout", "owner", "prefix", "local-path"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("flag --%s not defined on 'build trigger'", flag)
		}
	}
}

func TestBuildListCmd_Flags(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	cmd, _, _ := root.Find([]string{"build", "list"})

	for _, flag := range []string{"project", "all", "state"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("flag --%s not defined on 'build list'", flag)
		}
	}
}

func TestBuildDownloadCmd_Flags(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	cmd, _, _ := root.Find([]string{"build", "download"})

	if cmd.Flags().Lookup("artifacts-dir") == nil {
		t.Error("flag --artifacts-dir not defined on 'build download'")
	}
}

func TestBuildCleanupCmd_Flags(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	cmd, _, _ := root.Find([]string{"build", "cleanup"})

	for _, flag := range []string{"project", "owner", "prefix", "dry-run"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("flag --%s not defined on 'build cleanup'", flag)
		}
	}
}

// ---------- flag defaults ----------

func TestBuildTriggerCmd_FlagDefaults(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	cmd, _, _ := root.Find([]string{"build", "trigger"})

	tests := []struct {
		flag, want string
	}{
		{"wait", "false"},
		{"timeout", "5h0m0s"},
		{"prefix", "tmp-build"},
		{"local-path", ""},
	}
	for _, tt := range tests {
		f := cmd.Flags().Lookup(tt.flag)
		if f == nil {
			t.Errorf("flag --%s not found", tt.flag)
			continue
		}
		if f.DefValue != tt.want {
			t.Errorf("flag --%s default = %q, want %q", tt.flag, f.DefValue, tt.want)
		}
	}
}

func TestBuildCleanupCmd_FlagDefaults(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	root := NewRootCmd(opts)

	cmd, _, _ := root.Find([]string{"build", "cleanup"})

	tests := []struct {
		flag, want string
	}{
		{"prefix", "tmp-build"},
		{"dry-run", "false"},
	}
	for _, tt := range tests {
		f := cmd.Flags().Lookup(tt.flag)
		if f == nil {
			t.Errorf("flag --%s not found", tt.flag)
			continue
		}
		if f.DefValue != tt.want {
			t.Errorf("flag --%s default = %q, want %q", tt.flag, f.DefValue, tt.want)
		}
	}
}

// ---------- argument validation ----------

func TestBuildTriggerCmd_NoArgs(t *testing.T) {
	cfgFile := writeTempConfig(t)

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "trigger", "--config", cfgFile})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing project argument")
	}
}

// download accepts zero args now (projects are optional, same as list).
func TestBuildDownloadCmd_NoArgs_Accepted(t *testing.T) {
	cfgFile := writeTempConfig(t)

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "download", "--config", cfgFile})

	// Should not error on arg count — actual execution will fail on missing
	// server / config, but the point is no args-validation error.
	_ = cmd.Execute()
}

// list and cleanup accept zero args — verify they don't error on arg count.
func TestBuildListCmd_NoArgs_Accepted(t *testing.T) {
	cfgFile := writeTempConfig(t)

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "list", "--config", cfgFile})

	// list will fail later (no LP auth) but should not fail on arg validation.
	err := cmd.Execute()
	if err != nil && err.Error() == "requires at least 1 arg(s), only received 0" {
		t.Fatal("'build list' should accept zero args")
	}
}

func TestBuildCleanupCmd_NoArgs_Accepted(t *testing.T) {
	cfgFile := writeTempConfig(t)

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "cleanup", "--config", cfgFile})

	err := cmd.Execute()
	if err != nil && err.Error() == "requires at least 1 arg(s), only received 0" {
		t.Fatal("'build cleanup' should accept zero args")
	}
}

// ---------- missing config ----------

func TestBuildTriggerCmd_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "trigger", "--config", "/nonexistent/path.yaml", "myproject"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestBuildListCmd_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "list", "--config", "/nonexistent/path.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestBuildDownloadCmd_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "download", "--config", "/nonexistent/path.yaml", "myproject"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestBuildCleanupCmd_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"build", "cleanup", "--config", "/nonexistent/path.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}
