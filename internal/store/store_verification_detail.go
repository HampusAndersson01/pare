// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
// Copyright (c) Bright Interaction

package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bright-interaction/pare/internal/ledger"
)

// VerificationDetailView is one immutable verification together with the
// supporting documents attached directly to it.
type VerificationDetailView struct {
	Verification VerificationSummary
	Documents    []DocumentMeta
	ReversalOf   uuid.UUID
	ReversedBy   uuid.UUID
}

// VerificationDetail returns one verification and only documents attached to
// that verification, enforcing company scope before any metadata is decrypted.
func (s *Store) VerificationDetail(ctx context.Context, companyID, verificationID uuid.UUID) (VerificationDetailView, error) {
	v, err := s.q.GetVerification(ctx, verificationID)
	if err != nil {
		return VerificationDetailView{}, err
	}
	if v.CompanyID != companyID {
		return VerificationDetailView{}, ErrForeignCompany
	}

	out := VerificationDetailView{Verification: VerificationSummary{
		ID: verificationID, Series: v.Series, Number: int(v.Number),
		Date: v.Vdate.Time.Format("2006-01-02"), Description: v.Description,
		IsReversal: v.ReversalOf.Valid,
	}}
	if v.ReversalOf.Valid {
		out.ReversalOf = uuidFromPg(v.ReversalOf)
	}

	lines, err := s.q.ListVerificationLinesByVerification(ctx, verificationID)
	if err != nil {
		return VerificationDetailView{}, err
	}
	for _, l := range lines {
		out.Verification.Lines = append(out.Verification.Lines, VerificationLineView{
			Account: l.Account, Debit: ledger.Amount(l.DebitOre), Credit: ledger.Amount(l.CreditOre),
		})
	}

	var reversedBy uuid.UUID
	err = s.pool.QueryRow(ctx, `SELECT id FROM verifications WHERE company_id=$1 AND reversal_of=$2 LIMIT 1`, companyID, verificationID).Scan(&reversedBy)
	switch err {
	case nil:
		out.ReversedBy = reversedBy
		out.Verification.IsReversed = true
	case pgx.ErrNoRows:
	default:
		return VerificationDetailView{}, err
	}

	dek, err := s.companyDEK(ctx, companyID)
	if err != nil {
		return VerificationDetailView{}, err
	}
	rows, err := s.pool.Query(ctx, `
SELECT id, filename_enc, mime, byte_size, note_enc, created_at
FROM documents
WHERE company_id=$1 AND verification_id=$2
ORDER BY created_at ASC`, companyID, verificationID)
	if err != nil {
		return VerificationDetailView{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var d DocumentMeta
		var filenameEnc, noteEnc string
		if err := rows.Scan(&d.ID, &filenameEnc, &d.Mime, &d.ByteSize, &noteEnc, &d.CreatedAt); err != nil {
			return VerificationDetailView{}, err
		}
		d.Filename = decField(dek, filenameEnc)
		d.Note = decField(dek, noteEnc)
		d.Attached = true
		out.Documents = append(out.Documents, d)
	}
	if err := rows.Err(); err != nil {
		return VerificationDetailView{}, err
	}
	return out, nil
}
