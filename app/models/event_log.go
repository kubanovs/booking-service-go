package models

import "time"

// InitiatorSystem -- значение initiatedBy для автоматических (не пользовательских)
// изменений статуса: воркеры и обработчики событий из брокера.
const InitiatorSystem = "System"

// EventLog -- запись журнала изменений статусов бронирования.
type EventLog struct {
	id             int64
	bookingID      int64
	newStatus      BookingStatus
	previousStatus BookingStatus
	eventTimestamp time.Time
	cause          *string // nil, если причина не указана
	initiatedBy    string  // userId пользователя или "System" для автоматических изменений
}

// NewEventLog восстанавливает EventLog из данных хранилища (все поля известны).
func NewEventLog(
	id int64,
	bookingID int64,
	newStatus BookingStatus,
	previousStatus BookingStatus,
	eventTimestamp time.Time,
	cause *string,
	initiatedBy string,
) *EventLog {
	return &EventLog{
		id:             id,
		bookingID:      bookingID,
		newStatus:      newStatus,
		previousStatus: previousStatus,
		eventTimestamp: eventTimestamp,
		cause:          cause,
		initiatedBy:    initiatedBy,
	}
}

// RecordEventLog создаёт новую запись журнала для фиксации перехода статуса.
// id присваивается хранилищем, время проставляется текущим моментом.
func RecordEventLog(
	bookingID int64,
	previousStatus BookingStatus,
	newStatus BookingStatus,
	initiatedBy string,
	cause *string,
) *EventLog {
	return &EventLog{
		bookingID:      bookingID,
		newStatus:      newStatus,
		previousStatus: previousStatus,
		eventTimestamp: time.Now(),
		cause:          cause,
		initiatedBy:    initiatedBy,
	}
}

func (e *EventLog) ID() int64                     { return e.id }
func (e *EventLog) BookingID() int64              { return e.bookingID }
func (e *EventLog) NewStatus() BookingStatus      { return e.newStatus }
func (e *EventLog) PreviousStatus() BookingStatus { return e.previousStatus }
func (e *EventLog) EventTimestamp() time.Time     { return e.eventTimestamp }
func (e *EventLog) Cause() *string                { return e.cause }
func (e *EventLog) InitiatedBy() string           { return e.initiatedBy }
