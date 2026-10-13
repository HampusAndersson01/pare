// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bright-interaction/pare/internal/store"
)

type receiptInboxRow struct {
	ID        uuid.UUID
	Filename  string
	Note      string
	CreatedAt time.Time
	Attached  bool
	Target    string
}

type receiptInboxData struct {
	Documents     []receiptInboxRow
	Verifications []store.VerificationSummary
}

// handleReceiptsWithVerificationLinks is the receipt inbox with direct linking
// to existing verifications in addition to the supplier-invoice flow.
func (s *Server) handleReceiptsWithVerificationLinks(w http.ResponseWriter, r *http.Request) {
	co, ctx := companyID(r), r.Context()
	docs, err := s.Store.ListDocuments(ctx, co)
	if err != nil {
		s.fail(w, err)
		return
	}
	refs, err := s.Store.ListDocumentLinkRefs(ctx, co)
	if err != nil {
		s.fail(w, err)
		return
	}
	links := make(map[uuid.UUID]store.DocumentLinkRef, len(refs))
	for _, ref := range refs {
		links[ref.ID] = ref
	}
	vers, err := s.Store.ListVerificationSummaries(ctx, co)
	if err != nil {
		s.fail(w, err)
		return
	}

	rows := make([]receiptInboxRow, 0, len(docs))
	for _, d := range docs {
		row := receiptInboxRow{ID: d.ID, Filename: d.Filename, Note: d.Note, CreatedAt: d.CreatedAt, Attached: d.Attached}
		if ref, ok := links[d.ID]; ok {
			switch {
			case ref.VerificationID != uuid.Nil:
				row.Attached = true
				row.Target = "verifikat"
			case ref.SupplierInvoiceID != uuid.Nil:
				row.Attached = true
				row.Target = "leverantörsfaktura"
			}
		}
		rows = append(rows, row)
	}

	pd := s.base(r, "Kvitton")
	switch r.URL.Query().Get("msg") {
	case "uploaded":
		pd.Flash = "Underlaget laddades upp och krypterades."
	case "deleted":
		pd.Flash = "Underlaget togs bort."
	case "attached":
		pd.Error = "Underlaget är kopplat till en bokförd post och kan inte tas bort."
	case "linked":
		pd.Flash = "Underlaget kopplades till verifikatet."
	case "linkbad":
		pd.Error = "Underlaget kunde inte kopplas. Det kan redan vara kopplat eller verifikatet saknas."
	}
	pd.Data = receiptInboxData{Documents: rows, Verifications: vers}
	render(w, "kvitton", pd, http.StatusOK)
}

func (s *Server) handleReceiptAttachVerification(w http.ResponseWriter, r *http.Request) {
	docID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	_ = r.ParseForm()
	verificationID, err := uuid.Parse(r.PostFormValue("verification_id"))
	if err != nil {
		http.Redirect(w, r, "/kvitton?msg=linkbad", http.StatusSeeOther)
		return
	}
	err = s.Store.AttachDocumentToVerification(r.Context(), companyID(r), docID, verificationID)
	switch {
	case err == nil:
		http.Redirect(w, r, "/kvitton?msg=linked", http.StatusSeeOther)
	case errors.Is(err, store.ErrDocumentAttachConflict):
		http.Redirect(w, r, "/kvitton?msg=linkbad", http.StatusSeeOther)
	default:
		s.fail(w, err)
	}
}
