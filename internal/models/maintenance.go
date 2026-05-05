package models

import "encoding/json"

// Maintenance represents an Uptime Kuma maintenance window.
type Maintenance struct {
	ID             int     `json:"id"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Strategy       string  `json:"strategy"`
	Active         FlexInt `json:"active"`
	Cron           string  `json:"cron,omitempty"`
	StartDate      string  `json:"startDate,omitempty"`
	EndDate        string  `json:"endDate,omitempty"`
	StartTime      string  `json:"startTime,omitempty"`
	EndTime        string  `json:"endTime,omitempty"`
	Weekdays       []int   `json:"weekdays,omitempty"`
	DaysOfMonth    []int   `json:"daysOfMonth,omitempty"`
	IntervalDay    int     `json:"intervalDay,omitempty"`
	TimezoneOption string  `json:"timezoneOption,omitempty"`
	TimezoneOffset string  `json:"timezoneOffset,omitempty"`
}

func (m *Maintenance) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID             int      `json:"id"`
		Title          string   `json:"title"`
		Description    string   `json:"description"`
		Strategy       string   `json:"strategy"`
		Active         FlexInt  `json:"active"`
		Cron           string   `json:"cron"`
		StartDate      string   `json:"startDate"`
		EndDate        string   `json:"endDate"`
		StartTime      string   `json:"startTime"`
		EndTime        string   `json:"endTime"`
		Weekdays       []int    `json:"weekdays"`
		DaysOfMonth    []int    `json:"daysOfMonth"`
		IntervalDay    int      `json:"intervalDay"`
		TimezoneOption string   `json:"timezoneOption"`
		TimezoneOffset string   `json:"timezoneOffset"`
		DateRange      []string `json:"dateRange"`
		TimeRange      []struct {
			Hours   int `json:"hours"`
			Minutes int `json:"minutes"`
			Seconds int `json:"seconds"`
		} `json:"timeRange"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	m.ID = raw.ID
	m.Title = raw.Title
	m.Description = raw.Description
	m.Strategy = raw.Strategy
	m.Active = raw.Active
	m.Cron = raw.Cron
	m.StartDate = raw.StartDate
	m.EndDate = raw.EndDate
	m.StartTime = raw.StartTime
	m.EndTime = raw.EndTime
	m.Weekdays = raw.Weekdays
	m.DaysOfMonth = raw.DaysOfMonth
	m.IntervalDay = raw.IntervalDay
	m.TimezoneOption = raw.TimezoneOption
	m.TimezoneOffset = raw.TimezoneOffset

	if m.StartDate == "" && len(raw.DateRange) > 0 {
		m.StartDate = raw.DateRange[0]
	}
	if m.EndDate == "" && len(raw.DateRange) > 1 {
		m.EndDate = raw.DateRange[1]
	}
	if m.StartTime == "" && len(raw.TimeRange) > 0 {
		m.StartTime = formatMaintenanceTime(raw.TimeRange[0].Hours, raw.TimeRange[0].Minutes, raw.TimeRange[0].Seconds)
	}
	if m.EndTime == "" && len(raw.TimeRange) > 1 {
		m.EndTime = formatMaintenanceTime(raw.TimeRange[1].Hours, raw.TimeRange[1].Minutes, raw.TimeRange[1].Seconds)
	}

	return nil
}

func formatMaintenanceTime(hours, minutes, seconds int) string {
	if seconds != 0 {
		return twoDigits(hours) + ":" + twoDigits(minutes) + ":" + twoDigits(seconds)
	}
	return twoDigits(hours) + ":" + twoDigits(minutes)
}

func twoDigits(v int) string {
	if v < 10 {
		return "0" + string(rune('0'+v))
	}
	return string([]byte{'0' + byte(v/10), '0' + byte(v%10)})
}
