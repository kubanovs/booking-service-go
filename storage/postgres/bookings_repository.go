package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"booking-service/app/models"
)

// BookingsRepository реализует models.BookingRepository.
type BookingsRepository struct {
	pool *pgxpool.Pool
}

// NewBookingsRepository создаёт новый экземпляр BookingsRepository.
func NewBookingsRepository(pool *pgxpool.Pool) *BookingsRepository {
	return &BookingsRepository{pool: pool}
}

// Create сохраняет новое бронирование.
func (r *BookingsRepository) Create(ctx context.Context, booking *models.Booking) (int64, error) {
	var id int64
	err := r.pool.QueryRow(ctx, queryInsertBooking,
		string(booking.Status()),
		booking.UserID(),
		booking.ResourceID(),
		booking.StartDate(),
		booking.EndDate(),
		booking.CreatedAt(),
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("создание бронирования: %w", err)
	}
	return id, nil
}

// GetByID возвращает бронирование по ID.
func (r *BookingsRepository) GetByID(ctx context.Context, id int64) (*models.Booking, error) {
	booking, err := r.scanBooking(r.pool.QueryRow(ctx, queryGetBookingByID, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrBookingNotFound
		}
		return nil, fmt.Errorf("получение бронирования id=%d: %w", id, err)
	}
	return booking, nil
}

// Update обновляет все изменяемые поля бронирования.
func (r *BookingsRepository) Update(ctx context.Context, booking *models.Booking) error {
	// nil-указатели уходят в БД как NULL напрямую, без «угадывания» по нулям.
	var statusBefore *string
	if s := booking.StatusBeforeCancellation(); s != nil {
		v := string(*s)
		statusBefore = &v
	}

	tag, err := r.pool.Exec(ctx, queryUpdateBooking,
		string(booking.Status()),
		statusBefore,
		booking.RequestCancellationTimestamp(),
		booking.UserID(),
		booking.ResourceID(),
		booking.StartDate(),
		booking.EndDate(),
		booking.ID(),
	)
	if err != nil {
		return fmt.Errorf("обновление бронирования id=%d: %w", booking.ID(), err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrBookingNotFound
	}
	return nil
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
		booking, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *booking)
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
		booking, err := r.scanBookingFromRows(rows)
		if err != nil {
			return nil, fmt.Errorf("сканирование бронирования: %w", err)
		}
		bookings = append(bookings, *booking)
	}

	return bookings, rows.Err()
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
