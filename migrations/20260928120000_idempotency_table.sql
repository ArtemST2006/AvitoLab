-- +goose Up
-- +goose StatementBegin
CREATE TABLE idempotency (
    key        UUID PRIMARY KEY,
    body_hash  TEXT NOT NULL,
    trip_id    UUID REFERENCES trips(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idem_created_at_idx ON idempotency (created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idem_created_at_idx;
DROP TABLE IF EXISTS idempotency;
-- +goose StatementEnd
