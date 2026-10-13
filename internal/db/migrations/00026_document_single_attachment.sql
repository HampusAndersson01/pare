-- +goose Up
-- +goose StatementBegin
-- One receipt/document may support either a supplier invoice or a direct
-- verification, never both. Existing rows already use at most one side.
ALTER TABLE documents
ADD CONSTRAINT documents_single_attachment
CHECK (NOT (supplier_invoice_id IS NOT NULL AND verification_id IS NOT NULL));
CREATE INDEX IF NOT EXISTS idx_documents_verification ON documents (verification_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_documents_verification;
ALTER TABLE documents DROP CONSTRAINT IF EXISTS documents_single_attachment;
-- +goose StatementEnd
