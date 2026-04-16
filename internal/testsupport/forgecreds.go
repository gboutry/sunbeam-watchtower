// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

// Package testsupport provides shared test helpers.
package testsupport

import "testing"

// ForgeCredentialEnvVars is the canonical set of env vars that production
// credential stores read. Tests whose outcome depends on the absence (or a
// specific value) of forge credentials must override every entry so the
// developer's shell environment cannot influence the test.
var ForgeCredentialEnvVars = []string{
	"LP_ACCESS_TOKEN",
	"LP_ACCESS_TOKEN_SECRET",
	"GH_TOKEN",
	"GITHUB_TOKEN",
	"SNAPCRAFT_STORE_CREDENTIALS",
	"CHARMCRAFT_AUTH",
}

// ClearForgeCredentials sets every forge credential env var to the empty
// string for the duration of the test, isolating the test from any real
// credentials the developer may have exported in their shell.
func ClearForgeCredentials(t testing.TB) {
	t.Helper()
	for _, name := range ForgeCredentialEnvVars {
		t.Setenv(name, "")
	}
}
