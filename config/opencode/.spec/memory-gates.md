# Memory gates

Memory pressure motivates the migration, so two footprint gates decide whether it continues; both are measured by one reproducible protocol so the numbers can be compared honestly.

## Measurement protocol

- Memory is measured as PSS summed across each process tree, read from the kernel's per-process PSS rollup accounting.
- The server tree and each client tree are measured separately and then summed; PSS attributes shared pages proportionally so the sum is honest.
- No other agent backend or client runs during a measurement, so unrelated processes never share pages into the result.
- Every measurement is warm: the session has loaded real project context and completed tool calls, then sits quiescent with no generation in flight.
- Each reported number is the median of at least five samples that agree within a small band; a drifting reading is retaken after settling, never averaged away.
- Clients are measured as full TUIs attached to a project session, matching real tab usage.
- Every reported number names the V2 revision, the machine, and the branch state that produced it; a result is valid only for that combination.
- V1's baseline is already recorded: server about 580 MiB, five attach clients about 234 to 304 MiB each, about 1.82 GiB total, dominated by JavaScriptCore private allocations.

## Bare client gate

- The first measurement records the plugin-free V2 server and one plugin-free V2 TUI separately under warm load, taken on the migration branch before plugin porting.
- A client at or below 220 MiB: the migration continues.
- A client at or above 260 MiB: the memory-motivated migration stops and the default branch remains the workflow.
- Between the bounds: project five clients using the measured bare server and continue only if the projection clears the full stack target.

## Full stack gate

- The final measurement is the plugin-loaded server plus five tabs running the daily workflow on the migration branch.
- At or below 1.4 GiB total: the migration delivers its purpose.
- At or above 1.6 GiB: the benefit is insufficient and the branch does not land, regardless of functional success.
- Between the bounds, the decision weighs measured stability and feature gains against the memory saved.

## Decision rules

- Gates veto in both directions: functional parity never excuses a failed footprint, and a passing footprint never excuses broken behavior.
- An abandoned migration leaves the default branch untouched as the workflow and stops further porting work.
