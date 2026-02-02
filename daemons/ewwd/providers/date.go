package providers

import (
	"context"
	"time"

	"dotfiles/daemons/ewwd/config"
)

// DateState provides formatted date/time fields and personal life counter for statusbar display.
type DateState struct {
	Weekday      string `json:"weekday"`       // Full weekday name
	WeekdayShort string `json:"weekday_short"` // 3-letter weekday abbreviation
	Month        string `json:"month"`         // Full month name
	MonthShort   string `json:"month_short"`   // 3-letter month abbreviation
	Day          string `json:"day"`           // Day of month (zero-padded)
	ClockHour    string `json:"clock_hour"`    // Clock icon for current hour
	ClockMinute  string `json:"clock_minute"`  // Clock icon for 5-minute interval
	WeeksAlive   int    `json:"weeks_alive"`   // Weeks since configured birth date
}

// Date updates date/time state every minute, aligned to minute boundaries for efficiency.
type Date struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	birthDate time.Time // For weeks_alive calculation
}

// NewDate creates a Date provider with birth date from config (fallback: 1996-02-26).
func NewDate(state StateSetter, cfg config.DateConfig) Provider {
	birthDate, err := time.Parse("2006-01-02", cfg.BirthDate)
	if err != nil {
		birthDate, _ = time.Parse("2006-01-02", "1996-02-26") // Fallback
	}

	return &Date{
		state:     state,
		done:      make(chan struct{}),
		birthDate: birthDate,
	}
}

func (d *Date) Name() string {
	return "date"
}

// Start sends initial state then updates every minute, aligned to minute boundaries for efficiency.
func (d *Date) Start(ctx context.Context, notify func(data any)) error {
	d.active = true

	// Initial update
	state := d.read()
	d.state.Set("date", state)
	notify(state)

	// Update every minute (aligned to minute boundary)
	for {
		// Sleep until next minute
		now := time.Now()
		sleepDuration := time.Duration(60-now.Second())*time.Second - time.Duration(now.Nanosecond())
		if sleepDuration <= 0 {
			sleepDuration = time.Minute
		}

		select {
		case <-ctx.Done():
			return nil
		case <-d.done:
			return nil
		case <-time.After(sleepDuration):
			state := d.read()
			d.state.Set("date", state)
			notify(state)
		}
	}
}

func (d *Date) Stop() error {
	if d.active {
		close(d.done)
		d.active = false
	}
	return nil
}

func (d *Date) read() *DateState {
	now := time.Now()
	weeksAlive := int(now.Sub(d.birthDate).Hours() / 24 / 7)

	return &DateState{
		Weekday:      now.Format("Monday"),
		WeekdayShort: now.Format("Mon"),
		Month:        now.Format("January"),
		MonthShort:   now.Format("Jan"),
		Day:          now.Format("02"),
		ClockHour:    clockHourIcon(now.Hour()),
		ClockMinute:  clockMinuteIcon(now.Minute()),
		WeeksAlive:   weeksAlive,
	}
}

// clockHourIcon maps 24-hour time to 12-hour clockface icon.
func clockHourIcon(hour int) string {
	h := hour % 12
	if h == 0 {
		h = 12
	}

	icons := map[int]string{
		1: "", 2: "", 3: "", 4: "", 5: "", 6: "",
		7: "", 8: "", 9: "", 10: "", 11: "", 12: "",
	}
	if icon, ok := icons[h]; ok {
		return icon
	}
	return ""
}

// clockMinuteIcon maps minutes to 5-minute interval clockface icon (0-55 minutes).
func clockMinuteIcon(minute int) string {
	interval := minute / 5

	icons := map[int]string{
		0: "", 1: "", 2: "", 3: "", 4: "", 5: "", 6: "",
		7: "", 8: "", 9: "", 10: "", 11: "", 12: "",
	}
	if icon, ok := icons[interval]; ok {
		return icon
	}
	return ""
}
