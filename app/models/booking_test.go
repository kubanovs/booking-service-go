package models_test

import (
	"booking-service/app/models"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBooking_Success(t *testing.T) {
	// Arrange
	userID := int64(1)
	resourceID := int64(10)
	startDate := time.Now().AddDate(0, 0, 7)
	endDate := time.Now().AddDate(0, 0, 14)

	// Act
	b, err := models.NewBooking(userID, resourceID, startDate, endDate)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusAwaitsConfirmation, b.Status())
	assert.Equal(t, userID, b.UserID())
	assert.Equal(t, resourceID, b.ResourceID())
}

func TestNewBooking_InvalidUserID(t *testing.T) {
	_, err := models.NewBooking(0, 10, time.Now(), time.Now().AddDate(0, 0, 1))
	assert.ErrorIs(t, err, models.ErrInvalidUserID)
}

func TestNewBooking_EndDateBeforeStartDate(t *testing.T) {
	start := time.Now().AddDate(0, 0, 7)
	end := time.Now().AddDate(0, 0, 1)
	_, err := models.NewBooking(1, 10, start, end)
	assert.ErrorIs(t, err, models.ErrEndDateBeforeStartDate)
}

func TestConfirm_FromAwaitsConfirmation(t *testing.T) {
	b := createTestBooking(t)

	err := b.Confirm()

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusConfirmed, b.Status())
}

func TestConfirm_FromConfirmed_Error(t *testing.T) {
	b := createTestBooking(t)
	_ = b.Confirm()

	err := b.Confirm()

	assert.ErrorIs(t, err, models.ErrInvalidStatusTransition)
}

func TestCancel_FromAwaitsConfirmation(t *testing.T) {
	b := createTestBooking(t)

	err := b.StartCancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusCancellationPending, b.Status())
}

func TestCancel_FromConfirmed_FutureStartDate(t *testing.T) {
	b := createTestBooking(t)
	_ = b.Confirm()
	today := time.Now()

	err := b.StartCancel(today)

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusCancellationPending, b.Status())
}

func TestCancel_FromConfirmed_PastStartDate_Error(t *testing.T) {
	b := models.RestoreBooking(
		1,
		models.BookingStatusConfirmed,
		1, 10,
		time.Now().AddDate(0, 0, -3),
		time.Now().AddDate(0, 0, -1),
		time.Now().AddDate(0, 0, -5),
		nil,
		nil,
	)

	err := b.StartCancel(time.Now())

	assert.ErrorIs(t, err, models.ErrCannotCancelPastBooking)
}

func TestCancel_FromCancelled_Error(t *testing.T) {
	b := createTestBooking(t)
	_ = b.StartCancel(time.Now())

	err := b.StartCancel(time.Now())

	assert.ErrorIs(t, err, models.ErrInvalidStatusTransition)
}

func TestFinishCancel_FromCancelPendingStatus(t *testing.T) {
	b := createTestBookingCancelStarted()

	err := b.FinishCancel()

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusCancelled, b.Status())
}

func TestFinishCancel_FromOtherStatus(t *testing.T) {
	b := createTestBooking(t)
	err := b.FinishCancel()

	assert.ErrorIs(t, err, models.ErrInvalidStatusTransition)
}

func TestRollbackCancel_FromCancelPendingStatus(t *testing.T) {
	b := createTestBookingCancelStarted()

	err := b.RollbackCancel()

	require.NoError(t, err, models.ErrInvalidStatusTransition)
	assert.Equal(t, b.Status(), models.BookingStatusAwaitsConfirmation)
	assert.Empty(t, b.StatusBeforeCancellation())
	assert.Empty(t, b.RequestCancellationTimestamp())
}

func TestRollbackCancel_FromOtherStatus(t *testing.T) {
	b := createTestBooking(t)

	err := b.RollbackCancel()

	assert.Equal(t, err, models.ErrInvalidStatusTransition)
}

func createTestBooking(t *testing.T) *models.Booking {
	t.Helper()
	b, err := models.NewBooking(1, 10, time.Now().AddDate(0, 0, 7), time.Now().AddDate(0, 0, 14))
	require.NoError(t, err)
	return b
}

func createTestBookingCancelStarted() *models.Booking {
	statusBeforeCancellation := models.BookingStatusAwaitsConfirmation
	cancellationReqTime := time.Now().Add(1 * time.Hour)

	return models.RestoreBooking(
		1,
		models.BookingStatusCancellationPending,
		0,
		0,
		time.Now().AddDate(0, 0, 7),
		time.Now().AddDate(0, 0, 14),
		time.Now(),
		&statusBeforeCancellation,
		&cancellationReqTime,
	)
}
