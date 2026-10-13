// SPDX-License-Identifier: LicenseRef-Pare-Sustainable-Use-License
package mcp

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/bright-interaction/pare/internal/store"
)

type counterpartyResult struct {
	CounterpartyID string `json:"counterparty_id,omitempty"`
	Created        bool   `json:"created"`
	DryRun         bool   `json:"dry_run"`
	Note           string `json:"note,omitempty"`
}

func (s *Server) registerCounterpartyTools() {
	s.add(tool{
		name:  "pare_create_counterparty",
		desc:  "Create a customer or supplier in Pare. Identity fields are encrypted at rest. The result never echoes identity data.",
		write: true,
		schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":    map[string]any{"type": "string", "enum": []string{"customer", "supplier"}, "description": "customer or supplier"},
				"name":    map[string]any{"type": "string"},
				"orgnr":   map[string]any{"type": "string"},
				"address": map[string]any{"type": "string"},
				"iban":    map[string]any{"type": "string"},
				"email":   map[string]any{"type": "string"},
				"dry_run": map[string]any{"type": "boolean", "description": "validate only; nothing is created"},
			},
			"required": []string{"kind", "name"},
		},
		proto: &counterpartyResult{},
		run:   runCreateCounterparty,
	})
}

func runCreateCounterparty(ctx context.Context, tc toolCtx, args json.RawMessage) (any, error) {
	var in struct {
		Kind    string `json:"kind"`
		Name    string `json:"name"`
		OrgNr   string `json:"orgnr"`
		Address string `json:"address"`
		IBAN    string `json:"iban"`
		Email   string `json:"email"`
		DryRun  bool   `json:"dry_run"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, errInvalidArgs
	}
	in.Kind = strings.TrimSpace(in.Kind)
	in.Name = strings.TrimSpace(in.Name)
	if (in.Kind != "customer" && in.Kind != "supplier") || in.Name == "" {
		return nil, errInvalidArgs
	}
	if in.DryRun {
		return &counterpartyResult{Created: false, DryRun: true, Note: "Giltig motpart. Inget skapades (dry_run)."}, nil
	}
	id, err := tc.store.CreateCounterparty(ctx, tc.company, store.Counterparty{
		Kind:    in.Kind,
		Name:    in.Name,
		OrgNr:   strings.TrimSpace(in.OrgNr),
		Address: strings.TrimSpace(in.Address),
		IBAN:    strings.TrimSpace(in.IBAN),
		Email:   strings.TrimSpace(in.Email),
	})
	if err != nil {
		return nil, err
	}
	return &counterpartyResult{CounterpartyID: id.String(), Created: true}, nil
}
