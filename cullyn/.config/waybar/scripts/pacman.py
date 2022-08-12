#!/usr/bin/env python

import json
import subprocess


packages = subprocess.run(["paru", "-Qu"], capture_output=True)
upgrade_count = (
    subprocess.run(["wc", "-l"], input=packages.stdout, capture_output=True)
    .stdout.decode("utf8")
    .strip()
)

tooltip = packages.stdout.decode("utf8").strip()

if int(upgrade_count) > 10:
    tooltip = "lots to display; run 'paru -Qt' for more info"

if int(upgrade_count) == 0:
    upgrade_count = " "
status = "packages"

out_data = {
    "text": f" <sup>{upgrade_count}</sup>",
    "alt": status,
    "tooltip": tooltip,
    "class": "pacman",
}
print(json.dumps(out_data))
