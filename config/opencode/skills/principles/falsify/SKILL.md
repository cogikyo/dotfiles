---
name: falsify
description: "Use when preparing to claim a result, report a measurement, declare work verified, or rule an option out."
---

# Falsify

A claim needs a check on the real thing that could have failed.
A real result needs a direct observation, and a measured number also needs an explanation of its limits.

- Inspect the actual output, running behavior, or stored value; reject proxies and completion reports as proof.
- Before ruling an option out, run the cheap read-only probe that would confirm the premise it rests on.
- Choose the smallest check that would expose a wrong change, and leave its script and result in scratch (`/tmp/opencode` or `$OPENCODE_SCRATCH`) for review.
- When a check fails, inspect the observation method first; confirm the artifact and behavior without dismissing the failure.
- Identify the limiting resource or path during the run; keep it with the number, run count, spread, and supporting evidence.
- Rule out cached or skipped work, errors, noise, unequal settings, and a measurement too narrow for the claim.
- Withhold an unchecked or unexplained claim and name what remains unverified.

See: `mechanize`.
