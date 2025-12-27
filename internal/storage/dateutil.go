package storage

import "time"

// GetBillingCycleDates returns start and end dates for a billing cycle.
// cycleDay: the day of month when billing cycle starts (1-28)
// Example: cycleDay=15 on Jan 20 returns (Dec 15, Jan 14)
func GetBillingCycleDates(cycleDay int, reference time.Time) (start, end time.Time) {
	if cycleDay < 1 || cycleDay > 28 {
		cycleDay = 1 // Default to calendar month
	}

	year, month, day := reference.Date()
	loc := reference.Location()

	if day >= cycleDay {
		// Current cycle started this month
		start = time.Date(year, month, cycleDay, 0, 0, 0, 0, loc)
		end = time.Date(year, month+1, cycleDay-1, 23, 59, 59, 0, loc)
	} else {
		// Current cycle started last month
		start = time.Date(year, month-1, cycleDay, 0, 0, 0, 0, loc)
		end = time.Date(year, month, cycleDay-1, 23, 59, 59, 0, loc)
	}

	return start, end
}

// GetCurrentMonthDates returns 1st of current month to now.
func GetCurrentMonthDates(reference time.Time) (start, end time.Time) {
	year, month, _ := reference.Date()
	loc := reference.Location()

	start = time.Date(year, month, 1, 0, 0, 0, 0, loc)
	end = reference

	return start, end
}

// GetLastNDays returns date range for last N days.
func GetLastNDays(n int, reference time.Time) (start, end time.Time) {
	end = reference
	start = reference.AddDate(0, 0, -n+1) // -n+1 to include today
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, reference.Location())
	return start, end
}

// GetTodayDates returns start and end of today.
func GetTodayDates(reference time.Time) (start, end time.Time) {
	year, month, day := reference.Date()
	loc := reference.Location()

	start = time.Date(year, month, day, 0, 0, 0, 0, loc)
	end = reference

	return start, end
}

// GetYesterdayDates returns start and end of yesterday.
func GetYesterdayDates(reference time.Time) (start, end time.Time) {
	yesterday := reference.AddDate(0, 0, -1)
	year, month, day := yesterday.Date()
	loc := reference.Location()

	start = time.Date(year, month, day, 0, 0, 0, 0, loc)
	end = time.Date(year, month, day, 23, 59, 59, 0, loc)

	return start, end
}

// GetLastMonthDates returns the full previous month range.
func GetLastMonthDates(reference time.Time) (start, end time.Time) {
	year, month, _ := reference.Date()
	loc := reference.Location()

	// First day of last month
	start = time.Date(year, month-1, 1, 0, 0, 0, 0, loc)
	// Last day of last month (day 0 of current month)
	end = time.Date(year, month, 0, 23, 59, 59, 0, loc)

	return start, end
}

// FormatDateRange returns date strings for database queries.
func FormatDateRange(start, end time.Time) (startDate, endDate string) {
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}
