package providers

// date.go publishes wall-clock date/time fields and age-derived display metrics.

import (
	"context"
	"dotfiles/daemons/config"
	"time"
)

type DateState struct {
	Weekday      string `json:"weekday"`
	WeekdayShort string `json:"weekday_short"`
	Month        string `json:"month"`
	MonthShort   string `json:"month_short"`
	Day          string `json:"day"`
	ClockHour    string `json:"clock_hour"`
	ClockMinute  string `json:"clock_minute"`
	WeeksAlive   int    `json:"weeks_alive"`
}

type Date struct {
	state     StateSetter
	done      chan struct{}
	active    bool
	birthDate time.Time
}

// NewDate constructs a Date provider, falling back to 1996-02-26 if cfg.BirthDate is unparseable.
func NewDate(state StateSetter, cfg config.DateConfig) Provider {
	birthDate, err := time.Parse("2006-01-02", cfg.BirthDate)
	if err != nil {
		birthDate, _ = time.Parse("2006-01-02", "1996-02-26")
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

func (d *Date) Start(ctx context.Context, notify func(data any)) error {
	d.active = true

	state := d.read()
	d.state.Set("date", state)
	notify(state)

	for {
		// Align to the top of each minute.
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

func clockHourIcon(hour int) string {
	h := hour % 12
	if h == 0 {
		h = 12
	}

	icons := map[int]string{
		1: "", 2: "", 3: "", 4: "", 5: "", 6: "",
		7: "", 8: "", 9: "", 10: "", 11: "", 12: "",
	}
	if icon, ok := icons[h]; ok {
		return icon
	}
	return ""
}

func clockMinuteIcon(minute int) string {
	interval := minute / 5

	icons := map[int]string{
		0: "", 1: "", 2: "", 3: "", 4: "", 5: "", 6: "",
		7: "", 8: "", 9: "", 10: "", 11: "", 12: "",
	}
	if icon, ok := icons[interval]; ok {
		return icon
	}
	return ""
}
