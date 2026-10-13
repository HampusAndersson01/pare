// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bright-interaction/pare/internal/store"
)

func TestVerifikatTemplateCollapsesReversalHistory(t *testing.T) {
	pd := pageData{Title: "Verifikat", Email: "op@example.com", CSRF: "tok", Data: verifikatData{
		Verifications: []store.VerificationSummary{
			{Series: "A", Number: 1, Date: "2026-10-01", Description: "Aktiv post"},
			{Series: "A", Number: 2, Date: "2026-10-02", Description: "Ångrad post", IsReversed: true},
			{Series: "R", Number: 1, Date: "2026-10-03", Description: "Rättelsepost", IsReversal: true},
		},
	}}

	rec := httptest.NewRecorder()
	render(rec, "verifikat", pd, 200)
	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}

	detailsAt := strings.Index(body, "<details")
	if detailsAt < 0 || !strings.Contains(body, "Visa ångrade och rättelser") {
		t.Fatalf("expected reversal history to be collapsed in a details block: %s", body)
	}
	activeAt := strings.Index(body, "Aktiv post")
	reversedAt := strings.Index(body, "Ångrad post")
	reversalAt := strings.Index(body, "Rättelsepost")
	if activeAt < 0 || activeAt > detailsAt {
		t.Fatalf("active verification should remain in the main list")
	}
	if reversedAt < detailsAt || reversalAt < detailsAt {
		t.Fatalf("reversed verifications and reversals should be inside collapsed history")
	}
}
