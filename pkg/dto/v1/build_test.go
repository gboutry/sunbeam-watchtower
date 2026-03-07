// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package dto

import "testing"

func TestPreparedBuildSourceNormalizeDefaultsBackend(t *testing.T) {
	source := (&PreparedBuildSource{
		TargetProject: "lp-project",
		Repository:    "/repo/demo",
		Recipes: map[string]PreparedBuildRecipe{
			"tmp-keystone": {SourceRef: "/ref/tmp-keystone", BuildPath: "rocks/keystone"},
		},
	}).Normalize()

	if source == nil {
		t.Fatal("Normalize() = nil, want value")
	}
	if source.Backend != PreparedBuildBackendLaunchpad {
		t.Fatalf("Backend = %q, want %q", source.Backend, PreparedBuildBackendLaunchpad)
	}
	if source.TargetProject != "lp-project" || source.Repository != "/repo/demo" {
		t.Fatalf("unexpected normalized source: %+v", source)
	}
	recipe := source.Recipes["tmp-keystone"]
	if recipe.SourceRef != "/ref/tmp-keystone" || recipe.BuildPath != "rocks/keystone" {
		t.Fatalf("normalized recipe = %+v", recipe)
	}
}

func TestPreparedBuildSourceNormalizePreservesGenericFields(t *testing.T) {
	source := (&PreparedBuildSource{
		Backend:       PreparedBuildBackendLaunchpad,
		TargetProject: "generic-project",
		Repository:    "/repo/generic",
		Recipes: map[string]PreparedBuildRecipe{
			"tmp-keystone": {SourceRef: "/ref/generic", BuildPath: "rocks/keystone"},
		},
	}).Normalize()

	if source.TargetProject != "generic-project" || source.Repository != "/repo/generic" {
		t.Fatalf("Normalize() overwrote generic fields: %+v", source)
	}
	recipe := source.Recipes["tmp-keystone"]
	if recipe.SourceRef != "/ref/generic" {
		t.Fatalf("normalized recipe = %+v, want generic source ref", recipe)
	}
}
