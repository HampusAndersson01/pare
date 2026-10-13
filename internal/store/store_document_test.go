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
	got, err := s.GetDocumentContent(ctx, co, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !bytes.Equal(got.Content, plain) || got.Filename != "kvitto.pdf" {
		t.Fatalf("round-trip mismatch")
	}
	var enc []byte
	_ = pool.QueryRow(ctx, "SELECT content_enc FROM documents WHERE id=$1", id).Scan(&enc)
	if bytes.Contains(enc, []byte("Anthropic")) || bytes.Contains(enc, []byte("%PDF")) {
		t.Fatalf("document stored in clear at rest")
	}
	metas, _ := s.ListDocuments(ctx, co)
	if len(metas) != 1 || metas[0].Attached {
		t.Fatalf("unexpected inbox: %+v", metas)
	}
	sup, _ := s.CreateCounterparty(ctx, co, Counterparty{Kind: "supplier", Name: "Anthropic PBC", OrgNr: "US-0"})
	inv, _ := s.CreateSupplierInvoice(ctx, co, sup, "INV-1", day("2026-03-01"), day("2026-03-31"), "", ledger.SEK(1000, 0), moms.PIMP, "API")
	if err := s.AttachDocumentToSupplier(ctx, co, id, inv); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if err := s.DeleteDocument(ctx, co, id); err != ErrDocumentAttached {
		t.Fatalf("attached document should be retained, got %v", err)
	}
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
	refs, err := s.ListDocumentLinkRefs(ctx, co)
	if err != nil || len(refs) != 1 || refs[0].VerificationID != verID {
		t.Fatalf("linked metadata mismatch: %+v err=%v", refs, err)
	}
	if err := s.DeleteDocument(ctx, co, docID); err != ErrDocumentAttached {
		t.Fatalf("verification document should be retained, got %v", err)
	}

	sup, _ := s.CreateCounterparty(ctx, co, Counterparty{Kind: "supplier", Name: "Supplier"})
	inv, _ := s.CreateSupplierInvoice(ctx, co, sup, "INV-2", day("2026-03-02"), day("2026-04-01"), "", ledger.SEK(100, 0), moms.PIMP, "x")
	if err := s.AttachDocumentToSupplier(ctx, co, docID, inv); err == nil {
		t.Fatal("conflicting supplier attachment should fail")
	}
	supplierDoc, _ := s.SaveDocument(ctx, co, "supplier.pdf", "application/pdf", []byte("%PDF supplier"), "")
	if err := s.AttachDocumentToSupplier(ctx, co, supplierDoc, inv); err != nil {
		t.Fatalf("attach supplier first: %v", err)
	}
	if err := s.AttachDocumentToVerification(ctx, co, supplierDoc, verID); err != ErrDocumentAttachConflict {
		t.Fatalf("supplier-linked document should reject verification link, got %v", err)
	}
	otherDoc, _ := s.SaveDocument(ctx, co, "other.pdf", "application/pdf", []byte("%PDF other"), "")
	if err := s.AttachDocumentToVerification(ctx, co, otherDoc, uuid.New()); err != ErrDocumentAttachConflict {
		t.Fatalf("unknown verification should fail, got %v", err)
	}
}

func TestVerificationDetailIncludesOnlyItsConnectedDocuments(t *testing.T) {
	s, pool := testStore(t)
	defer pool.Close()
	ctx := context.Background()
	co, _ := s.BootstrapCompany(ctx, "BI AB", "556000-0000")

	verID, err := s.PostVerification(ctx, co, "V", day("2026-10-13"), "OpenAI Plus", []ledger.Line{
		{Account: "4535", Debit: ledger.SEK(199, 20)},
		{Account: "2018", Credit: ledger.SEK(199, 20)},
	}, uuid.Nil)
	if err != nil {
		t.Fatalf("post verification: %v", err)
	}
	otherVerID, err := s.PostVerification(ctx, co, "V", day("2026-10-14"), "Other cost", []ledger.Line{
		{Account: "6540", Debit: ledger.SEK(100, 0)},
		{Account: "2018", Credit: ledger.SEK(100, 0)},
	}, uuid.Nil)
	if err != nil {
		t.Fatalf("post control verification: %v", err)
	}

	pdfID, _ := s.SaveDocument(ctx, co, "openai.pdf", "application/pdf", []byte("%PDF receipt"), "June")
	imageID, _ := s.SaveDocument(ctx, co, "receipt.jpg", "image/jpeg", []byte("jpeg"), "card receipt")
	otherID, _ := s.SaveDocument(ctx, co, "other.pdf", "application/pdf", []byte("%PDF other"), "other")
	if err := s.AttachDocumentToVerification(ctx, co, pdfID, verID); err != nil {
		t.Fatalf("attach pdf: %v", err)
	}
	if err := s.AttachDocumentToVerification(ctx, co, imageID, verID); err != nil {
		t.Fatalf("attach image: %v", err)
	}
	if err := s.AttachDocumentToVerification(ctx, co, otherID, otherVerID); err != nil {
		t.Fatalf("attach control document: %v", err)
	}

	detail, err := s.VerificationDetail(ctx, co, verID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if detail.Verification.ID != verID || detail.Verification.Series != "V" || detail.Verification.Number != 1 {
		t.Fatalf("unexpected verification: %+v", detail.Verification)
	}
	if len(detail.Documents) != 2 {
		t.Fatalf("documents = %d, want 2: %+v", len(detail.Documents), detail.Documents)
	}
	seen := map[uuid.UUID]bool{}
	for _, d := range detail.Documents {
		seen[d.ID] = true
	}
	if !seen[pdfID] || !seen[imageID] || seen[otherID] {
		t.Fatalf("wrong documents: %+v", seen)
	}
	if _, err := s.VerificationDetail(ctx, uuid.New(), verID); err != ErrForeignCompany {
		t.Fatalf("cross-company detail = %v, want ErrForeignCompany", err)
	}
}
