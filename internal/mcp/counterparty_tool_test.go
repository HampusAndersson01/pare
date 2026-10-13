// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package mcp

import (
	"bytes"
	"testing"
)

func TestCreateCounterpartyTool(t *testing.T) {
	h, done := setup(t)
	defer done()

	list := call(t, h, testKey, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	if list.Code != 200 {
		t.Fatalf("tools/list code %d", list.Code)
	}
	if !bytes.Contains(list.Body.Bytes(), []byte("pare_create_counterparty")) {
		t.Fatalf("tools/list missing pare_create_counterparty: %s", list.Body.String())
	}

	rec := call(t, h, testKey, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"pare_create_counterparty","arguments":{"kind":"supplier","name":"Example Supplier AB","orgnr":"556123-4567","address":"Testgatan 1, 111 11 Stockholm","email":"billing@example.test"}}}`)
	if rec.Code != 200 {
		t.Fatalf("create code %d: %s", rec.Code, rec.Body.String())
	}
	text := toolText(t, rec.Body.Bytes())
	if !bytes.Contains([]byte(text), []byte(`"created": true`)) || !bytes.Contains([]byte(text), []byte(`"counterparty_id"`)) {
		t.Fatalf("unexpected create result: %s", text)
	}
	if bytes.Contains([]byte(text), []byte("Example Supplier")) || bytes.Contains([]byte(text), []byte("556123")) {
		t.Fatalf("counterparty identity leaked in result: %s", text)
	}
}

func TestCreateCounterpartyRejectsInvalidKind(t *testing.T) {
	h, done := setup(t)
	defer done()

	rec := call(t, h, testKey, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"pare_create_counterparty","arguments":{"kind":"other","name":"Nope"}}}`)
	if rec.Code != 200 {
		t.Fatalf("create code %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"isError":true`)) {
		t.Fatalf("invalid kind was not rejected: %s", rec.Body.String())
	}
}
