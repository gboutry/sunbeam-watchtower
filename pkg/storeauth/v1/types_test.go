// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrPendingAuthFlowNotFound", ErrPendingAuthFlowNotFound, "store auth flow not found"},
		{"ErrPendingAuthFlowExpired", ErrPendingAuthFlowExpired, "store auth flow expired"},
		{"ErrDischargePending", ErrDischargePending, "store auth discharge pending"},
		{"ErrDischargeExpired", ErrDischargeExpired, "store auth discharge expired"},
		{"ErrDischargeDenied", ErrDischargeDenied, "store auth discharge denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	if errors.Is(ErrPendingAuthFlowNotFound, ErrPendingAuthFlowExpired) {
		t.Fatal("ErrPendingAuthFlowNotFound should not match ErrPendingAuthFlowExpired")
	}
	if errors.Is(ErrDischargePending, ErrDischargeDenied) {
		t.Fatal("ErrDischargePending should not match ErrDischargeDenied")
	}
}

func TestPendingAuthFlowFieldsAccessible(t *testing.T) {
	flow := PendingAuthFlow{
		ID:           "test-id",
		RootMacaroon: "root-mac",
		CaveatID:     "caveat-id",
		DischargeID:  "discharge-id",
		VisitURL:     "https://example.com/visit",
		WaitURL:      "https://example.com/wait",
	}
	if flow.ID != "test-id" {
		t.Fatalf("ID = %q, want test-id", flow.ID)
	}
	if flow.RootMacaroon != "root-mac" {
		t.Fatalf("RootMacaroon = %q, want root-mac", flow.RootMacaroon)
	}
	if flow.VisitURL != "https://example.com/visit" {
		t.Fatalf("VisitURL = %q", flow.VisitURL)
	}
}
