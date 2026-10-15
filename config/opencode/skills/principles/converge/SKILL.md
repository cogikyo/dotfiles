---
name: converge
description: "Use when designing state-changing operations that may run again after success, interruption, or a crash."
---

# Converge

Reruns and crash recovery reach the same state.
A repeat run should compare current state with the goal, so a crash does not require reconstructing the last run's intentions.

- Check that a second run and recovery from every interruption reach the same intended state.
- Apply only the remaining difference; leave no work for an immediate repeat after success.
- Make live and stale locks, temporary files, partial records, and claimed jobs distinguishable from evidence wherever execution stops.
- For file output, write to a temporary path before renaming into place; keep leftovers attributable or safe to clean up.
- Stop if recovery depends on someone's memory or a guess about where the last run stopped.

See: `ownership`, `falsify`.
