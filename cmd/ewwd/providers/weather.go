package providers

// ================================================================================
// OpenWeatherMap integration for conditions and forecasts
// ================================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WeatherState holds weather information from OpenWeatherMap.
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

// Weather provides weather data from OpenWeatherMap API.
type Weather struct {
	state  StateSetter
	done   chan struct{}
	active bool
	client *http.Client // Reused HTTP client for connection pooling

	lat    string
	lon    string
	apiKey string
}

// NewWeather creates a Weather provider.
func NewWeather(state StateSetter) Provider {
	return &Weather{
		state: state,
		done:  make(chan struct{}),
	}
}

func (w *Weather) Name() string {
	return "weather"
}

func (w *Weather) Start(ctx context.Context, notify func(data any)) error {
	w.active = true
	w.client = &http.Client{Timeout: 10 * time.Second}

	// Load config
	if err := w.loadConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: weather config error: %v\n", err)
		return nil // Don't fail the daemon, just skip weather updates
	}

	// Initial fetch
	if state := w.fetch(); state != nil {
		w.state.Set("weather", state)
		notify(state)
	}

	// Poll every 60 seconds
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.done:
			return nil
		case <-ticker.C:
			if state := w.fetch(); state != nil {
				w.state.Set("weather", state)
				notify(state)
			}
		}
	}
}

func (w *Weather) Stop() error {
	if w.active {
		close(w.done)
		w.active = false
	}
	return nil
}

func (w *Weather) loadConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	// Read location: "LAT LON"
	locPath := filepath.Join(home, ".local", ".location")
	locData, err := os.ReadFile(locPath)
	if err != nil {
		return fmt.Errorf("read location: %w", err)
	}
	parts := strings.Fields(string(locData))
	if len(parts) < 2 {
		return fmt.Errorf("invalid location format: expected 'LAT LON'")
	}
	w.lat = parts[0]
	w.lon = parts[1]

	// Read API key
	keyPath := filepath.Join(home, ".local", ".owm_api_key")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("read api key: %w", err)
	}
	w.apiKey = strings.TrimSpace(string(keyData))

	return nil
}

func (w *Weather) fetch() *WeatherState {
	// Fetch weather data
	weatherURL := fmt.Sprintf(
		"https://api.openweathermap.org/data/3.0/onecall?lat=%s&lon=%s&units=metric&exclude=minutely&appid=%s",
		w.lat, w.lon, w.apiKey,
	)
	weatherData, err := w.httpGet(weatherURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: weather fetch error: %v\n", err)
		return nil
	}

	// Fetch air quality data
	airURL := fmt.Sprintf(
		"http://api.openweathermap.org/data/2.5/air_pollution?lat=%s&lon=%s&appid=%s",
		w.lat, w.lon, w.apiKey,
	)
	airData, err := w.httpGet(airURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: air quality fetch error: %v\n", err)
		return nil
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

// OpenWeatherMap API response structures
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
	if err := json.Unmarshal(airData, &air); err != nil {
		fmt.Fprintf(os.Stderr, "ewwd: air parse error: %v\n", err)
		return nil
	}

	// Validate response
	if len(weather.Current.Weather) == 0 || len(weather.Daily) == 0 {
		return nil
	}

	// Determine if night
	now := time.Now().Unix()
	sunset := weather.Current.Sunset
	night := now > sunset

	// Weather ID for icon selection
	id := weather.Current.Weather[0].ID
	desc := weather.Current.Weather[0].Main

	// Convert temperatures (round to nearest int)
	temp := roundToInt(weather.Current.Temp)
	feelsLike := roundToInt(weather.Current.FeelsLike)
	tempMorn := roundToInt(weather.Daily[0].Temp.Morn)
	tempDay := roundToInt(weather.Daily[0].Temp.Day)
	tempEve := roundToInt(weather.Daily[0].Temp.Eve)
	tempNight := roundToInt(weather.Daily[0].Temp.Night)
	tempMax := roundToInt(weather.Daily[0].Temp.Max)

	// UVI
	uvi := roundToInt(weather.Current.UVI)
	uviDesc := getUVIDesc(uvi)

	// AQI
	aqi := 0
	if len(air.List) > 0 {
		aqi = air.List[0].Main.AQI
	}
	aqiDesc := getAQIDesc(aqi)

	// Rain probability
	rain1h := 0
	if len(weather.Hourly) > 0 {
		rain1h = int(weather.Hourly[0].Pop * 100)
	}
	rainDay := int(weather.Daily[0].Pop * 100)

	// Wind
	windSpeed := roundToInt(weather.Current.WindSpeed)
	knots := roundToInt(weather.Current.WindSpeed * 1.943844)
	bfIcon, bfDesc := getBeaufortScale(knots)

	// Sunset time formatted
	sunsetTime := time.Unix(sunset, 0).Format("15:04")

	// Moon phase
	moonPhase := int(weather.Daily[0].MoonPhase * 100)
	moon, moonDesc := getMoonPhase(moonPhase)

	// Weather icon
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

// getWeatherIcon returns weather icon based on condition ID and day/night.
// https://openweathermap.org/weather-conditions#How-to-get-icon-URL
func getWeatherIcon(id int, night bool) string {
	if !night {
		// Day icons
		switch {
		case id < 300:
			return " " // thunderstorm
		case id < 500:
			return " " // drizzle
		case id == 504:
			return " " // extreme rain
		case id < 600:
			return " " // rain
		case id < 700:
			return " " // snow
		case id == 711:
			return " " // smoke
		case id == 781:
			return " " // tornado
		case id < 800:
			return " " // atmosphere (fog, mist, etc.)
		case id == 800:
			return " " // clear
		case id < 803:
			return " " // few clouds
		default:
			return " " // cloudy
		}
	} else {
		// Night icons
		switch {
		case id < 300:
			return " " // thunderstorm
		case id < 500:
			return " " // drizzle
		case id == 504:
			return " " // extreme rain
		case id < 600:
			return " " // rain
		case id < 700:
			return " " // snow
		case id == 711:
			return " " // smoke
		case id == 781:
			return " " // tornado
		case id < 800:
			return " " // atmosphere (fog, mist, etc.)
		case id == 800:
			return " " // clear night
		case id < 803:
			return " " // few clouds night
		default:
			return " " // cloudy
		}
	}
}

// getMoonPhase returns moon icon and description based on phase (0-100).
func getMoonPhase(phase int) (icon, desc string) {
	switch {
	case phase == 0:
		return " ", "New Moon (0%)"
	case phase <= 4:
		return " ", "Waxing Crescent (0-4%)"
	case phase <= 8:
		return " ", "Waxing Crescent (4-8%)"
	case phase <= 12:
		return " ", "Waxing Crescent (8-12%)"
	case phase <= 16:
		return " ", "Waxing Crescent (12-16%)"
	case phase <= 20:
		return " ", "Waxing Crescent (16-20%)"
	case phase <= 24:
		return " ", "Waxing Crescent (20-24%)"
	case phase == 25:
		return " ", "First Quarter (25%)"
	case phase <= 29:
		return " ", "Waxing Gibbous (25-29%)"
	case phase <= 33:
		return " ", "Waxing Gibbous (29-33%)"
	case phase <= 37:
		return " ", "Waxing Gibbous (33-37%)"
	case phase <= 41:
		return " ", "Waxing Gibbous (37-41%)"
	case phase <= 45:
		return " ", "Waxing Gibbous (41-45%)"
	case phase <= 49:
		return " ", "Waxing Gibbous (45-49%)"
	case phase == 50:
		return " ", "Full Moon (50%)"
	case phase <= 54:
		return " ", "Waning Gibbous (50-54%)"
	case phase <= 58:
		return " ", "Waning Gibbous (54-58%)"
	case phase <= 62:
		return " ", "Waning Gibbous (58-62%)"
	case phase <= 66:
		return " ", "Waning Gibbous (62-66%)"
	case phase <= 70:
		return " ", "Waning Gibbous (66-70%)"
	case phase <= 74:
		return " ", "Waning Gibbous (70-74%)"
	case phase == 75:
		return " ", "Third Quarter (75%)"
	case phase <= 79:
		return " ", "Waning Crescent (75-79%)"
	case phase <= 83:
		return " ", "Waning Crescent (79-83%)"
	case phase <= 87:
		return " ", "Waning Crescent (83-87%)"
	case phase <= 91:
		return " ", "Waning Crescent (87-91%)"
	case phase <= 95:
		return " ", "Waning Crescent (91-95%)"
	case phase <= 99:
		return " ", "Waning Crescent (95-99%)"
	default:
		return " ", "New Moon (100%)"
	}
}

// getBeaufortScale returns Beaufort scale icon and description based on wind speed in knots.
func getBeaufortScale(knots int) (icon, desc string) {
	switch {
	case knots < 1:
		return " ", "calm"
	case knots < 4:
		return " ", "light air"
	case knots < 7:
		return " ", "light breeze"
	case knots < 11:
		return " ", "gentle breeze"
	case knots < 17:
		return " ", "moderate breeze"
	case knots < 22:
		return " ", "fresh breeze"
	case knots < 28:
		return " ", "strong breeze"
	case knots < 34:
		return " ", "near gale"
	case knots < 41:
		return " ", "gale"
	case knots < 48:
		return " ", "strong gale"
	case knots < 56:
		return " ", "storm"
	case knots < 64:
		return " ", "violent storm"
	default:
		return " ", "hurricane"
	}
}

// getUVIDesc returns UV index description.
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

// getAQIDesc returns Air Quality Index description.
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
