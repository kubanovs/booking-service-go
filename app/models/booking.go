package models

import "time"

// BookingStatus представляет статус бронирования.
type BookingStatus string

const (
	BookingStatusAwaitsConfirmation  BookingStatus = "awaits_confirmation"
	BookingStatusConfirmed           BookingStatus = "confirmed"
	BookingStatusCancelled           BookingStatus = "cancelled"
	BookingStatusCancellationPending BookingStatus = "cancellation_pending"
)

// AllBookingStatuses возвращает все допустимые статусы бронирования.
func AllBookingStatuses() []BookingStatus {
	return []BookingStatus{
		BookingStatusAwaitsConfirmation,
		BookingStatusConfirmed,
		BookingStatusCancelled,
		BookingStatusCancellationPending,
	}
}

// IsValid проверяет, что статус принадлежит допустимому множеству.
func (s BookingStatus) IsValid() bool {
	switch s {
	case BookingStatusAwaitsConfirmation, BookingStatusConfirmed, BookingStatusCancelled, BookingStatusCancellationPending:
		return true
	default:
		return false
	}
}

// Booking -- доменная сущность бронирования.
// Поля неэкспортируемые для обеспечения инкапсуляции.
type Booking struct {
	id                           int64
	status                       BookingStatus
	statusBeforeCancellation     *BookingStatus // nil, если бронь не в процессе отмены
	userID                       int64
	resourceID                   int64
	startDate                    time.Time
	endDate                      time.Time
	createdAt                    time.Time
	requestCancellationTimestamp *time.Time // nil, если бронь не в процессе отмены
}

func (b *Booking) ID() int64                                { return b.id }
func (b *Booking) Status() BookingStatus                    { return b.status }
func (b *Booking) UserID() int64                            { return b.userID }
func (b *Booking) ResourceID() int64                        { return b.resourceID }
func (b *Booking) StartDate() time.Time                     { return b.startDate }
func (b *Booking) EndDate() time.Time                       { return b.endDate }
func (b *Booking) CreatedAt() time.Time                     { return b.createdAt }
func (b *Booking) StatusBeforeCancellation() *BookingStatus { return b.statusBeforeCancellation }
func (b *Booking) RequestCancellationTimestamp() *time.Time { return b.requestCancellationTimestamp }

// NewBooking создаёт новое бронирование в статусе AwaitsConfirmation.
func NewBooking(userID, resourceID int64, startDate, endDate time.Time) (*Booking, error) {
	if userID <= 0 {
		return nil, ErrInvalidUserID
	}
	if resourceID <= 0 {
		return nil, ErrInvalidResourceID
	}
	if startDate.IsZero() || endDate.IsZero() {
		return nil, ErrInvalidDateRange
	}
	if !endDate.After(startDate) {
		return nil, ErrEndDateBeforeStartDate
	}

	return &Booking{
		status:     BookingStatusAwaitsConfirmation,
		userID:     userID,
		resourceID: resourceID,
		startDate:  startDate,
		endDate:    endDate,
		createdAt:  time.Now(),
	}, nil
}

// Confirm подтверждает бронирование.
// Допустимый переход: AwaitsConfirmation || CancellationPending -> Confirmed.
func (b *Booking) Confirm() error {
	if b.status != BookingStatusAwaitsConfirmation && b.status != BookingStatusCancellationPending {
		return ErrInvalidStatusTransition
	}
	b.status = BookingStatusConfirmed
	return nil
}

// StartCancel начинает процесс отмены.
// Допустимые переходы:
//   - AwaitsConfirmation -> CancellationPending
//   - Confirmed -> CancellationPending (только если StartDate > today)
func (b *Booking) StartCancel(today time.Time) error {
	switch b.status {
	case BookingStatusAwaitsConfirmation:
		b.beginCancellation(today)
		return nil
	case BookingStatusConfirmed:
		if !b.startDate.After(today) {
			return ErrCannotCancelPastBooking
		}
		b.beginCancellation(today)
		return nil
	case BookingStatusCancelled:
		return ErrInvalidStatusTransition
	default:
		return ErrInvalidStatusTransition
	}
}

// beginCancellation запоминает текущий статус и момент запроса,
// затем переводит бронь в промежуточный статус CancellationPending.
func (b *Booking) beginCancellation(today time.Time) {
	prev := b.status
	t := today
	b.statusBeforeCancellation = &prev
	b.requestCancellationTimestamp = &t
	b.status = BookingStatusCancellationPending
}

// RollbackCancel откатывает статус CancellationPending к предыдущему.
// Метаданные отмены сбрасываются в nil (в БД -> NULL): бронь снова активна.
func (b *Booking) RollbackCancel() error {
	switch b.status {
	case BookingStatusCancellationPending:
		if b.statusBeforeCancellation == nil {
			return ErrInvalidStatusTransition
		}
		b.status = *b.statusBeforeCancellation
		b.statusBeforeCancellation = nil
		b.requestCancellationTimestamp = nil
		return nil
	default:
		return ErrInvalidStatusTransition
	}
}

// FinishCancel переводит статус в Cancelled
func (b *Booking) FinishCancel() error {
	switch b.status {
	case BookingStatusCancellationPending:
		b.status = BookingStatusCancelled
		b.statusBeforeCancellation = nil
		b.requestCancellationTimestamp = nil
		return nil
	default:
		return ErrInvalidStatusTransition
	}
}

// RestoreBooking восстанавливает Booking из данных хранилища.
// Используется только в слое storage для маппинга строк БД на доменный объект.
func RestoreBooking(
	id int64,
	status BookingStatus,
	userID, resourceID int64,
	startDate, endDate, createdAt time.Time,
	statusBeforeCancellation *BookingStatus,
	requestCancellationTimestamp *time.Time,
) *Booking {
	return &Booking{
		id:                           id,
		status:                       status,
		userID:                       userID,
		resourceID:                   resourceID,
		startDate:                    startDate,
		endDate:                      endDate,
		createdAt:                    createdAt,
		statusBeforeCancellation:     statusBeforeCancellation,
		requestCancellationTimestamp: requestCancellationTimestamp,
	}
}
