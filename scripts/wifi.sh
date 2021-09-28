#!/usr/bin/env bash

rm -rf /var/run/wpa_supplicant/wlp1s0
wpa_supplicant -B -i wlp1s0 -c /etc/wpa_supplicant/$1.conf
echo "conecting... $1" "wifi on"

sleep 2

dhcpcd

sleep 1

echo "connected?"

exit 0
