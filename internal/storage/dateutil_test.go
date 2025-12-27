package storage

import (
	"testing"
	"time"
)

func TestGetBillingCycleDates(t *testing.T) {
	loc := time.Local

	tests := []struct {
		name      string
		cycleDay  int
		reference time.Time
		wantStart string
		wantEnd   string
	}{
		{
			name:      "mid-month billing, after cycle day",
			cycleDay:  15,
			reference: time.Date(2025, 1, 20, 12, 0, 0, 0, loc),
			wantStart: "2025-01-15",
			wantEnd:   "2025-02-14",
		},
		{
			name:      "mid-month billing, before cycle day",
			cycleDay:  15,
			reference: time.Date(2025, 1, 10, 12, 0, 0, 0, loc),
			wantStart: "2024-12-15",
			wantEnd:   "2025-01-14",
		},
		{
			name:      "calendar month (day 1), mid-month",
			cycleDay:  1,
			reference: time.Date(2025, 1, 15, 12, 0, 0, 0, loc),
			wantStart: "2025-01-01",
			wantEnd:   "2025-01-31",
		},
		{
			name:      "invalid cycle day defaults to 1",
			cycleDay:  30,
			reference: time.Date(2025, 1, 15, 12, 0, 0, 0, loc),
			wantStart: "2025-01-01",
			wantEnd:   "2025-01-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := GetBillingCycleDates(tt.cycleDay, tt.reference)
			gotStart := start.Format("2006-01-02")
			gotEnd := end.Format("2006-01-02")

			if gotStart != tt.wantStart {
				t.Errorf("start = %s, want %s", gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("end = %s, want %s", gotEnd, tt.wantEnd)
			}
		})
	}
}

func TestGetCurrentMonthDates(t *testing.T) {
	ref := time.Date(2025, 1, 15, 14, 30, 0, 0, time.Local)
	start, end := GetCurrentMonthDates(ref)

	if start.Day() != 1 || start.Month() != 1 {
		t.Errorf("start should be Jan 1, got %v", start)
	}
	if !end.Equal(ref) {
		t.Errorf("end should be reference time, got %v", end)
	}
}

func TestGetLastNDays(t *testing.T) {
	ref := time.Date(2025, 1, 15, 14, 30, 0, 0, time.Local)
	start, end := GetLastNDays(7, ref)

	expectedStart := time.Date(2025, 1, 9, 0, 0, 0, 0, time.Local)
	if !start.Equal(expectedStart) {
		t.Errorf("start = %v, want %v", start, expectedStart)
	}
	if !end.Equal(ref) {
		t.Errorf("end should be reference time")
	}
}

func TestGetLastMonthDates(t *testing.T) {
	ref := time.Date(2025, 2, 15, 14, 30, 0, 0, time.Local)
	start, end := GetLastMonthDates(ref)

	if start.Month() != 1 || start.Day() != 1 {
		t.Errorf("start should be Jan 1, got %v", start)
	}
	if end.Month() != 1 || end.Day() != 31 {
		t.Errorf("end should be Jan 31, got %v", end)
	}
}
