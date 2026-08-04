package dto

// EventLogResponse -- запись журнала изменений статуса бронирования в ответе API.
type EventLogResponse struct {
	ID             int64   `json:"id"`
	BookingID      int64   `json:"bookingId"`
	NewStatus      string  `json:"newStatus"`
	PreviousStatus string  `json:"previousStatus"` // пустая строка для события создания
	EventTimestamp string  `json:"eventTimestamp"` // формат: RFC3339
	Cause          *string `json:"cause,omitempty"`
	InitiatedBy    string  `json:"initiatedBy"` // userId инициатора или "System"
}
