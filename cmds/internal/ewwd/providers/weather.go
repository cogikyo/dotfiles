package providers

// weather.go fetches weather, forecast, and air-quality data from OpenWeatherMap APIs.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"dotfiles/cmds/internal/config"
)

type WeatherState struct {
	Icon      string `json:"icon"`
	Desc      string `json:"desc"`
	Temp      int    `json:"temp"`
	FeelsLike int    `json:"feels_like"`
	TempMorn  int    `json:"temp_morn"`
	TempDay   int    `json:"temp_day"`
	TempEve   int    `json:"temp_eve"`
	TempNight int    `json:"temp_night"`
	TempMax   int    `json:"temp_max"`
	UVI       int    `json:"uvi"`
	UVIDesc   string `json:"uvi_desc"`
	AQI       int    `json:"aqi"`
	AQIDesc   string `json:"aqi_desc"`
	Rain1h    int    `json:"rain_1h"`
	RainDay   int    `json:"rain_day"`
	Clouds    int    `json:"clouds"`
	WindSpeed int    `json:"wind_speed"`
	BFIcon    string `json:"bf_icon"`
	BFDesc    string `json:"bf_desc"`
	Sunset    string `json:"sunset"`
	Moon      string `json:"moon"`
	MoonDesc  string `json:"moon_desc"`
	Night     bool   `json:"night"`
}

// Weather pulls current conditions, forecasts, and AQI from OpenWeatherMap.
type Weather struct {
	state  StateSetter
	config config.WeatherConfig
	done   chan struct{}
	active bool
	client *http.Client

	lat    string
	lon    string
	apiKey string
}

// NewWeather constructs a Weather provider; credentials and coordinates load lazily in Start.
func NewWeather(state StateSetter, cfg config.WeatherConfig) Provider {
	return &Weather{
		state:  state,
		config: cfg,
		done:   make(chan struct{}),
	}
}

func (w *Weather) Name() string {
	return "weather"
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ lifecycle                                                                    │
// ╰──────────────────────────────────────────────────────────────────────────────╯

const weatherStartupRetryInterval = time.Minute

// Start loads credentials, publishes the first successful fetch, then polls at PollInterval.
func (w *Weather) Start(ctx context.Context, notify func(data any)) error {
	w.active = true
	w.client = &http.Client{Timeout: 10 * time.Second}

	for {
		if w.apiKey == "" || w.lat == "" || w.lon == "" {
			if err := w.loadCredentials(); err != nil {
				fmt.Fprintf(os.Stderr, "ewwd: weather config error: %v\n", err)
				if w.wait(ctx, weatherStartupRetryInterval) {
					return nil
				}
				continue
			}
		}

		state := w.fetch()
		if state != nil {
			w.publish(notify, state)
			break
		}
		if w.wait(ctx, weatherStartupRetryInterval) {
			return nil
		}
	}

	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.done:
			return nil
		case <-ticker.C:
			if state := w.fetch(); state != nil {
				w.publish(notify, state)
			}
		}
	}
}

func (w *Weather) wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return true
	case <-w.done:
		return true
	case <-timer.C:
		return false
	}
}

func (w *Weather) publish(notify func(data any), state *WeatherState) {
	w.state.Set("weather", state)
	notify(state)
}

func (w *Weather) Stop() error {
	if w.active {
		close(w.done)
		w.active = false
	}
	return nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ credentials and geolocation                                                  │
// ╰──────────────────────────────────────────────────────────────────────────────╯

type geoResponse struct {
	Status string  `json:"status"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// loadCredentials reads the API key and resolves location via IP geolocation, falling back to ~/.local/.location.
func (w *Weather) loadCredentials() error {
	keyPath := config.ExpandPath(w.config.APIKeyFile)
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}
	w.apiKey = strings.TrimSpace(string(keyData))

	if err := w.geolocate(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: geolocation failed (%v), trying fallback\n", err)

		locPath := config.ExpandPath("~/.local/.location")
		locData, err := os.ReadFile(locPath)
		if err != nil {
			return fmt.Errorf("geolocation failed and no fallback (~/.local/.location): %w", err)
		}
		parts := strings.Fields(string(locData))
		if len(parts) < 2 {
			return fmt.Errorf("invalid fallback location format: expected 'LAT LON'")
		}
		w.lat = parts[0]
		w.lon = parts[1]
		fmt.Fprintf(os.Stderr, "ewwd: using fallback location: %s, %s\n", w.lat, w.lon)
	}

	return nil
}

// geolocate queries ip-api.com for approximate coordinates.
func (w *Weather) geolocate() error {
	resp, err := w.client.Get("http://ip-api.com/json/")
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var geo geoResponse
	if err := json.Unmarshal(body, &geo); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if geo.Status != "success" {
		return fmt.Errorf("api status: %s", geo.Status)
	}

	w.lat = fmt.Sprintf("%f", geo.Lat)
	w.lon = fmt.Sprintf("%f", geo.Lon)
	return nil
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ api fetch and parse                                                          │
// ╰──────────────────────────────────────────────────────────────────────────────╯

func (w *Weather) fetch() *WeatherState {
	weatherURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/3.0/onecall?lat=%s&lon=%s&units=metric&exclude=minutely&appid=%s",
		w.lat, w.lon, w.apiKey,
	)
	weatherData, err := w.httpGet(weatherURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: weather fetch error: %v\n", err)
		return nil
	}

	airURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/2.5/air_pollution?lat=%s&lon=%s&appid=%s",
		w.lat, w.lon, w.apiKey,
	)
	airData, err := w.httpGet(airURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: air quality fetch error: %v\n", err)
	}

	return w.parse(weatherData, airData)
}

func (w *Weather) httpGet(url string) ([]byte, error) {
	resp, err := w.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

type owmResponse struct {
	Current struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Sunset    int64   `json:"sunset"`
		UVI       float64 `json:"uvi"`
		Clouds    int     `json:"clouds"`
		WindSpeed float64 `json:"wind_speed"`
		Weather   []struct {
			ID   int    `json:"id"`
			Main string `json:"main"`
		} `json:"weather"`
	} `json:"current"`
	Hourly []struct {
		Pop float64 `json:"pop"`
	} `json:"hourly"`
	Daily []struct {
		MoonPhase float64 `json:"moon_phase"`
		Pop       float64 `json:"pop"`
		Temp      struct {
			Morn  float64 `json:"morn"`
			Day   float64 `json:"day"`
			Eve   float64 `json:"eve"`
			Night float64 `json:"night"`
			Max   float64 `json:"max"`
		} `json:"temp"`
	} `json:"daily"`
}

type airResponse struct {
	List []struct {
		Main struct {
			AQI int `json:"aqi"`
		} `json:"main"`
	} `json:"list"`
}

func (w *Weather) parse(weatherData, airData []byte) *WeatherState {
	var weather owmResponse
	if err := json.Unmarshal(weatherData, &weather); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: weather parse error: %v\n", err)
		return nil
	}

	var air airResponse
	if len(airData) > 0 {
		if err := json.Unmarshal(airData, &air); err != nil {
			fmt.Fprintf(os.Stderr, "ewwd: air parse error: %v\n", err)
		}
	}

	if len(weather.Current.Weather) == 0 || len(weather.Daily) == 0 {
		fmt.Fprintf(os.Stderr, "ewwd: weather parse error: missing current weather or daily forecast\n")
		return nil
	}

	now := time.Now().Unix()
	sunset := weather.Current.Sunset
	night := now > sunset

	id := weather.Current.Weather[0].ID
	desc := weather.Current.Weather[0].Main

	temp := roundToInt(weather.Current.Temp)
	feelsLike := roundToInt(weather.Current.FeelsLike)
	tempMorn := roundToInt(weather.Daily[0].Temp.Morn)
	tempDay := roundToInt(weather.Daily[0].Temp.Day)
	tempEve := roundToInt(weather.Daily[0].Temp.Eve)
	tempNight := roundToInt(weather.Daily[0].Temp.Night)
	tempMax := roundToInt(weather.Daily[0].Temp.Max)

	uvi := roundToInt(weather.Current.UVI)
	uviDesc := getUVIDesc(uvi)

	aqi := 0
	if len(air.List) > 0 {
		aqi = air.List[0].Main.AQI
	}
	aqiDesc := getAQIDesc(aqi)

	rain1h := 0
	if len(weather.Hourly) > 0 {
		rain1h = int(weather.Hourly[0].Pop * 100)
	}
	rainDay := int(weather.Daily[0].Pop * 100)

	windSpeed := roundToInt(weather.Current.WindSpeed)
	knots := roundToInt(weather.Current.WindSpeed * 1.943844) // m/s -> kn for Beaufort scale
	bfIcon, bfDesc := getBeaufortScale(knots)

	sunsetTime := time.Unix(sunset, 0).Format("15:04")

	moonPhase := int(weather.Daily[0].MoonPhase * 100)
	moon, moonDesc := getMoonPhase(moonPhase)

	icon := getWeatherIcon(id, night)

	return &WeatherState{
		Icon:      icon,
		Desc:      desc,
		Temp:      temp,
		FeelsLike: feelsLike,
		TempMorn:  tempMorn,
		TempDay:   tempDay,
		TempEve:   tempEve,
		TempNight: tempNight,
		TempMax:   tempMax,
		UVI:       uvi,
		UVIDesc:   uviDesc,
		AQI:       aqi,
		AQIDesc:   aqiDesc,
		Rain1h:    rain1h,
		RainDay:   rainDay,
		Clouds:    weather.Current.Clouds,
		WindSpeed: windSpeed,
		BFIcon:    bfIcon,
		BFDesc:    bfDesc,
		Sunset:    sunsetTime,
		Moon:      moon,
		MoonDesc:  moonDesc,
		Night:     night,
	}
}

func roundToInt(f float64) int {
	return int(math.Round(f))
}

// ╭──────────────────────────────────────────────────────────────────────────────╮
// │ icon and description mapping                                                 │
// ╰──────────────────────────────────────────────────────────────────────────────╯

// getWeatherIcon maps an OWM condition ID to a Nerd Font glyph with day/night variants.
func getWeatherIcon(id int, night bool) string {
	if !night {
		switch {
		case id < 300:
			return " "
		case id < 500:
			return " "
		case id == 504:
			return " "
		case id < 600:
			return " "
		case id < 700:
			return " "
		case id == 711:
			return " "
		case id == 781:
			return " "
		case id < 800:
			return " "
		case id == 800:
			return " "
		case id < 803:
			return " "
		default:
			return " "
		}
	} else {
		switch {
		case id < 300:
			return " "
		case id < 500:
			return " "
		case id == 504:
			return " "
		case id < 600:
			return " "
		case id < 700:
			return " "
		case id == 711:
			return " "
		case id == 781:
			return " "
		case id < 800:
			return " "
		case id == 800:
			return " "
		case id < 803:
			return " "
		default:
			return " "
		}
	}
}

// getMoonPhase maps a 0-100 phase percentage to a glyph and label.
func getMoonPhase(phase int) (icon, desc string) {
	switch {
	case phase == 0:
		return " ", "New Moon (0%)"
	case phase <= 4:
		return " ", "Waxing Crescent (0-4%)"
	case phase <= 8:
		return " ", "Waxing Crescent (4-8%)"
	case phase <= 12:
		return " ", "Waxing Crescent (8-12%)"
	case phase <= 16:
		return " ", "Waxing Crescent (12-16%)"
	case phase <= 20:
		return " ", "Waxing Crescent (16-20%)"
	case phase <= 24:
		return " ", "Waxing Crescent (20-24%)"
	case phase == 25:
		return " ", "First Quarter (25%)"
	case phase <= 29:
		return " ", "Waxing Gibbous (25-29%)"
	case phase <= 33:
		return " ", "Waxing Gibbous (29-33%)"
	case phase <= 37:
		return " ", "Waxing Gibbous (33-37%)"
	case phase <= 41:
		return " ", "Waxing Gibbous (37-41%)"
	case phase <= 45:
		return " ", "Waxing Gibbous (41-45%)"
	case phase <= 49:
		return " ", "Waxing Gibbous (45-49%)"
	case phase == 50:
		return " ", "Full Moon (50%)"
	case phase <= 54:
		return " ", "Waning Gibbous (50-54%)"
	case phase <= 58:
		return " ", "Waning Gibbous (54-58%)"
	case phase <= 62:
		return " ", "Waning Gibbous (58-62%)"
	case phase <= 66:
		return " ", "Waning Gibbous (62-66%)"
	case phase <= 70:
		return " ", "Waning Gibbous (66-70%)"
	case phase <= 74:
		return " ", "Waning Gibbous (70-74%)"
	case phase == 75:
		return " ", "Third Quarter (75%)"
	case phase <= 79:
		return " ", "Waning Crescent (75-79%)"
	case phase <= 83:
		return " ", "Waning Crescent (79-83%)"
	case phase <= 87:
		return " ", "Waning Crescent (83-87%)"
	case phase <= 91:
		return " ", "Waning Crescent (87-91%)"
	case phase <= 95:
		return " ", "Waning Crescent (91-95%)"
	case phase <= 99:
		return " ", "Waning Crescent (95-99%)"
	default:
		return " ", "New Moon (100%)"
	}
}

// getBeaufortScale maps wind speed in knots to icon and label.
func getBeaufortScale(knots int) (icon, desc string) {
	switch {
	case knots < 1:
		return " ", "calm"
	case knots < 4:
		return " ", "light air"
	case knots < 7:
		return " ", "light breeze"
	case knots < 11:
		return " ", "gentle breeze"
	case knots < 17:
		return " ", "moderate breeze"
	case knots < 22:
		return " ", "fresh breeze"
	case knots < 28:
		return " ", "strong breeze"
	case knots < 34:
		return " ", "near gale"
	case knots < 41:
		return " ", "gale"
	case knots < 48:
		return " ", "strong gale"
	case knots < 56:
		return " ", "storm"
	case knots < 64:
		return " ", "violent storm"
	default:
		return " ", "hurricane"
	}
}

// getUVIDesc maps UV index to a WHO severity label.
func getUVIDesc(uvi int) string {
	switch {
	case uvi <= 2:
		return "low"
	case uvi <= 5:
		return "moderate"
	case uvi <= 7:
		return "high"
	case uvi <= 10:
		return "intense"
	default:
		return "extreme"
	}
}

// getAQIDesc maps OWM AQI (1-5) to a label.
func getAQIDesc(aqi int) string {
	switch aqi {
	case 1:
		return "good"
	case 2:
		return "moderate"
	case 3:
		return "poor"
	case 4:
		return "intense"
	case 5:
		return "extreme"
	default:
		return "unknown"
	}
}
