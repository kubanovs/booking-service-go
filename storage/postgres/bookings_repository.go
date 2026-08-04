package postgres

import (
	"booking-service/app/models"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BookingsRepository реализует models.BookingRepository.
type BookingsRepository struct {
	pool *pgxpool.Pool
}

// New создаёт новый экземпляр BookingsRepository.
func NewBookingsRepository(pool *pgxpool.Pool) *BookingsRepository {
	return &BookingsRepository{pool: pool}
}

// CreateWithLog сохраняет новое бронирование и запись журнала о создании
// в рамках одной транзакции: либо создаётся всё, либо ничего.
func (r *BookingsRepository) CreateWithLog(ctx context.Context, b *models.Booking, initiatedBy string) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("старт транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // откат: no-op после успешного Commit

	var id int64
	err = tx.QueryRow(ctx, queryInsertBooking,
		string(b.Status()),
		b.UserID(),
		b.ResourceID(),
		b.StartDate(),
		b.EndDate(),
		b.CreatedAt(),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("создание бронирования: %w", err)
	}

	// Запись журнала о создании: предыдущего статуса нет (пустая строка),
	// время события совпадает с моментом создания брони.
	cause := models.CauseCreated
	if _, err := tx.Exec(ctx, queryInsertBookingLog,
		id,
		string(b.Status()),
		"",
		b.CreatedAt(),
		cause,
		initiatedBy,
	); err != nil {
		return 0, fmt.Errorf("запись в журнал о создании: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("коммит транзакции: %w", err)
	}
	return id, nil
}

// GetByID возвращает бронирование по ID.
func (r *BookingsRepository) GetByID(ctx context.Context, id int64) (*models.Booking, error) {
	b, err := r.scanBooking(r.pool.QueryRow(ctx, queryGetBookingByID, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrBookingNotFound
		}
		return nil, fmt.Errorf("получение бронирования id=%d: %w", id, err)
	}
	return b, nil
}

// GetByFilter возвращает бронирования с фильтрацией и пагинацией.
func (r *BookingsRepository) GetByFilter(ctx context.Context, filter models.BookingFilter) ([]models.Booking, int64, error) {
	offset := (filter.Page - 1) * filter.Size

	var userID, resourceID *int64
	var status *string
	if filter.UserID != nil {
		userID = filter.UserID
	}
	if filter.ResourceID != nil {
		resourceID = filter.ResourceID
	}
	if filter.Status != nil {
		s := string(*filter.Status)
		status = &s
	}

	// Получение общего количества
	var totalCount int64
	err := r.pool.QueryRow(ctx, queryCountBookingsByFilter, userID, resourceID, status).Scan(&totalCount)
	if err != nil {
		return nil, 0, fmt.Errorf("подсчёт бронирований: %w", err)
	}

	// Получение данных
	rows, err := r.pool.Query(ctx, queryGetBookingsByFilter, userID, resourceID, status, filter.Size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("получение бронирований по фильтру: %w", err)
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		b, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *b)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("итерация по строкам: %w", err)
	}

	return bookings, totalCount, nil
}

// GetAwaitingConfirmation возвращает бронирования, ожидающие подтверждения,
// с пессимистичной блокировкой FOR UPDATE SKIP LOCKED.
func (r *BookingsRepository) GetAwaitingConfirmation(ctx context.Context, limit int) ([]models.Booking, error) {
	rows, err := r.pool.Query(ctx, queryGetAwaitingConfirmation, limit)
	if err != nil {
		return nil, fmt.Errorf("получение бронирований для подтверждения: %w", err)
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		b, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *b)
	}

	return bookings, rows.Err()
}

// GetAwaitingCancellation возвращает бронирования в статусе CancellationPending,
// для которых с момента запроса отмены прошло не менее timeout.
func (r *BookingsRepository) GetAwaitingCancellation(ctx context.Context, limit int, timeout time.Duration) ([]models.Booking, error) {
	rows, err := r.pool.Query(ctx, queryGetAwaitingCancellation, limit, timeout.Microseconds())
	if err != nil {
		return nil, fmt.Errorf("получение бронирований для отмены: %w", err)
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		b, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("итерация по строкам: %w", err)
	}

	return bookings, nil
}

func (r *BookingsRepository) CountBookingsForPeriod(ctx context.Context, dateFrom time.Time, dateTo time.Time) (int, error) {
	rows, err := r.pool.Query(ctx, queryCountAllBookingsForPeriod, dateFrom, dateTo)
	if err != nil {
		return 0, fmt.Errorf("подсчет общего числа бронирований: %w", err)
	}
	defer rows.Close()

	totalCount, err := pgx.CollectOneRow(rows, pgx.RowTo[int])

	if err != nil {
		return 0, fmt.Errorf("маппинг сырой строки в число: %w", err)
	}

	return totalCount, nil
}

func (r *BookingsRepository) GetStatusCountsForPeriod(ctx context.Context, dateFrom time.Time, dateTo time.Time) (map[string]int, error) {
	rows, err := r.pool.Query(ctx, queryGetStatusCountsForPeriod, dateFrom, dateTo)

	if err != nil {
		return nil, fmt.Errorf("подсчет числа бронирований по статусам: %w", err)
	}

	defer rows.Close()

	stats := make(map[string]int)

	for rows.Next() {
		var status string
		var count int

		err := rows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("сканирование числа бронирований по статусам: %w", err)
		}

		stats[status] = count
	}

	return stats, rows.Err()
}

func (r *BookingsRepository) GetTopResourcesForPeriod(ctx context.Context, limit int, dateFrom time.Time, dateTo time.Time) ([]int, error) {
	rows, err := r.pool.Query(ctx, queryGetTopResourcesForPeriod, dateFrom, dateTo, limit)
	if err != nil {
		return nil, fmt.Errorf("получение топа ресурсов: %w", err)
	}
	defer rows.Close()

	resourceIDs, err := pgx.CollectRows(rows, pgx.RowTo[int])
	if err != nil {
		return nil, fmt.Errorf("маппинг сырых строк в список id ресурсов: %w", err)
	}

	return resourceIDs, nil
}

// scanBooking сканирует одну строку в доменный объект Booking.
func (r *BookingsRepository) scanBooking(row pgx.Row) (*models.Booking, error) {
	return scanBookingRow(row.Scan)
}

// scanBookingFromRows сканирует строку из pgx.Rows.
func (r *BookingsRepository) scanBookingFromRows(rows pgx.Rows) (*models.Booking, error) {
	return scanBookingRow(rows.Scan)
}

// scanBookingRow маппит одну строку БД на доменный объект Booking.
// Принимает функцию Scan, общую для pgx.Row и pgx.Rows.
func scanBookingRow(scan func(dest ...any) error) (*models.Booking, error) {
	var (
		id           int64
		status       string
		userID       int64
		resourceID   int64
		startDate    time.Time
		endDate      time.Time
		createdAt    time.Time
		statusBefore *string    // nullable
		cancelTS     *time.Time // nullable
	)

	err := scan(&id, &status, &userID, &resourceID, &startDate, &endDate, &createdAt,
		&statusBefore, &cancelTS)
	if err != nil {
		return nil, err
	}

	var sb *models.BookingStatus
	if statusBefore != nil {
		v := models.BookingStatus(*statusBefore)
		sb = &v
	}

	return models.RestoreBooking(
		id, models.BookingStatus(status), userID, resourceID,
		startDate, endDate, createdAt, sb, cancelTS,
	), nil
}

// UpdateWithLog обновляет бронирование и добавляет запись в журнал
// в рамках одной транзакции: либо применяется всё, либо ничего.
func (r *BookingsRepository) UpdateWithLog(ctx context.Context, b *models.Booking, log *models.EventLog) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("старт транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // откат: no-op после успешного Commit

	var statusBefore *string
	if s := b.StatusBeforeCancellation(); s != nil {
		v := string(*s)
		statusBefore = &v
	}

	tag, err := tx.Exec(ctx, queryUpdateBooking,
		string(b.Status()),
		statusBefore,
		b.RequestCancellationTimestamp(),
		b.UserID(),
		b.ResourceID(),
		b.StartDate(),
		b.EndDate(),
		b.ID(),
	)
	if err != nil {
		return fmt.Errorf("обновление бронирования id=%d: %w", b.ID(), err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrBookingNotFound
	}

	if _, err := tx.Exec(ctx, queryInsertBookingLog,
		log.BookingID(),
		string(log.NewStatus()),
		string(log.PreviousStatus()),
		log.EventTimestamp(),
		log.Cause(),
		log.InitiatedBy(),
	); err != nil {
		return fmt.Errorf("запись в журнал: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("коммит транзакции: %w", err)
	}
	return nil
}

// GetLogsByBookingID возвращает записи журнала по бронированию с пагинацией
// (от новых к старым) и их общее количество.
//
// COUNT и выборка страницы выполняются в одной read-only транзакции с изоляцией
// REPEATABLE READ, поэтому оба запроса видят единый снимок данных: total и список
// логов согласованы даже при конкурентной вставке новых записей журнала.
func (r *BookingsRepository) GetLogsByBookingID(ctx context.Context, bookingID int64, page, size int) ([]models.EventLog, int64, error) {
	offset := (page - 1) * size

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("старт транзакции журнала booking_id=%d: %w", bookingID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // откат: no-op после успешного Commit

	var total int64
	if err := tx.QueryRow(ctx, queryCountBookingLogsByBookingID, bookingID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("подсчёт записей журнала по booking_id=%d: %w", bookingID, err)
	}

	rows, err := tx.Query(ctx, queryGetBookingLogsByBookingID, bookingID, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("получение записей журнала по booking_id=%d: %w", bookingID, err)
	}

	logs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.EventLog, error) {
		var (
			id          int64
			bID         int64
			newStatus   string
			prevStatus  string
			eventTS     time.Time
			cause       *string
			initiatedBy string
		)
		if err := row.Scan(&id, &bID, &newStatus, &prevStatus, &eventTS, &cause, &initiatedBy); err != nil {
			return models.EventLog{}, err
		}
		return *models.NewEventLog(
			id, bID,
			models.BookingStatus(newStatus),
			models.BookingStatus(prevStatus),
			eventTS, cause, initiatedBy,
		), nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("сканирование записей журнала: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, 0, fmt.Errorf("коммит транзакции журнала booking_id=%d: %w", bookingID, err)
	}

	return logs, total, nil
}
