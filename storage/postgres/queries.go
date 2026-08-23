package postgres

const (
	queryInsertBooking = `
		INSERT INTO bookings (status, user_id, resource_id, start_date, end_date, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	queryGetBookingByID = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at,
		       status_before_cancellation, request_cancellation_timestamp
		FROM bookings
		WHERE id = $1`

	queryUpdateBooking = `
		UPDATE bookings
		SET status = $1,
		    status_before_cancellation = $2,
		    request_cancellation_timestamp = $3,
		    user_id = $4,
		    resource_id = $5,
		    start_date = $6,
		    end_date = $7
		WHERE id = $8`

	queryGetBookingsByFilter = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at,
		       status_before_cancellation, request_cancellation_timestamp
		FROM bookings
		WHERE ($1::BIGINT IS NULL OR user_id = $1)
		  AND ($2::BIGINT IS NULL OR resource_id = $2)
		  AND ($3::VARCHAR IS NULL OR status = $3)
		ORDER BY id DESC
		LIMIT $4 OFFSET $5`

	queryCountBookingsByFilter = `
		SELECT COUNT(*)
		FROM bookings
		WHERE ($1::BIGINT IS NULL OR user_id = $1)
		  AND ($2::BIGINT IS NULL OR resource_id = $2)
		  AND ($3::VARCHAR IS NULL OR status = $3)`

	queryGetAwaitingConfirmation = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at,
		       status_before_cancellation, request_cancellation_timestamp
		FROM bookings
		WHERE status = 'awaits_confirmation'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	queryGetAwaitingCancellation = `
		SELECT id, status, user_id, resource_id, start_date, end_date, created_at,
		       status_before_cancellation, request_cancellation_timestamp
		FROM bookings
		WHERE status = 'cancellation_pending'
		  AND request_cancellation_timestamp IS NOT NULL
		  AND request_cancellation_timestamp <= NOW() - ($2::bigint * INTERVAL '1 microsecond')
		ORDER BY request_cancellation_timestamp ASC
		LIMIT $1`

	queryCountAllBookingsForPeriod = `
		SELECT COUNT(*) AS total_bookings
		FROM bookings
		WHERE created_at >= $1 AND created_at < ($2::date + INTERVAL '1 day')`

	queryGetStatusCountsForPeriod = `
		SELECT status, COUNT(*) AS total_bookings
		FROM bookings
		WHERE created_at >= $1 AND created_at < ($2::date + INTERVAL '1 day')
		GROUP BY status`

	queryGetTopResourcesForPeriod = `
		SELECT resource_id
		FROM bookings
		WHERE created_at >= $1 AND created_at < ($2::date + INTERVAL '1 day')
		GROUP BY resource_id
		ORDER BY COUNT(*) DESC, resource_id
		LIMIT $3`

	queryInsertBookingLog = `
		INSERT INTO bookings_log (booking_id, new_status, previous_status, event_timestamp, cause, initiated_by)
		VALUES ($1, $2, $3, $4, $5, $6)`

	queryGetBookingLogsByBookingID = `
		SELECT id, booking_id, new_status, previous_status, event_timestamp, cause, initiated_by
		FROM bookings_log
		WHERE booking_id = $1
		ORDER BY event_timestamp DESC, id DESC
		LIMIT $2 OFFSET $3`

	queryCountBookingLogsByBookingID = `
		SELECT COUNT(*)
		FROM bookings_log
		WHERE booking_id = $1`

	queryEventExists = `
		SELECT EXISTS(SELECT 1 FROM processed_events WHERE event_id = $1)`

	// ON CONFLICT DO NOTHING: при гонке двух инстансов вставка второго вернёт
	// 0 строк вместо ошибки, что обрабатывается как "уже обработано".
	queryInsertProcessedEvent = `
		INSERT INTO processed_events (event_id) VALUES ($1)
		ON CONFLICT DO NOTHING`
)
