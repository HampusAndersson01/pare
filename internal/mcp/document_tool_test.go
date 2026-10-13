// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bright-interaction/pare/internal/crypto"
	"github.com/bright-interaction/pare/internal/ledger"
	"github.com/bright-interaction/pare/internal/store"
	"github.com/bright-interaction/pare/internal/testdb"
)

func setupDocumentTools(t *testing.T) (http.Handler, *store.Store, uuid.UUID, func()) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testdb.New(t, "mcp-docs"))
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	testdb.Reset(t, pool)
	ctx := context.Background()
	key, _ := crypto.NewDEK()
	kek, _ := crypto.NewKEK(key)
	st := store.New(pool, kek)
	co, err := st.BootstrapCompany(ctx, "BI AB", "556000-0000")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	shieldKey, _ := crypto.NewDEK()
	srv, err := New(st, pool, shieldKey, testKey, 0)
	if err != nil {
		t.Fatalf("mcp: %v", err)
	}
	return srv.Handler(), st, co, func() { pool.Close() }
}

func TestDocumentToolsRegistered(t *testing.T) {
	h, _, _, done := setupDocumentTools(t)
	defer done()
	rec := call(t, h, testKey, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	body := rec.Body.Bytes()
	for _, name := range [][]byte{[]byte("pare_list_documents"), []byte("pare_list_verifications"), []byte("pare_attach_document_to_verification")} {
		if !bytes.Contains(body, name) {
			t.Fatalf("tools/list missing %s: %s", name, body)
		}
	}
}

func TestAttachDocumentToVerificationTool(t *testing.T) {
	h, st, co, done := setupDocumentTools(t)
	defer done()
	ctx := context.Background()
	docID, err := st.SaveDocument(ctx, co, "secret-name.pdf", "application/pdf", []byte("%PDF hello"), "private note")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	verID, err := st.PostVerification(ctx, co, "V", day("2026-10-13"), "private description", []ledger.Line{
		{Account: "6540", Debit: ledger.SEK(10, 0)},
		{Account: "2018", Credit: ledger.SEK(10, 0)},
	}, uuid.Nil)
	if err != nil {
		t.Fatalf("post: %v", err)
	}

	args, _ := json.Marshal(map[string]string{"document_id": docID.String(), "verification_id": verID.String()})
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 2, "method": "tools/call",
		"params": map[string]any{"name": "pare_attach_document_to_verification", "arguments": json.RawMessage(args)},
	})
	rec := call(t, h, testKey, string(body))
	text := toolText(t, rec.Body.Bytes())
	if !bytes.Contains([]byte(text), []byte(`"attached": true`)) {
		t.Fatalf("attach result: %s", text)
	}

	// The list tool exposes only non-PII metadata: never filename, note or free-text verification description.
	rec = call(t, h, testKey, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"pare_list_documents","arguments":{}}}`)
	text = toolText(t, rec.Body.Bytes())
	if bytes.Contains([]byte(text), []byte("secret-name")) || bytes.Contains([]byte(text), []byte("private note")) {
		t.Fatalf("document PII leaked: %s", text)
	}
	if !bytes.Contains([]byte(text), []byte(docID.String())) || !bytes.Contains([]byte(text), []byte("sha256")) {
		t.Fatalf("safe document reference missing: %s", text)
	}

	rec = call(t, h, testKey, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"pare_list_verifications","arguments":{}}}`)
	text = toolText(t, rec.Body.Bytes())
	if bytes.Contains([]byte(text), []byte("private description")) {
		t.Fatalf("verification free text leaked: %s", text)
	}
	if !bytes.Contains([]byte(text), []byte(verID.String())) || !bytes.Contains([]byte(text), []byte(`"number": 1`)) {
		t.Fatalf("verification ref missing: %s", text)
	}
}
