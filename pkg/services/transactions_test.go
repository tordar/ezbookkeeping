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

func TestIsTransactionAfterMaxDayOfMonth_NoLimit(t *testing.T) {
	utcTimezone := time.FixedZone("Test Timezone", 0)
	transactionTime := transactionTimeOf(t, "2026-09-28 12:00:00")

	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTime, utcTimezone, 0))
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTime, utcTimezone, -1))
}

func TestIsTransactionAfterMaxDayOfMonth_BeforeAndOnCutoff(t *testing.T) {
	utcTimezone := time.FixedZone("Test Timezone", 0)

	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-09-01 00:00:00"), utcTimezone, 7))
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-09-06 23:59:59"), utcTimezone, 7))
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-09-07 00:00:00"), utcTimezone, 7))
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-09-07 23:59:59"), utcTimezone, 7))
}

func TestIsTransactionAfterMaxDayOfMonth_AfterCutoff(t *testing.T) {
	utcTimezone := time.FixedZone("Test Timezone", 0)

	assert.True(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-09-08 00:00:00"), utcTimezone, 7))
	assert.True(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-09-30 23:59:59"), utcTimezone, 7))
}

func TestIsTransactionAfterMaxDayOfMonth_ShortMonthKeepsEveryDay(t *testing.T) {
	utcTimezone := time.FixedZone("Test Timezone", 0)

	// a cutoff of 31 must never exclude anything, whatever the length of the month
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-02-28 23:59:59"), utcTimezone, 31))
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTimeOf(t, "2026-04-30 23:59:59"), utcTimezone, 31))
}

func TestIsTransactionAfterMaxDayOfMonth_RespectsTimezone(t *testing.T) {
	utc8Timezone := time.FixedZone("Test Timezone", 8*60*60)
	utcMinus5Timezone := time.FixedZone("Test Timezone", -5*60*60)

	// 2026-09-07 20:00 UTC is 2026-09-08 04:00 in UTC+8, so it falls outside a day 7 cutoff there
	transactionTime := transactionTimeOf(t, "2026-09-07 20:00:00")
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTime, time.UTC, 7))
	assert.True(t, isTransactionAfterMaxDayOfMonth(transactionTime, utc8Timezone, 7))

	// 2026-09-08 03:00 UTC is 2026-09-07 22:00 in UTC-5, so it falls inside a day 7 cutoff there
	transactionTime = transactionTimeOf(t, "2026-09-08 03:00:00")
	assert.True(t, isTransactionAfterMaxDayOfMonth(transactionTime, time.UTC, 7))
	assert.False(t, isTransactionAfterMaxDayOfMonth(transactionTime, utcMinus5Timezone, 7))
}
