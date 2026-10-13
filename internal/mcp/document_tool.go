// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/bright-interaction/pare/internal/store"
)

type documentRefResult struct {
	Documents []documentRefRow `json:"documents"`
}

type documentRefRow struct {
	ID         string `json:"document_id"`
	SHA256     string `json:"sha256"`
	Mime       string `json:"mime"`
	ByteSize   int64  `json:"byte_size"`
	CreatedAt  string `json:"created_at"`
	AttachedTo string `json:"attached_to"`
	TargetID   string `json:"target_id,omitempty"`
}

type verificationRefResult struct {
	Verifications []verificationRefRow `json:"verifications"`
}

type verificationRefRow struct {
	ID     string `json:"verification_id"`
	Series string `json:"series"`
	Number int    `json:"number"`
	Date   string `json:"date"`
}

type documentAttachResult struct {
	DocumentID     string `json:"document_id"`
	VerificationID string `json:"verification_id"`
	Attached       bool   `json:"attached"`
	Note           string `json:"note,omitempty"`
}

func (s *Server) registerDocumentTools() {
	s.add(tool{
		name:   "pare_list_documents",
		desc:   "List uploaded receipt/document references without exposing filenames, notes or file contents. SHA-256 can be used to match a user-provided file to its encrypted Pare copy.",
		schema: emptySchema(),
		proto:  &documentRefResult{},
		run:    runListDocuments,
	})
	s.add(tool{
		name:   "pare_list_verifications",
		desc:   "List posted verification references (ID, series, number and date) without free-text descriptions or identities.",
		schema: emptySchema(),
		proto:  &verificationRefResult{},
		run:    runListVerifications,
	})
	s.add(tool{
		name:  "pare_attach_document_to_verification",
		desc:  "Attach an existing uploaded receipt/document to an existing posted verification. A document can support only one accounting record.",
		write: true,
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"document_id":     map[string]any{"type": "string", "description": "UUID from pare_list_documents"},
				"verification_id": map[string]any{"type": "string", "description": "UUID from pare_list_verifications"},
			},
			"required": []string{"document_id", "verification_id"},
		},
		proto: &documentAttachResult{},
		run:   runAttachDocumentToVerification,
	})
}

func runListDocuments(ctx context.Context, tc toolCtx, _ json.RawMessage) (any, error) {
	refs, err := tc.store.ListDocumentLinkRefs(ctx, tc.company)
	if err != nil {
		return nil, err
	}
	res := &documentRefResult{}
	for _, r := range refs {
		row := documentRefRow{
			ID: r.ID.String(), SHA256: r.SHA256, Mime: r.Mime, ByteSize: r.ByteSize,
			CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), AttachedTo: "none",
		}
		switch {
		case r.VerificationID != uuid.Nil:
			row.AttachedTo = "verification"
			row.TargetID = r.VerificationID.String()
		case r.SupplierInvoiceID != uuid.Nil:
			row.AttachedTo = "supplier_invoice"
			row.TargetID = r.SupplierInvoiceID.String()
		}
		res.Documents = append(res.Documents, row)
	}
	return res, nil
}

func runListVerifications(ctx context.Context, tc toolCtx, _ json.RawMessage) (any, error) {
	vers, err := tc.store.ListVerificationSummaries(ctx, tc.company)
	if err != nil {
		return nil, err
	}
	res := &verificationRefResult{}
	for _, v := range vers {
		res.Verifications = append(res.Verifications, verificationRefRow{
			ID: v.ID.String(), Series: v.Series, Number: v.Number, Date: v.Date,
		})
	}
	return res, nil
}

func runAttachDocumentToVerification(ctx context.Context, tc toolCtx, args json.RawMessage) (any, error) {
	var in struct {
		DocumentID     string `json:"document_id"`
		VerificationID string `json:"verification_id"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, errInvalidArgs
	}
	docID, err := uuid.Parse(in.DocumentID)
	if err != nil {
		return nil, errInvalidArgs
	}
	verificationID, err := uuid.Parse(in.VerificationID)
	if err != nil {
		return nil, errInvalidArgs
	}
	if err := tc.store.AttachDocumentToVerification(ctx, tc.company, docID, verificationID); err != nil {
		if errors.Is(err, store.ErrDocumentAttachConflict) {
			return &documentAttachResult{
				DocumentID: docID.String(), VerificationID: verificationID.String(), Attached: false,
				Note: "Underlaget är redan kopplat eller verifikatet saknas/tillhör inte företaget.",
			}, nil
		}
		return nil, err
	}
	return &documentAttachResult{DocumentID: docID.String(), VerificationID: verificationID.String(), Attached: true}, nil
}
