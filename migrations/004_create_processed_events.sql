-- +goose Up
CREATE TABLE IF NOT EXISTS processed_events (
    event_id     VARCHAR(64) PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE processed_events IS 'Обработанные события брокера для защиты идемпотентности (at-least-once delivery)';
COMMENT ON COLUMN processed_events.event_id IS 'EventId обработанного события; UNIQUE обеспечивает обработку ровно один раз';
COMMENT ON COLUMN processed_events.processed_at IS 'Момент фиксации обработки события';

-- +goose Down
DROP TABLE IF EXISTS processed_events;
