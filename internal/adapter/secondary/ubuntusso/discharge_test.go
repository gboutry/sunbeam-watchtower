// SPDX-FileCopyrightText: 2026 - gboutry
// SPDX-License-Identifier: Apache-2.0

package ubuntusso

import (
	"context"
	"testing"
)

func TestDischargeAllRejectsInvalidMacaroon(t *testing.T) {
	_, err := DischargeAll(context.Background(), "not-a-macaroon", nil)
	if err == nil {
		t.Fatal("expected error for invalid macaroon input")
	}
}
