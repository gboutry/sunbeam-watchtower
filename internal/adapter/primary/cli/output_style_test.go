package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	distro "github.com/gboutry/sunbeam-watchtower/pkg/distro/v1"
	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
	forge "github.com/gboutry/sunbeam-watchtower/pkg/forge/v1"
)

func TestRenderReleaseList_ColorizesTableOutput(t *testing.T) {
	var out bytes.Buffer
	err := renderReleaseList(&out, "table", newOutputStyler(true), []dto.ReleaseListEntry{{
		Project:      "sunbeam",
		ArtifactType: dto.ArtifactSnap,
		Name:         "snap-openstack",
		Track:        "2024.1",
		Risk:         dto.ReleaseRiskStable,
		Targets: []dto.ReleaseTargetSnapshot{{
			Architecture: "amd64",
			Base:         dto.ReleaseBase{Name: "ubuntu", Channel: "24.04"},
			Revision:     41,
			Version:      "1.2.3",
		}},
		ReleasedAt: time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		t.Fatalf("renderReleaseList() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI color codes in output, got %q", got)
	}
	if !strings.Contains(got, "PROJECT") || !strings.Contains(got, "amd64@ubuntu/24.04:r41/1.2.3") {
		t.Fatalf("missing expected table content: %q", got)
	}
}

func TestRenderReleaseList_JSONRemainsPlain(t *testing.T) {
	var out bytes.Buffer
	err := renderReleaseList(&out, "json", newOutputStyler(true), []dto.ReleaseListEntry{{
		Project:      "sunbeam",
		ArtifactType: dto.ArtifactSnap,
		Name:         "snap-openstack",
		Track:        "2024.1",
		Risk:         dto.ReleaseRiskStable,
	}})
	if err != nil {
		t.Fatalf("renderReleaseList() error = %v", err)
	}

	if strings.Contains(out.String(), "\x1b[") {
		t.Fatalf("json output should not contain ANSI color codes: %q", out.String())
	}
}

func TestRenderPackageInfoTable_ColorizesKeys(t *testing.T) {
	info := &distro.SourcePackageInfo{
		SourcePackage: distro.SourcePackage{
			Package:   "keystone",
			Version:   "1:27.0.0-0ubuntu1",
			Suite:     "noble",
			Component: "main",
		},
		Fields: []distro.FieldEntry{
			{Key: "Homepage", Value: "https://example.invalid"},
			{Key: "Description", Value: "line 1\nline 2"},
		},
	}

	var out bytes.Buffer
	if err := renderPackageInfoTable(&out, newOutputStyler(true), info); err != nil {
		t.Fatalf("renderPackageInfoTable() error = %v", err)
	}

	got := out.String()
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected ANSI color codes in output, got %q", got)
	}
	if !strings.Contains(got, "Source:") || !strings.Contains(got, "Homepage:") {
		t.Fatalf("missing expected key labels: %q", got)
	}
}

func TestWriteWarningLine_PlainWhenDisabled(t *testing.T) {
	var out bytes.Buffer
	if err := writeWarningLine(&out, newOutputStyler(false), "partial tracker failure"); err != nil {
		t.Fatalf("writeWarningLine() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("plain warning output should not contain ANSI color codes: %q", got)
	}
	if !strings.Contains(got, "warning: partial tracker failure") {
		t.Fatalf("unexpected warning output: %q", got)
	}
}

func TestRenderBugTasks_TableStripsLaunchpadPrefixAndQuotes(t *testing.T) {
	var out bytes.Buffer
	err := renderBugTasks(&out, "table", newOutputStyler(false), []forge.BugTask{{
		Project:    "snap-openstack",
		BugID:      "2134598",
		Status:     "Triaged",
		Importance: "Medium",
		Title:      `Bug #2134598 in OpenStack Snap: "Fix bootstrap race in update flow"`,
		URL:        "https://bugs.launchpad.net/bugs/2134598",
	}})
	if err != nil {
		t.Fatalf("renderBugTasks() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, `Bug #2134598 in OpenStack Snap:`) {
		t.Fatalf("expected Launchpad bug prefix to be stripped: %q", got)
	}
	if strings.Contains(got, `"Fix bootstrap race in update flow"`) {
		t.Fatalf("expected surrounding quotes to be stripped: %q", got)
	}
	if !strings.Contains(got, "Fix bootstrap race in update flow") {
		t.Fatalf("expected cleaned bug title in output: %q", got)
	}
}
