// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
// Copyright (c) Bright Interaction

package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ErrDocumentAttachConflict means the document or target cannot be linked safely:
// the document is already attached, the target verification does not belong to
// the company, or either id does not exist.
var ErrDocumentAttachConflict = errors.New("store: document cannot be attached to that verification")

// DocumentLinkRef is safe metadata for matching an uploaded document without
// exposing its filename, note or encrypted contents across the MCP boundary.
type DocumentLinkRef struct {
	ID                uuid.UUID
	SHA256            string
	Mime              string
	ByteSize          int64
	SupplierInvoiceID uuid.UUID
	VerificationID    uuid.UUID
	CreatedAt         time.Time
}

// AttachDocumentToVerification links one currently-unattached document to one
// existing verification in the same company. The guarded UPDATE makes the
// operation atomic and prevents a document from serving two accounting records.
func (s *Store) AttachDocumentToVerification(ctx context.Context, companyID, docID, verificationID uuid.UUID) error {
	res, err := s.pool.Exec(ctx, `
UPDATE documents d
SET verification_id = $3
WHERE d.id = $1
  AND d.company_id = $2
  AND d.supplier_invoice_id IS NULL
  AND d.verification_id IS NULL
  AND EXISTS (
    SELECT 1 FROM verifications v
    WHERE v.id = $3 AND v.company_id = $2
  )`, docID, companyID, verificationID)
	if err != nil {
		return err
	}
	if res.RowsAffected() != 1 {
		return ErrDocumentAttachConflict
	}
	return s.logAudit(ctx, s.q, companyID, "attach_document", "verification", verificationID.String(), docID.String())
}

// ListDocumentLinkRefs returns only non-PII document metadata. SHA-256 lets an
// assistant match a receipt already supplied by the user to its encrypted Pare
// copy without exposing the receipt or its identifying filename/note.
func (s *Store) ListDocumentLinkRefs(ctx context.Context, companyID uuid.UUID) ([]DocumentLinkRef, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, sha256, mime, byte_size, supplier_invoice_id, verification_id, created_at
FROM documents
WHERE company_id = $1
ORDER BY created_at DESC`, companyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DocumentLinkRef
	for rows.Next() {
		var r DocumentLinkRef
		var supplier, verification pgtype.UUID
		var created pgtype.Timestamptz
		if err := rows.Scan(&r.ID, &r.SHA256, &r.Mime, &r.ByteSize, &supplier, &verification, &created); err != nil {
			return nil, err
		}
		r.SupplierInvoiceID = uuidFromPg(supplier)
		r.VerificationID = uuidFromPg(verification)
		r.CreatedAt = created.Time
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
