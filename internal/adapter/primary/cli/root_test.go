package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(out.String(), "watchtower") {
		t.Errorf("output = %q, want to contain 'watchtower'", out.String())
	}
}

func TestConfigShowCommand(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")

	yaml := `
launchpad:
  default_owner: test-team
projects:
  - name: test-project
    code:
      forge: github
      owner: org
      project: repo
    bugs:
      - forge: launchpad
        project: test-project
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"config", "show", "--config", cfgFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "test-team") {
		t.Errorf("output should contain 'test-team', got: %s", output)
	}
	if !strings.Contains(output, "test-project") {
		t.Errorf("output should contain 'test-project', got: %s", output)
	}
}

func TestConfigShowCommand_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"config", "show", "--config", "/nonexistent/path.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestReviewListCommand_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "list", "--config", "/nonexistent/path.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestReviewListCommand_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "list", "--config", cfgFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(out.String(), "No merge requests") {
		t.Errorf("expected 'No merge requests' message, got: %s", out.String())
	}
}

func TestReviewListCommand_InvalidState(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "list", "--config", cfgFile, "--state", "invalid"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestReviewListCommand_InvalidForge(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "list", "--config", cfgFile, "--forge", "gitlab"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid forge")
	}
}

func TestOutputFormats(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			var out, errOut bytes.Buffer
			opts := &Options{Out: &out, ErrOut: &errOut}
			cmd := NewRootCmd(opts)
			cmd.SetArgs([]string{"review", "list", "--config", cfgFile, "-o", format})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error: %v", err)
			}

			// Both JSON and YAML should produce valid output (even if empty).
			if out.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestBugListCommand_NoConfig(t *testing.T) {
	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"bug", "list", "--config", "/nonexistent/path.yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
}

func TestBugListCommand_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"bug", "list", "--config", cfgFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(out.String(), "No bug tasks") {
		t.Errorf("expected 'No bug tasks' message, got: %s", out.String())
	}
}

func TestBugListCommand_OutputFormats(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	for _, format := range []string{"json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			var out, errOut bytes.Buffer
			opts := &Options{Out: &out, ErrOut: &errOut}
			cmd := NewRootCmd(opts)
			cmd.SetArgs([]string{"bug", "list", "--config", cfgFile, "-o", format})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error: %v", err)
			}

			if out.Len() == 0 {
				t.Error("expected non-empty output")
			}
		})
	}
}

func TestReviewShowCommand_MissingProject(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "show", "--config", cfgFile, "#1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --project flag")
	}
	if !strings.Contains(err.Error(), "--project") {
		t.Errorf("error should mention --project, got: %v", err)
	}
}

func TestReviewShowCommand_UnknownProject(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "show", "--config", cfgFile, "--project", "nonexistent", "#1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown project")
	}
}

func TestBugShowCommand_NoArgs(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"bug", "show", "--config", cfgFile})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing bug ID argument")
	}
}

func TestVerboseFlag(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgFile, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"review", "list", "--config", cfgFile, "--verbose"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !opts.Verbose {
		t.Error("expected Verbose to be true")
	}
}

func TestProjectSyncCommand_NoLPProjects(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	yaml := `
projects:
  - name: test
    code:
      forge: github
      owner: org
      project: repo
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &errOut}
	cmd := NewRootCmd(opts)
	cmd.SetArgs([]string{"project", "sync", "--config", cfgFile})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if !strings.Contains(out.String(), "No changes needed") {
		t.Errorf("expected 'No changes needed' message, got: %s", out.String())
	}
}

func TestProjectSyncCommand_HasFlags(t *testing.T) {
	var out bytes.Buffer
	opts := &Options{Out: &out, ErrOut: &bytes.Buffer{}}
	cmd := NewRootCmd(opts)

	projectCmd, _, err := cmd.Find([]string{"project", "sync"})
	if err != nil {
		t.Fatalf("project sync command not found: %v", err)
	}

	if projectCmd.Flag("dry-run") == nil {
		t.Error("expected --dry-run flag")
	}
	if projectCmd.Flag("project") == nil {
		t.Error("expected --project flag")
	}
}
