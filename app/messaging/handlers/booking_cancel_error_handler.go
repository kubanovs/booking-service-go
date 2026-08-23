package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"booking-service/app/messaging"
	"booking-service/app/models"
	"booking-service/app/service"
)

// CancelBookingErrorHandler обрабатывает события CancelBookingError.
type CancelBookingErrorHandler struct {
	service *service.BookingsService
	logger  *zap.Logger
}

// NewCancelBookingErrorHandler создаёт новый обработчик.
func NewCancelBookingErrorHandler(svc *service.BookingsService, logger *zap.Logger) *CancelBookingErrorHandler {
	return &CancelBookingErrorHandler{
		service: svc,
		logger:  logger,
	}
}

// Handle обрабатывает событие ошибки отмены бронирования.
func (h *CancelBookingErrorHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.CancelBookingError
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация CancelBookingError: %w", err)
	}

	bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
	if err != nil {
		return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
	}

	h.logger.Info("получено событие CancelBookingError",
		zap.Int64("bookingId", bookingID),
		zap.Int64("catalogJobId", event.Id),
		zap.String("errorDescription", event.ErrorDesc),
	)

	if err := h.service.HandleCancelError(ctx, bookingID, event.EventId); err != nil {
		if errors.Is(err, models.ErrEventAlreadyProcessed) {
			h.logger.Warn("дубликат события CancelBookingError, пропускаем",
				zap.String("eventId", event.EventId),
				zap.Int64("bookingId", bookingID),
			)
			return nil
		}
		return fmt.Errorf("откат отмены бронирования %d: %w", bookingID, err)
	}

	h.logger.Info("откат отмены",
		zap.Int64("bookingId", bookingID),
		zap.String("errorDesc", event.ErrorDesc),
	)
	return nil
}
