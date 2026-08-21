-- +goose Up
-- +goose StatementBegin
ALTER TABLE bookings
ADD COLUMN status_before_cancellation VARCHAR(30),
ADD COLUMN request_cancellation_timestamp TIMESTAMPTZ;
-- +goose StatementEnd
-- +goose Down
ALTER TABLE bookings
DROP COLUMN IF EXISTS status_before_cancellation,
DROP COLUMN IF EXISTS request_cancellation_timestamp;