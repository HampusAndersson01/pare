// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package store

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/bright-interaction/pare/internal/ledger"
	"github.com/bright-interaction/pare/internal/moms"
)

// A receipt is encrypted at rest, round-trips on read, and an attached one
// (verifikationsunderlag) cannot be deleted.
func TestDocumentEncryptedAndRetention(t *testing.T) {
	s, pool := testStore(t)
	defer pool.Close()
	ctx := context.Background()
	co, _ := s.BootstrapCompany(ctx, "BI AB", "556000-0000")

	plain := []byte("%PDF-1.4 secret receipt from Anthropic PBC 100 USD")
	id, err := s.SaveDocument(ctx, co, "kvitto.pdf", "application/pdf", plain, "Anthropic mars")
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// Round-trips on read.
	got, err := s.GetDocumentContent(ctx, co, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(got.Content, plain) || got.Filename != "kvitto.pdf" {
		t.Fatalf("round-trip mismatch")
	}

	// The stored blob must be ciphertext (no plaintext at rest).
	var enc []byte
	_ = pool.QueryRow(ctx, "SELECT content_enc FROM documents WHERE id=$1", id).Scan(&enc)
	if bytes.Contains(enc, []byte("Anthropic")) || bytes.Contains(enc, []byte("%PDF")) {
		t.Fatalf("document stored in clear at rest")
	}

	// List returns metadata, not the blob.
	metas, _ := s.ListDocuments(ctx, co)
	if len(metas) != 1 || metas[0].Attached {
		t.Fatalf("unexpected inbox: %+v", metas)
	}

	// Attach to a supplier invoice -> becomes underlag -> cannot be deleted.
	sup, _ := s.CreateCounterparty(ctx, co, Counterparty{Kind: "supplier", Name: "Anthropic PBC", OrgNr: "US-0"})
	inv, _ := s.CreateSupplierInvoice(ctx, co, sup, "INV-1", day("2026-03-01"), day("2026-03-31"), "", ledger.SEK(1000, 0), moms.PIMP, "API")
	if err := s.AttachDocumentToSupplier(ctx, co, id, inv); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := s.DeleteDocument(ctx, co, id); err != ErrDocumentAttached {
		t.Fatalf("attached document should be retained, got %v", err)
	}

	// Cross-company read is refused.
	if _, err := s.GetDocumentContent(ctx, uuid.New(), id); err != ErrForeignCompany {
		t.Fatalf("want ErrForeignCompany, got %v", err)
	}
}

func TestAttachDocumentToVerification(t *testing.T) {
	s, pool := testStore(t)
	defer pool.Close()
	ctx := context.Background()
	co, _ := s.BootstrapCompany(ctx, "BI AB", "556000-0000")

	docID, err := s.SaveDocument(ctx, co, "receipt.pdf", "application/pdf", []byte("%PDF receipt"), "server")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	verID, err := s.PostVerification(ctx, co, "V", day("2026-03-02"), "Serverel", []ledger.Line{
		{Account: "5090", Debit: ledger.SEK(100, 0)},
		{Account: "2018", Credit: ledger.SEK(100, 0)},
	}, uuid.Nil)
	if err != nil {
		t.Fatalf("post verification: %v", err)
	}

	if err := s.AttachDocumentToVerification(ctx, co, docID, verID); err != nil {
		t.Fatalf("attach verification: %v", err)
	}
	var got uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT verification_id FROM documents WHERE id=$1", docID).Scan(&got); err != nil {
		t.Fatalf("query link: %v", err)
	}
	if got != verID {
		t.Fatalf("verification link = %s, want %s", got, verID)
	}
	metas, err := s.ListDocuments(ctx, co)
	if err != nil || len(metas) != 1 || !metas[0].Attached || metas[0].VerificationID != verID {
		t.Fatalf("linked metadata mismatch: %+v err=%v", metas, err)
	}
	if err := s.DeleteDocument(ctx, co, docID); err != ErrDocumentAttached {
		t.Fatalf("verification document should be retained, got %v", err)
	}

	// A linked document cannot simultaneously become supplier-invoice evidence.
	sup, _ := s.CreateCounterparty(ctx, co, Counterparty{Kind: "supplier", Name: "Supplier"})
	inv, _ := s.CreateSupplierInvoice(ctx, co, sup, "INV-2", day("2026-03-02"), day("2026-04-01"), "", ledger.SEK(100, 0), moms.PIMP, "x")
	if err := s.AttachDocumentToSupplier(ctx, co, docID, inv); err != ErrDocumentAttachConflict {
		t.Fatalf("conflicting attachment should fail, got %v", err)
	}

	// Cross-company / unknown verification must not create a link.
	otherDoc, _ := s.SaveDocument(ctx, co, "other.pdf", "application/pdf", []byte("%PDF other"), "")
	if err := s.AttachDocumentToVerification(ctx, co, otherDoc, uuid.New()); err != ErrDocumentAttachConflict {
		t.Fatalf("unknown verification should fail, got %v", err)
	}
}
