package service

import (
	"booking-service/app/models"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"booking-service/app/api/dto"
	"booking-service/app/messaging"
)

// BookingsService обрабатывает команды (изменение состояния) для бронирований.
//
// Этот сервис -- оркестратор: он координирует домен и репозиторий,
// но НЕ содержит бизнес-правила (они в models.Booking).
type BookingsService struct {
	repo      models.BookingRepository
	publisher *messaging.Publisher
	logger    *zap.Logger
}

// NewBookingsService создаёт новый BookingsService.
func NewBookingsService(repo models.BookingRepository, publisher *messaging.Publisher, logger *zap.Logger) *BookingsService {
	return &BookingsService{
		repo:      repo,
		publisher: publisher,
		logger:    logger,
	}
}

// Create создаёт новое бронирование.
//
// Шаги:
//  1. Парсинг дат из строкового формата
//  2. Создание доменного объекта (валидация в конструкторе)
//  3. Сохранение в БД
//  4. Публикация команды в Catalog
//  5. Возврат ID
func (s *BookingsService) Create(ctx context.Context, req dto.CreateBookingRequest) (int64, error) {
	startDate, err := time.Parse(dto.DateFormat, req.StartDate)
	if err != nil {
		return 0, fmt.Errorf("некорректный формат startDate: %w", err)
	}

	endDate, err := time.Parse(dto.DateFormat, req.EndDate)
	if err != nil {
		return 0, fmt.Errorf("некорректный формат endDate: %w", err)
	}

	b, err := models.NewBooking(req.UserID, req.ResourceID, startDate, endDate)
	if err != nil {
		return 0, err
	}

	initiatedBy := strconv.FormatInt(req.UserID, 10)

	id, err := s.repo.CreateWithLog(ctx, b, initiatedBy)
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
		StartDate:  req.StartDate,
		EndDate:    req.EndDate,
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
	switch {
	case errors.Is(err, models.ErrBookingNotFound):
		s.logger.Warn("не найдено бронирование", zap.Int64("id", id), zap.Error(err))
		return nil
	case err != nil:
		return fmt.Errorf("ошибка получения бронирования с id=%d: %w", id, err)
	}

	prev := b.Status()
	if err := b.StartCancel(time.Now()); err != nil {
		return err
	}

	// Пустой initiatedBy => пользовательская отмена: инициатор -- владелец брони.
	if initiatedBy == "" {
		initiatedBy = strconv.FormatInt(b.UserID(), 10)
	}

	logEntry := models.RecordEventLog(b.ID(), prev, b.Status(), initiatedBy, nil)

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

	switch {
	case errors.Is(err, models.ErrBookingNotFound):
		s.logger.Warn("не найдено бронирование", zap.Int64("id", id), zap.Error(err))
		return nil
	case err != nil:
		return fmt.Errorf("ошибка получения бронирования с id=%d: %w", id, err)
	}

	prev := b.Status()
	if err := b.RollbackCancel(); err != nil {
		return err
	}

	logEntry := models.RecordEventLog(b.ID(), prev, b.Status(), models.InitiatorSystem, nil)

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

	logEntry := models.RecordEventLog(b.ID(), prev, b.Status(), models.InitiatorSystem, nil)

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

	logEntry := models.RecordEventLog(b.ID(), prev, b.Status(), models.InitiatorSystem, nil)

	if err := s.repo.UpdateWithLog(ctx, b, logEntry); err != nil {
		return fmt.Errorf("обновление бронирования: %w", err)
	}

	s.logger.Info("бронирование подтверждено", zap.Int64("id", id))

	return nil
}
