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

// CancelBookingConfirmationHandler обрабатывает события CancelBookingConfirmation.
type CancelBookingConfirmationHandler struct {
	service *service.BookingsService
	logger  *zap.Logger
}

// NewCancelBookingConfirmationHandler создаёт новый обработчик.
func NewCancelBookingConfirmationHandler(svc *service.BookingsService, logger *zap.Logger) *CancelBookingConfirmationHandler {
	return &CancelBookingConfirmationHandler{
		service: svc,
		logger:  logger,
	}
}

// Handle обрабатывает событие подтверждения отмены бронирования.
func (h *CancelBookingConfirmationHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.CancelBookingConfirmation
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация CancelBookingConfirmation: %w", err)
	}

	bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
	if err != nil {
		return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
	}

	h.logger.Info("получено событие CancelBookingConfirmation",
		zap.Int64("bookingId", bookingID),
		zap.Int64("catalogJobId", event.Id),
	)

	if err := h.service.HandleConfirmCancel(ctx, bookingID, event.EventId); err != nil {
		if errors.Is(err, models.ErrEventAlreadyProcessed) {
			h.logger.Warn("дубликат события CancelBookingConfirmation, пропускаем",
				zap.String("eventId", event.EventId),
				zap.Int64("bookingId", bookingID),
			)
			return nil
		}
		return fmt.Errorf("подтверждение отмены бронирования %d: %w", bookingID, err)
	}

	h.logger.Info("бронирование отменено", zap.Int64("bookingId", bookingID))
	return nil
}
