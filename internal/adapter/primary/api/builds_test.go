package api

import (
	"testing"
	"time"

	dto "github.com/gboutry/sunbeam-watchtower/pkg/dto/v1"
)

func TestBuildTriggerOptionsFromInput_PreparedSource(t *testing.T) {
	input := &BuildsTriggerInput{}
	input.Body.Project = "demo"
	input.Body.Wait = true
	input.Body.Timeout = "45m"
	input.Body.Owner = "lp-user"
	input.Body.Prefix = "tmp-build"
	input.Body.LPProject = "remote-project"
	input.Body.Prepared = &dto.PreparedBuildSource{
		LPProject:    "prepared-project",
		RepoSelfLink: "/repo/demo",
		GitRefLinks:  map[string]string{"tmp-build-01234567-keystone": "/ref/tmp"},
		BuildPaths:   map[string]string{"tmp-build-01234567-keystone": "rocks/keystone"},
	}

	got, err := buildTriggerOptionsFromInput(input)
	if err != nil {
		t.Fatalf("buildTriggerOptionsFromInput() error = %v", err)
	}

	if !got.Wait {
		t.Fatal("Wait = false, want true")
	}
	if got.Timeout != 45*time.Minute {
		t.Fatalf("Timeout = %s, want 45m", got.Timeout)
	}
	if got.Owner != "lp-user" || got.Prefix != "tmp-build" || got.LPProject != "remote-project" {
		t.Fatalf("unexpected trigger options: %+v", got)
	}
	if got.Prepared == nil {
		t.Fatal("Prepared = nil, want value")
	}
	if got.Prepared.LPProject != "prepared-project" || got.Prepared.RepoSelfLink != "/repo/demo" {
		t.Fatalf("unexpected prepared source: %+v", got.Prepared)
	}
	if got.Prepared.BuildPaths["tmp-build-01234567-keystone"] != "rocks/keystone" {
		t.Fatalf("unexpected build paths: %+v", got.Prepared.BuildPaths)
	}
}
