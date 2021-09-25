#!/usr/bin/env bash

rm -rf /var/run/wpa_supplicant/wlp1s0
echo "stopping wpa_supplicant" "wifi off"
wpa_supplicant -B -i wlp1s0 -c /etc/wpa_supplicant/$1.conf
echo "conecting... psu" "wifi on"

dhcpcd

echo "connected?"

exit 0
