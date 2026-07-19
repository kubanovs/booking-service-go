-- +goose Up
-- +goose StatementBegin
ALTER TABLE bookings
ADD COLUMN status_before_cancellation VARCHAR(30)
ADD COLUMN request_cancellation_timestamp TIMESTAMPTZ;
-- +goose StatementEnd
