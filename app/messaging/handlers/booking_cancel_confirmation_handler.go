package handlers

import (
	"booking-service/app/messaging"
	"booking-service/app/service"
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

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

// Handle обрабатывает событие отклонения бронирования.
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

	if err := h.service.HandleConfirmCancel(ctx, bookingID); err != nil {
		return fmt.Errorf("подтверждение отмены бронирования %d: %w", bookingID, err)
	}

	h.logger.Info("бронирование отменено", zap.Int64("bookingId", bookingID))
	return nil
}
