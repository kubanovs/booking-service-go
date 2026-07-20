package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"booking-service/app/models"
)

func TestNewBooking_Success(t *testing.T) {
	// Arrange
	userID := int64(1)
	resourceID := int64(10)
	startDate := time.Now().AddDate(0, 0, 7)
	endDate := time.Now().AddDate(0, 0, 14)

	// Act
	booking, err := models.NewBooking(userID, resourceID, startDate, endDate)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusAwaitsConfirmation, booking.Status())
	assert.Equal(t, userID, booking.UserID())
	assert.Equal(t, resourceID, booking.ResourceID())
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
	booking := createTestBooking(t)

	err := booking.Confirm()

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusConfirmed, booking.Status())
}

func TestConfirm_FromConfirmed_Error(t *testing.T) {
	booking := createTestBooking(t)
	_ = booking.Confirm()

	err := booking.Confirm()

	assert.ErrorIs(t, err, models.ErrInvalidStatusTransition)
}

func TestCancel_FromAwaitsConfirmation(t *testing.T) {
	booking := createTestBooking(t)

	err := booking.StartCancel(time.Now())

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusCancellationPending, booking.Status())
}

func TestCancel_FromConfirmed_FutureStartDate(t *testing.T) {
	booking := createTestBooking(t)
	_ = booking.Confirm()
	today := time.Now()

	err := booking.StartCancel(today)

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusCancellationPending, booking.Status())
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
	booking := createTestBooking(t)
	_ = booking.StartCancel(time.Now())

	err := booking.StartCancel(time.Now())

	assert.ErrorIs(t, err, models.ErrInvalidStatusTransition)
}

func TestFinishCancel_FromCancelPendingStatus(t *testing.T) {
	booking := createTestBookingCancelStarted()

	err := booking.FinishCancel()

	require.NoError(t, err)
	assert.Equal(t, models.BookingStatusCancelled, booking.Status())
}

func TestFinishCancel_FromOtherStatus(t *testing.T) {
	booking := createTestBooking(t)
	err := booking.FinishCancel()

	assert.ErrorIs(t, err, models.ErrInvalidStatusTransition)
}

func TestRollbackCancel_FromCancelPendingStatus(t *testing.T) {
	booking := createTestBookingCancelStarted()

	err := booking.RollbackCancel()

	require.NoError(t, err, models.ErrInvalidStatusTransition)
	assert.Equal(t, booking.Status(), models.BookingStatusAwaitsConfirmation)
	assert.Empty(t, booking.StatusBeforeCancellation())
	assert.Empty(t, booking.RequestCancellationTimestamp())
}

func TestRollbackCancel_FromOtherStatus(t *testing.T) {
	booking := createTestBooking(t)

	err := booking.RollbackCancel()

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
