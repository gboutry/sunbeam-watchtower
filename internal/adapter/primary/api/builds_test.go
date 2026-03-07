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
	input.Body.TargetProject = "remote-project"
	input.Body.Prepared = &dto.PreparedBuildSource{
		Backend:       dto.PreparedBuildBackendLaunchpad,
		TargetProject: "prepared-project",
		Repository:    "/repo/demo",
		Recipes: map[string]dto.PreparedBuildRecipe{
			"tmp-build-01234567-keystone": {SourceRef: "/ref/tmp", BuildPath: "rocks/keystone"},
		},
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
	if got.Owner != "lp-user" || got.Prefix != "tmp-build" || got.TargetProject != "remote-project" {
		t.Fatalf("unexpected trigger options: %+v", got)
	}
	if got.Prepared == nil {
		t.Fatal("Prepared = nil, want value")
	}
	normalized := got.Prepared.Normalize()
	if normalized.TargetProject != "prepared-project" || normalized.Repository != "/repo/demo" {
		t.Fatalf("unexpected prepared source: %+v", got.Prepared)
	}
	if normalized.Recipes["tmp-build-01234567-keystone"].BuildPath != "rocks/keystone" {
		t.Fatalf("unexpected prepared recipes: %+v", normalized.Recipes)
	}
}
