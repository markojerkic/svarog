package db_test

import (
	"testing"
	"time"

	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/stretchr/testify/require"
)

func TestParseLogRange(t *testing.T) {
	got, err := db.ParseLogRange("2026-11-02", "2026-11-02", -60)
	require.NoError(t, err, "a local calendar day must parse")
	require.True(t, got.From.Equal(time.Date(2026, 11, 1, 23, 0, 0, 0, time.UTC)), "local midnight is the inclusive start")
	require.True(t, got.To.Equal(time.Date(2026, 11, 2, 23, 0, 0, 0, time.UTC)), "the next local midnight is the exclusive end")

	swapped, err := db.ParseLogRange("2026-11-05", "2026-11-02", 0)
	require.NoError(t, err, "a reversed range is still a valid selection")
	require.Equal(t, "2026-11-02", swapped.FromDate, "reversed dates are stored in calendar order")
	require.Equal(t, "2026-11-05", swapped.ToDate, "the later date remains the inclusive end day")

	_, err = db.ParseLogRange("yesterday", "2026-11-02", 0)
	require.Error(t, err, "a non-ISO date is rejected")
	_, err = db.ParseLogRange("2026-11-02", "2026-11-02", 24*60)
	require.Error(t, err, "an impossible timezone offset is rejected")
}
