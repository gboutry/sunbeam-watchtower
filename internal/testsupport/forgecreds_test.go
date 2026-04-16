// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package testsupport

import (
	"os"
	"testing"
)

func TestClearForgeCredentialsBlanksEveryVar(t *testing.T) {
	for _, name := range ForgeCredentialEnvVars {
		t.Setenv(name, "sentinel-"+name)
	}

	ClearForgeCredentials(t)

	for _, name := range ForgeCredentialEnvVars {
		value, ok := os.LookupEnv(name)
		if !ok {
			t.Fatalf("env var %q not set after ClearForgeCredentials", name)
		}
		if value != "" {
			t.Fatalf("env var %q = %q, want empty string", name, value)
		}
	}
}
