#!/usr/bin/env python

from pyquery import PyQuery  # install using `pip install pyquery`
import json

# weather icons
weather_icons = {
    "sunnyDay": "滛",
    "clearNight": "望",
    "cloudyFoggyDay": "",
    "cloudyFoggyNight": "",
    "rainyDay": "",
    "rainyNight": "",
    "snowyIcyDay": "",
    "snowyIcyNight": "",
    "severe": "",
    "default": "",
}

# get location_id
# to get your own location_id, go to https://weather.com & search your location.
# once you choose your location, you can see the location_id in the URL(64 chars long hex string)
# like this: https://weather.com/en-IN/weather/today/l/c3e96d6cc4965fc54f88296b54449571c4107c73b9638c16aafc83575b4ddf2e
location_id = "7fd2c8d5fd5c6067bccab7955d643755478de23380fa548b657e63b2765cf519"

# priv_env_cmd = 'cat $PRIV_ENV_FILE | grep weather_location | cut -d "=" -f 2'
# location_id = subprocess.run(
#     priv_env_cmd, shell=True, capture_output=True).stdout.decode('utf8').strip()

# get html page
url = "https://weather.com/en-IN/weather/today/l/" + location_id
html_data = PyQuery(url)

# current temperature
temp = html_data(".CurrentConditions--tempValue--3a50n").text()

# current status phrase
status = html_data(".CurrentConditions--phraseValue--2Z18W").text()
status = f"{status[:16]}.." if len(status) > 17 else status

# status code
status_code = html_data("#regionHeader").attr("class").split(" ")[2].split("-")[2]

# status icon
icon = (
    weather_icons[status_code]
    if status_code in weather_icons
    else weather_icons["default"]
)

# prediction phrase
prediction = html_data(".CurrentConditions--precipValue--3nxCj").text()
prediction = prediction.replace("chance of rain", "  ")
prediction = f"\n\n   {prediction}" if len(prediction) > 0 else prediction

# temperature feels like
temp_feel = html_data(".TodayDetailsCard--feelsLikeTempValue--Cf9Sl").text()
temp_feel_text = f"Feels like {temp_feel}c"

# min-max temperature
temp_max = html_data(
    ".CurrentConditions--tempHiLoValue--3SUHy > span:nth-child(1)"
).text()
temp_min = html_data(
    ".CurrentConditions--tempHiLoValue--3SUHy > span:nth-child(2)"
).text()
temp_min_max = f" {temp_min}\t\t {temp_max}"

# temperature distribution (day)
temp_morning = html_data(
    ".WeatherTable--wide--3dFXu > li:nth-child(1) > a:nth-child(1) > div:nth-child(2) > span:nth-child(1)"
).text()
temp_afternoon = html_data(
    ".WeatherTable--wide--3dFXu > li:nth-child(2) > a:nth-child(1) > div:nth-child(2) > span:nth-child(1)"
).text()
temp_evening = html_data(
    ".WeatherTable--wide--3dFXu > li:nth-child(3) > a:nth-child(1) > div:nth-child(2) > span:nth-child(1)"
).text()
temp_overnight = html_data(
    ".WeatherTable--wide--3dFXu > li:nth-child(4) > a:nth-child(1) > div:nth-child(2) > span:nth-child(1)"
).text()
temp_dist = f"🌄 {temp_morning} 🔆 {temp_afternoon} 🌃 {temp_evening} 🌘 {temp_overnight}"

# sunrise and sunset time
time_to_sunrise = html_data(
    "#SunriseSunsetContainer-fd88de85-7aa1-455f-832a-eacb037c140a > div > div > div > div:nth-child(1) > p"
).text()
time_to_sunset = html_data(
    "#SunriseSunsetContainer-fd88de85-7aa1-455f-832a-eacb037c140a > div > div > div > div:nth-child(2) > p"
).text()
time_sunrise_sunset = f"  {time_to_sunrise} |   {time_to_sunset}"

# air quality index
air_quality_index = html_data(
    "#WxuAirQuality-sidebar-aa4a4fb6-4a9b-43be-9004-b14790f57d73 > div > section > div > div > div:nth-child(1) > svg > text"
).text()


def get_air_quality_value_icon(air_quality_index):
    val = int(air_quality_index)
    res = "?"
    if val > 0 & val <= 50:
        res = "🟢"
    elif val > 50 & val <= 100:
        res = "🟡"
    elif val > 100 & val <= 200:
        res = "🟠"
    elif val > 150 & val <= 200:
        res = "🔴"
    elif val > 200 & val <= 300:
        res = "🟣"
    else:
        res = "☣️"
    return res


air_quality_icon = get_air_quality_value_icon(air_quality_index)
air_quality_text = f"{air_quality_index}|<small>{air_quality_icon}</small>"

# wind details
wind_speed = html_data(".Wind--windWrapper--3aqXJ").text().split("\n")[1]
wind_text = f"煮  {wind_speed}"

# visibility
visbility = html_data(
    "div.ListItem--listItem--2wQRK:nth-child(7) > div:nth-child(3) > span:nth-child(1)"
).text()
visbility_text = f"  {visbility}"

# humidity
humidity = html_data(
    "div.ListItem--listItem--2wQRK:nth-child(3) > div:nth-child(3) > span:nth-child(1)"
).text()
humidity_text = f" {humidity}"

# moon phase
moon_phase = html_data(
    "div.ListItem--listItem--2wQRK:nth-child(8) > div:nth-child(3)"
).text()


def get_mooon_phase_icon(moon_phase):
    match moon_phase:
        case "New Moon":
            return ""
        case "Waning Crescent":
            return ""
        case "Third Quarter":
            return ""
        case "Waning Gibbous":
            return ""
        case "Full Moon":
            return ""
        case "Waxing Gibbous":
            return ""
        case "First Quarter":
            return ""
        case "Waxing Crescent":
            return ""
        case _:
            return "?"


moon_phase_icon = get_mooon_phase_icon(moon_phase)


# humidity
uv_index = (
    html_data(
        "div.ListItem--listItem--2wQRK:nth-child(6) > div:nth-child(3) > span:nth-child(1)"
    )
    .text()
    .replace(" of ", "/")
)
uv_index_text = f"履  {uv_index}"


# tooltip text
tooltip_text = str.format(
    "{}\t\t{} {}\n{}\n{}\n{}\n{}\n{}{}",
    f"<big>{status}</big>",
    f"<span size='xx-large'>{moon_phase_icon}</span>",
    f"<small><i>{moon_phase}</i></small>",
    f"<small>{temp_feel_text}</small>",
    f"    ⮩ <small><i>{temp_dist}</i></small>\n",
    f"<span size='xx-large'>{time_sunrise_sunset}</span>\n",
    f"{wind_text} \t{humidity_text}",
    f"{uv_index_text}  \t {air_quality_text}",
    f"<i>{prediction}</i>",
)

# tooltip_text += f'{temp_dist_icons}\n{temp_dist}\n\n{time_sunrise_sunset}'

# print waybar module data
out_data = {
    "text": f"{icon} {temp}",
    "alt": status,
    "tooltip": tooltip_text,
    "class": status_code,
}
print(json.dumps(out_data))
