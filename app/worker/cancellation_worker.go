package worker

import (
	"booking-service/app/messaging"
	"booking-service/app/models"
	"context"
	"time"

	"go.uber.org/zap"
)

// CancellationWorker -- фоновый процесс переотправки сообщений для бронирований в статусе cancellation_pending
//
// Логика работы
// 1. Получить бронирования в статусе cancellation_pending, для которых истёк timeout
// 2. Переотправить сообщение об отмене в Catalog
type CancellationWorker struct {
	publisher *messaging.Publisher
	repo      models.BookingRepository
	interval  time.Duration
	batchSize int
	timeout   time.Duration
	logger    *zap.Logger
}

func NewCancellationWorker(
	publisher *messaging.Publisher,
	repo models.BookingRepository,
	interval time.Duration,
	batchSize int,
	timeout time.Duration,
	logger *zap.Logger,
) *CancellationWorker {
	return &CancellationWorker{
		publisher: publisher,
		repo:      repo,
		interval:  interval,
		batchSize: batchSize,
		timeout:   timeout,
		logger:    logger.With(TypeCancellation.LogField()),
	}
}

func (p *CancellationWorker) Run(ctx context.Context) {

	p.logger.Info("воркер отмены запущен",
		zap.Duration("interval", p.interval),
		zap.Int("batchSize", p.batchSize),
	)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("воркер отмены остановлен")
			return
		case <-ticker.C:
			p.processBatch(ctx)
		}
	}
}

func (p *CancellationWorker) processBatch(ctx context.Context) {
	bookings, err := p.repo.GetAwaitingCancellation(ctx, p.batchSize, p.timeout)
	if err != nil {
		p.logger.Error("ошибка получения бронирований для повторной отмены", zap.Error(err))
		return
	}

	if len(bookings) == 0 {
		return
	}

	p.logger.Info("обработка бронирований", zap.Int("count", len(bookings)))

	for _, booking := range bookings {
		p.processBooking(ctx, &booking)
	}
}

func (p *CancellationWorker) processBooking(ctx context.Context, booking *models.Booking) {
	bookingID := booking.ID()
	logger := p.logger.With(zap.Int64("bookingId", bookingID))

	if err := p.publisher.PublishCancelBookingJob(ctx, messaging.CancelBookingJobCommand{
		EventId:   messaging.NewMessageID(),
		RequestId: messaging.BookingIDToRequestID(bookingID),
	}); err != nil {
		logger.Error("ошибка публикации CancelBookingJob", zap.Error(err))
		return
	}

	logger.Info("повторно отправлено сообщение об отмене в Catalog")
}
