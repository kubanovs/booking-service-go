package models

import (
	"context"
	"time"
)

// BookingRepository -- интерфейс репозитория бронирований.
type BookingRepository interface {
	// CreateWithLog сохраняет новое бронирование и запись журнала о создании
	// в рамках одной транзакции. Возвращает присвоенный ID.
	CreateWithLog(ctx context.Context, booking *Booking, log *EventLog) (int64, error)

	// GetByID возвращает бронирование по ID.
	GetByID(ctx context.Context, id int64) (*Booking, error)

	// GetByFilter возвращает список бронирований с пагинацией.
	GetByFilter(ctx context.Context, filter BookingFilter) ([]Booking, int64, error)

	// GetAwaitingConfirmation возвращает бронирования в статусе AwaitsConfirmation
	// с пессимистичной блокировкой (SELECT ... FOR UPDATE SKIP LOCKED).
	GetAwaitingConfirmation(ctx context.Context, limit int) ([]Booking, error)

	// GetAwaitingCancellation возвращает бронирования в статусе CancellationPending,
	// для которых с момента запроса отмены прошло не менее timeout.
	// Сортировка — от самых давних запросов отмены.
	GetAwaitingCancellation(ctx context.Context, limit int, timeout time.Duration) ([]Booking, error)

	CountBookingsForPeriod(ctx context.Context, dateFrom time.Time, dateTo time.Time) (int, error)

	GetStatusCountsForPeriod(ctx context.Context, dateFrom time.Time, dateTo time.Time) (map[string]int, error)

	GetTopResourcesForPeriod(ctx context.Context, limit int, dateFrom time.Time, dateTo time.Time) ([]int, error)

	// IsEventProcessed сообщает, было ли событие с данным eventID уже обработано.
	IsEventProcessed(ctx context.Context, eventID string) (bool, error)

	// UpdateWithLog обновляет бронирование и добавляет запись в журнал в одной транзакции.
	// Непустой eventID означает обработку события брокера: в той же транзакции
	// делается claim eventID (защита идемпотентности при горизонтальном масштабировании).
	UpdateWithLog(ctx context.Context, booking *Booking, log *EventLog, eventID string) error

	// GetLogsByBookingID возвращает записи журнала по бронированию с пагинацией:
	// список записей и общее количество.
	GetLogsByBookingID(ctx context.Context, bookingID int64, page, size int) ([]EventLog, int64, error)
}

// BookingFilter содержит параметры фильтрации и пагинации.
type BookingFilter struct {
	UserID     *int64
	ResourceID *int64
	Status     *BookingStatus
	Page       int
	Size       int
}

// NewDefaultFilter создаёт фильтр с пагинацией по умолчанию.
func NewDefaultFilter() BookingFilter {
	return BookingFilter{
		Page: 1,
		Size: 25,
	}
}
