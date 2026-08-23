package service

import (
	"booking-service/app/api/dto"
	"booking-service/app/messaging"
	"booking-service/app/models"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/guregu/null/v5"
	"go.uber.org/zap"
)

const successCreationID = 1

// fixedClock -- детерминированные часы для тестов: всегда возвращают заданный момент.
type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

// spyRepository запоминает аргументы, с которыми его вызвали, чтобы тест
// мог сверить их после Act. Сам ничего не проверяет.
type spyRepository struct {
	existingBooking *models.Booking // nil => GetByID вернёт ErrBookingNotFound
	eventProcessed  bool            // что вернёт IsEventProcessed

	gotBooking *models.Booking
	gotLog     *models.EventLog
	gotEventID string
}

func (s *spyRepository) CreateWithLog(ctx context.Context, booking *models.Booking, log *models.EventLog) (int64, error) {
	s.gotBooking = booking
	s.gotLog = log
	return successCreationID, nil
}

func (s *spyRepository) GetByID(ctx context.Context, id int64) (*models.Booking, error) {
	if s.existingBooking == nil || s.existingBooking.ID() != id {
		return nil, models.ErrBookingNotFound
	}
	return s.existingBooking, nil
}

func (s *spyRepository) IsEventProcessed(ctx context.Context, eventID string) (bool, error) {
	return s.eventProcessed, nil
}

func (s *spyRepository) UpdateWithLog(ctx context.Context, booking *models.Booking, log *models.EventLog, eventID string) error {
	s.gotBooking = booking
	s.gotLog = log
	s.gotEventID = eventID
	return nil
}

// spyPublisher запоминает опубликованные команды для проверки в тесте.
type spyPublisher struct {
	createCmd *messaging.CreateBookingJobCommand
	cancelCmd *messaging.CancelBookingJobCommand
}

func (s *spyPublisher) PublishCreateBookingJob(ctx context.Context, cmd messaging.CreateBookingJobCommand) error {
	s.createCmd = &cmd
	return nil
}

func (s *spyPublisher) PublishCancelBookingJob(ctx context.Context, cmd messaging.CancelBookingJobCommand) error {
	s.cancelCmd = &cmd
	return nil
}

func TestCreate_Success(t *testing.T) {
	// Arrange
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	userID, resourceID := int64(1), int64(10)
	startDate := now.AddDate(0, 0, 7)
	endDate := now.AddDate(0, 0, 14)

	repo := &spyRepository{}
	pub := &spyPublisher{}
	svc := NewBookingsService(repo, pub, zap.NewNop(), fixedClock{now: now})

	req := dto.CreateBookingRequest{
		UserID:     userID,
		ResourceID: resourceID,
		StartDate:  dto.Date{Time: startDate},
		EndDate:    dto.Date{Time: endDate},
	}

	// Act
	id, err := svc.Create(context.Background(), req)

	// Assert
	if err != nil {
		t.Fatalf("Create вернул ошибку: %v", err)
	}
	if id != successCreationID {
		t.Errorf("id = %d, ожидали %d", id, successCreationID)
	}

	expectedBooking := models.RestoreBooking(
		0,
		models.BookingStatusAwaitsConfirmation,
		userID, resourceID,
		startDate, endDate, now,
		null.String{},
		null.Time{},
	)
	if repo.gotBooking == nil {
		t.Fatal("CreateWithLog не был вызван")
	}
	if diff := cmp.Diff(*expectedBooking, *repo.gotBooking, cmp.AllowUnexported(models.Booking{})); diff != "" {
		t.Errorf("Booking не совпал (-want +got):\n%s", diff)
	}

	expectedLog := models.EventLog{
		NewStatus:      models.BookingStatusAwaitsConfirmation,
		PreviousStatus: null.Value[models.BookingStatus]{}, // событие создания -> NULL
		EventTimestamp: now,
		Cause:          null.StringFrom(models.CauseCreated),
		InitiatedBy:    strconv.FormatInt(userID, 10),
	}
	if diff := cmp.Diff(expectedLog, *repo.gotLog); diff != "" {
		t.Errorf("EventLog не совпал (-want +got):\n%s", diff)
	}

	if pub.createCmd == nil {
		t.Fatal("PublishCreateBookingJob не был вызван")
	}
	expectedCmd := messaging.CreateBookingJobCommand{
		RequestId:  messaging.BookingIDToRequestID(successCreationID),
		ResourceId: resourceID,
		StartDate:  startDate.Format(dto.DateFormat),
		EndDate:    endDate.Format(dto.DateFormat),
	}
	if diff := cmp.Diff(expectedCmd, *pub.createCmd,
		cmpopts.IgnoreFields(messaging.CreateBookingJobCommand{}, "EventId")); diff != "" {
		t.Errorf("опубликованная команда не совпала (-want +got):\n%s", diff)
	}
}
