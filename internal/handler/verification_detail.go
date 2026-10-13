// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
// Copyright (c) Bright Interaction

package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bright-interaction/pare/internal/ledger"
	"github.com/bright-interaction/pare/internal/store"
)

type verificationDetailLine struct {
	Account string
	Name    string
	Debit   ledger.Amount
	Credit  ledger.Amount
}

type verificationDocumentView struct {
	store.DocumentMeta
	IsImage bool
	IsPDF   bool
}

type verificationRelation struct {
	ID    uuid.UUID
	Label string
}

type verificationDetailData struct {
	Verification store.VerificationSummary
	Lines        []verificationDetailLine
	Documents    []verificationDocumentView
	TotalDebit   ledger.Amount
	TotalCredit  ledger.Amount
	ReversalOf   verificationRelation
	ReversedBy   verificationRelation
}

func (s *Server) handleVerificationDetail(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	co, ctx := companyID(r), r.Context()
	detail, err := s.Store.VerificationDetail(ctx, co, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	names := map[string]string{}
	if accounts, err := s.Store.ChartAccounts(ctx, co); err == nil {
		for _, a := range accounts {
			names[a.Number] = a.Name
		}
	}
	data := verificationDetailData{Verification: detail.Verification}
	for _, l := range detail.Verification.Lines {
		data.Lines = append(data.Lines, verificationDetailLine{Account: l.Account, Name: names[l.Account], Debit: l.Debit, Credit: l.Credit})
		data.TotalDebit += l.Debit
		data.TotalCredit += l.Credit
	}
	for _, d := range detail.Documents {
		mime := strings.ToLower(d.Mime)
		data.Documents = append(data.Documents, verificationDocumentView{
			DocumentMeta: d,
			IsImage:      strings.HasPrefix(mime, "image/"),
			IsPDF:        mime == "application/pdf",
		})
	}

	if detail.ReversalOf != uuid.Nil || detail.ReversedBy != uuid.Nil {
		if all, err := s.Store.ListVerificationSummaries(ctx, co); err == nil {
			for _, v := range all {
				label := v.Series + strconv.Itoa(v.Number)
				if v.ID == detail.ReversalOf {
					data.ReversalOf = verificationRelation{ID: v.ID, Label: label}
				}
				if v.ID == detail.ReversedBy {
					data.ReversedBy = verificationRelation{ID: v.ID, Label: label}
				}
			}
		}
	}

	pd := s.base(r, detail.Verification.Series+strconv.Itoa(detail.Verification.Number))
	pd.Data = data
	render(w, "verification_detail", pd, http.StatusOK)
}
