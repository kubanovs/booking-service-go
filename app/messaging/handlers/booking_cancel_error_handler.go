package handlers

import (
	"booking-service/app/messaging"
	"booking-service/app/service"
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// CancelBookingErrorHandler обрабатывает события BookingJobDenied.
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

// Handle обрабатывает событие отклонения бронирования.
func (h *CancelBookingErrorHandler) Handle(ctx context.Context, body []byte) error {
	var event messaging.CancelBookingError
	if err := json.Unmarshal(body, &event); err != nil {
		return fmt.Errorf("десериализация CancelBookingError: %w", err)
	}

	bookingID, err := messaging.RequestIDToBookingID(event.RequestId)
	if err != nil {
		return fmt.Errorf("извлечение bookingId из RequestId: %w", err)
	}

	h.logger.Info("получено событие BookingJobDenied",
		zap.Int64("bookingId", bookingID),
		zap.Int64("catalogJobId", event.Id),
		zap.String("errorDescription", event.ErrorDesc),
	)

	if err := h.service.HandleCancelError(ctx, bookingID); err != nil {
		return fmt.Errorf("подтверждение отмены бронирования %d: %w", bookingID, err)
	}

	h.logger.Info("откат отмены",
		zap.Int64("bookingId", bookingID),
		zap.String("errorDesc", event.ErrorDesc),
	)
	return nil
}
