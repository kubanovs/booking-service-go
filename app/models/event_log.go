package models

import (
	"time"

	"github.com/guregu/null/v5"
)

// InitiatorSystem -- значение initiatedBy для автоматических (не пользовательских)
// изменений статуса: воркеры и обработчики событий из брокера.
const InitiatorSystem = "System"

// Причины изменения статуса (колонка cause журнала).
const (
	CauseCreated          = "created"           // создание брони
	CauseUserRequest      = "user_request"      // отмена по запросу пользователя
	CauseCatalogConfirmed = "catalog_confirmed" // подтверждение от Catalog
	CauseCatalogDenied    = "catalog_denied"    // отказ Catalog
	CauseCancelConfirmed  = "cancel_confirmed"  // отмена подтверждена
	CauseCancelFailed     = "cancel_failed"     // откат неудавшейся отмены
)

// EventLog -- запись журнала изменений статусов бронирования.
type EventLog struct {
	Id             int64
	BookingID      int64
	NewStatus      BookingStatus
	PreviousStatus null.Value[BookingStatus] // NULL для события создания
	EventTimestamp time.Time
	Cause          null.String // NULL, если причина не указана
	InitiatedBy    string
}
