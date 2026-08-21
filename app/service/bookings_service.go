package service

import (
	"booking-service/app/models"
	"booking-service/app/utils"
	"context"
	"fmt"
	"strconv"

	"github.com/guregu/null/v5"
	"go.uber.org/zap"

	"booking-service/app/api/dto"
	"booking-service/app/messaging"
)

type Repository interface {
	CreateWithLog(ctx context.Context, booking *models.Booking, log *models.EventLog) (int64, error)
	GetByID(ctx context.Context, id int64) (*models.Booking, error)
	UpdateWithLog(ctx context.Context, booking *models.Booking, log *models.EventLog) error
}

type Publisher interface {
	PublishCreateBookingJob(ctx context.Context, cmd messaging.CreateBookingJobCommand) error
	PublishCancelBookingJob(ctx context.Context, cmd messaging.CancelBookingJobCommand) error
}

// BookingsService обрабатывает команды (изменение состояния) для бронирований.
//
// Этот сервис -- оркестратор: он координирует домен и репозиторий,
// но НЕ содержит бизнес-правила (они в models.Booking).
type BookingsService struct {
	repo      Repository
	publisher Publisher
	logger    *zap.Logger
	clock     utils.Clock
}

// NewBookingsService создаёт новый BookingsService.
func NewBookingsService(repo Repository, publisher Publisher, logger *zap.Logger, clock utils.Clock) *BookingsService {
	return &BookingsService{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
		clock:     clock,
	}
}

// Create создаёт новое бронирование.
//
// Шаги:
//  1. Создание доменного объекта (валидация в конструкторе)
//  2. Сохранение в БД
//  3. Публикация команды в Catalog
//  4. Возврат ID
func (s *BookingsService) Create(ctx context.Context, req dto.CreateBookingRequest) (int64, error) {
	now := s.clock.Now()

	b, err := models.NewBooking(req.UserID, req.ResourceID, req.StartDate.Time, req.EndDate.Time, now)
	if err != nil {
		return 0, err
	}
	// BookingID проставит репозиторий сгенерированным id после вставки брони.
	log := &models.EventLog{
		NewStatus:      b.Status(),
		PreviousStatus: null.Value[models.BookingStatus]{},
		EventTimestamp: now,
		Cause:          null.StringFrom(models.CauseCreated),
		InitiatedBy:    strconv.FormatInt(req.UserID, 10),
	}

	id, err := s.repo.CreateWithLog(ctx, b, log)
	if err != nil {
		return 0, fmt.Errorf("сохранение бронирования: %w", err)
	}

	s.logger.Info("бронирование создано",
		zap.Int64("id", id),
		zap.Int64("userId", req.UserID),
		zap.Int64("resourceId", req.ResourceID),
	)

	if err := s.publisher.PublishCreateBookingJob(ctx, messaging.CreateBookingJobCommand{
		EventId:    messaging.NewMessageID(),
		RequestId:  messaging.BookingIDToRequestID(id),
		ResourceId: req.ResourceID,
		StartDate:  req.StartDate.Format(dto.DateFormat),
		EndDate:    req.EndDate.Format(dto.DateFormat),
	}); err != nil {
		s.logger.Error("ошибка публикации CreateBookingJob", zap.Error(err), zap.Int64("bookingId", id))
		// Не возвращаем ошибку -- бронирование уже создано, команда может быть обработана позже
	}

	return id, nil
}

// Cancel отменяет бронирование по ID.
//
// initiatedBy -- инициатор изменения для журнала. Пустая строка означает
// пользовательскую отмену через HTTP (инициатором становится владелец брони);
// автоматические вызовы (воркер, брокер) передают models.InitiatorSystem.
//
// Шаги:
//  1. Загрузка бронирования из БД
//  2. Вызов доменного метода StartCancel() (валидация перехода статуса)
//  3. Сохранение обновлённого состояния и записи журнала в одной транзакции
//  4. Публикация команды в Catalog
func (s *BookingsService) Cancel(ctx context.Context, id int64, initiatedBy string) error {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	prev := b.Status()
	if err := b.StartCancel(s.clock.Now()); err != nil {
		return err
	}

	// Причина зависит от инициатора: System => отказ Catalog, иначе пользовательский запрос.
	cause := models.CauseUserRequest
	if initiatedBy == models.InitiatorSystem {
		cause = models.CauseCatalogDenied
	}

	logEntry := &models.EventLog{
		BookingID:      b.ID(),
		NewStatus:      b.Status(),
		PreviousStatus: null.ValueFrom(prev),
		EventTimestamp: s.clock.Now(),
		Cause:          null.StringFrom(cause),
		InitiatedBy:    initiatedBy,
	}

	if err := s.repo.UpdateWithLog(ctx, b, logEntry); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}

	s.logger.Info("процесс отмены бронирования начат", zap.Int64("id", id))

	if err := s.publisher.PublishCancelBookingJob(ctx, messaging.CancelBookingJobCommand{
		EventId:   messaging.NewMessageID(),
		RequestId: messaging.BookingIDToRequestID(id),
	}); err != nil {
		s.logger.Error("ошибка публикации CancelBookingJob", zap.Error(err), zap.Int64("bookingId", id))
	}

	return nil
}

func (s *BookingsService) HandleCancelError(ctx context.Context, id int64) error {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	prev := b.Status()
	if err := b.RollbackCancel(); err != nil {
		return err
	}

	cause := models.CauseCancelFailed
	logEntry := &models.EventLog{
		BookingID:      b.ID(),
		NewStatus:      b.Status(),
		PreviousStatus: null.ValueFrom(prev),
		EventTimestamp: s.clock.Now(),
		Cause:          null.StringFrom(cause),
		InitiatedBy:    models.InitiatorSystem,
	}

	if err := s.repo.UpdateWithLog(ctx, b, logEntry); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}

	s.logger.Info(fmt.Sprintf("отмена бронирования отклонена, возвращён предыдущий статус: %s", b.Status()), zap.Int64("id", id))

	return nil
}

func (s *BookingsService) HandleConfirmCancel(ctx context.Context, id int64) error {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	prev := b.Status()
	if err := b.FinishCancel(); err != nil {
		return err
	}

	cause := models.CauseCancelConfirmed
	logEntry := &models.EventLog{
		BookingID:      b.ID(),
		NewStatus:      b.Status(),
		PreviousStatus: null.ValueFrom(prev),
		EventTimestamp: s.clock.Now(),
		Cause:          null.StringFrom(cause),
		InitiatedBy:    models.InitiatorSystem,
	}

	if err := s.repo.UpdateWithLog(ctx, b, logEntry); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}

	s.logger.Info("бронирование отменено", zap.Int64("id", id))

	return nil
}

// Confirm подтверждает бронирование по ID.
// Используется обработчиком событий RabbitMQ.
func (s *BookingsService) Confirm(ctx context.Context, id int64) error {
	b, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if b.Status() == models.BookingStatusCancellationPending {
		s.logger.Warn("race condition: подтверждение брони в процессе отмены",
			zap.Int64("id", id),
		)
	}

	prev := b.Status()
	if err := b.Confirm(); err != nil {
		return err
	}

	cause := models.CauseCatalogConfirmed
	logEntry := &models.EventLog{
		BookingID:      b.ID(),
		NewStatus:      b.Status(),
		PreviousStatus: null.ValueFrom(prev),
		EventTimestamp: s.clock.Now(),
		Cause:          null.StringFrom(cause),
		InitiatedBy:    models.InitiatorSystem,
	}

	if err := s.repo.UpdateWithLog(ctx, b, logEntry); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}

	s.logger.Info("бронирование подтверждено", zap.Int64("id", id))

	return nil
}
