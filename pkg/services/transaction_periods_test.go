package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// transactionTimeOf builds a transaction time value (unix time in milliseconds precision as
// stored in the transaction_time column) from a UTC date and time
func transactionTimeOf(t *testing.T, value string) int64 {
	t.Helper()
	parsed, err := time.Parse("2006-01-02 15:04:05", value)
	assert.Nil(t, err)
	return parsed.Unix() * 1000
}

func TestTransactionPeriod_CalendarMonths(t *testing.T) {
	utc := time.FixedZone("Test Timezone", 0)

	for _, periodStartDay := range []int32{0, 1} {
		yearMonth, daysInto := transactionPeriod(transactionTimeOf(t, "2026-09-01 00:00:00"), utc, periodStartDay)
		assert.Equal(t, int32(202609), yearMonth)
		assert.Equal(t, int32(1), daysInto)

		yearMonth, daysInto = transactionPeriod(transactionTimeOf(t, "2026-09-30 23:59:59"), utc, periodStartDay)
		assert.Equal(t, int32(202609), yearMonth)
		assert.Equal(t, int32(30), daysInto)
	}
}

func TestTransactionPeriod_SalaryMonthStartsItsOwnPeriod(t *testing.T) {
	utc := time.FixedZone("Test Timezone", 0)

	// the 25th is day one of the period named after that month
	yearMonth, daysInto := transactionPeriod(transactionTimeOf(t, "2026-08-25 00:00:00"), utc, 25)
	assert.Equal(t, int32(202608), yearMonth)
	assert.Equal(t, int32(1), daysInto)

	yearMonth, daysInto = transactionPeriod(transactionTimeOf(t, "2026-08-31 23:59:59"), utc, 25)
	assert.Equal(t, int32(202608), yearMonth)
	assert.Equal(t, int32(7), daysInto)
}

func TestTransactionPeriod_DaysBeforeStartBelongToThePreviousPeriod(t *testing.T) {
	utc := time.FixedZone("Test Timezone", 0)

	// 8 Sep is 25-31 Aug (7 days) plus 1-8 Sep (8 days) into the period that began 25 Aug
	yearMonth, daysInto := transactionPeriod(transactionTimeOf(t, "2026-09-08 12:00:00"), utc, 25)
	assert.Equal(t, int32(202608), yearMonth)
	assert.Equal(t, int32(15), daysInto)

	// the last day of the period
	yearMonth, daysInto = transactionPeriod(transactionTimeOf(t, "2026-09-24 23:59:59"), utc, 25)
	assert.Equal(t, int32(202608), yearMonth)
	assert.Equal(t, int32(31), daysInto)

	// and the next day starts the next one
	yearMonth, daysInto = transactionPeriod(transactionTimeOf(t, "2026-09-25 00:00:00"), utc, 25)
	assert.Equal(t, int32(202609), yearMonth)
	assert.Equal(t, int32(1), daysInto)
}

func TestTransactionPeriod_ShortMonthsStayCorrect(t *testing.T) {
	utc := time.FixedZone("Test Timezone", 0)

	// 25 Feb - 24 Mar is only 28 days in a non leap year
	yearMonth, daysInto := transactionPeriod(transactionTimeOf(t, "2026-03-24 12:00:00"), utc, 25)
	assert.Equal(t, int32(202602), yearMonth)
	assert.Equal(t, int32(28), daysInto)

	// 25 Feb 2024 - 24 Mar 2024 covers a leap day, so it is one longer
	yearMonth, daysInto = transactionPeriod(transactionTimeOf(t, "2024-03-24 12:00:00"), utc, 25)
	assert.Equal(t, int32(202402), yearMonth)
	assert.Equal(t, int32(29), daysInto)
}

func TestTransactionPeriod_CrossesTheYearBoundary(t *testing.T) {
	utc := time.FixedZone("Test Timezone", 0)

	yearMonth, daysInto := transactionPeriod(transactionTimeOf(t, "2026-01-03 12:00:00"), utc, 25)
	assert.Equal(t, int32(202512), yearMonth)
	assert.Equal(t, int32(10), daysInto) // 25-31 Dec is 7 days, plus 3 in January
}

func TestTransactionPeriod_RespectsTimezone(t *testing.T) {
	utc8 := time.FixedZone("Test Timezone", 8*60*60)

	// 24 Sep 20:00 UTC is 25 Sep 04:00 in UTC+8, which starts the next period there
	transactionTime := transactionTimeOf(t, "2026-09-24 20:00:00")

	yearMonth, _ := transactionPeriod(transactionTime, time.UTC, 25)
	assert.Equal(t, int32(202608), yearMonth)

	yearMonth, daysInto := transactionPeriod(transactionTime, utc8, 25)
	assert.Equal(t, int32(202609), yearMonth)
	assert.Equal(t, int32(1), daysInto)
}

func TestIsTransactionAfterMaxDaysIntoPeriod(t *testing.T) {
	utc := time.FixedZone("Test Timezone", 0)

	// no limit
	assert.False(t, isTransactionAfterMaxDaysIntoPeriod(transactionTimeOf(t, "2026-09-30 12:00:00"), utc, 0, 0))

	// calendar months behave as before: a cutoff of 7 keeps days 1-7
	assert.False(t, isTransactionAfterMaxDaysIntoPeriod(transactionTimeOf(t, "2026-09-07 23:59:59"), utc, 0, 7))
	assert.True(t, isTransactionAfterMaxDaysIntoPeriod(transactionTimeOf(t, "2026-09-08 00:00:00"), utc, 0, 7))

	// on a 25th-to-24th cycle, 15 days in reaches 8 September
	assert.False(t, isTransactionAfterMaxDaysIntoPeriod(transactionTimeOf(t, "2026-09-08 23:59:59"), utc, 25, 15))
	assert.True(t, isTransactionAfterMaxDaysIntoPeriod(transactionTimeOf(t, "2026-09-09 00:00:00"), utc, 25, 15))

	// and the 25th itself is day one, so it is always included
	assert.False(t, isTransactionAfterMaxDaysIntoPeriod(transactionTimeOf(t, "2026-08-25 00:00:00"), utc, 25, 1))
}
