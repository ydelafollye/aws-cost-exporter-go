package timeutil

import "time"

type Period struct {
	Start time.Time
	End   time.Time
}

func DailyPeriod(delayDays int) Period {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end = end.AddDate(0, 0, -delayDays)
	start := end.AddDate(0, 0, -1)

	return Period{Start: start, End: end}
}

func MonthlyPeriod(delayDays int) Period {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end = end.AddDate(0, 0, -delayDays)
	start := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)

	return Period{Start: start, End: end}
}
