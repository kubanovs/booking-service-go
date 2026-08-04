-- +goose Up
CREATE TABLE IF NOT EXISTS bookings_log (
    id BIGSERIAL PRIMARY KEY,
    booking_id BIGINT NOT NULL,
    new_status VARCHAR(30) NOT NULL,
    previous_status VARCHAR(30) NOT NULL,
    event_timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cause VARCHAR(50),
    initiated_by VARCHAR(30) NOT NULL
);

COMMENT ON TABLE bookings_log IS 'Журнал изменений статусов бронирований';
COMMENT ON COLUMN bookings_log.id IS 'Идентификатор записи журнала';
COMMENT ON COLUMN bookings_log.booking_id IS 'Идентификатор бронирования, к которому относится запись';
COMMENT ON COLUMN bookings_log.new_status IS 'Новый статус бронирования после изменения';
COMMENT ON COLUMN bookings_log.previous_status IS 'Предыдущий статус бронирования до изменения';
COMMENT ON COLUMN bookings_log.event_timestamp IS 'Момент времени, когда произошло изменение статуса';
COMMENT ON COLUMN bookings_log.cause IS 'Причина изменения статуса';
COMMENT ON COLUMN bookings_log.initiated_by IS 'Инициатор изменения статуса (userId для пользователей или "System" для автоматических изменений)';

CREATE INDEX IF NOT EXISTS idx_bookings_log_booking_id ON bookings_log (booking_id);

-- +goose Down
DROP TABLE IF EXISTS bookings_log;
