# Cutover

The migration branch lands only after memory and functional gates pass and a soak of real work proves the workflow on the branch; rollback is a branch switch plus restart before landing and a revert plus restart after.

## Preconditions

- Both memory gates passed on the current V2 pin.
- Functional gates are green: fleet and permission parity probes, delegation behaviors, notification routing, and five-tab topology all demonstrated on the migration branch.
- A soak of at least five consecutive working days of normal use completes on the migration branch without switching back for capability reasons.

## Landing

- Landing is merging the migration branch into the default branch; the ported configuration, plugins, dependencies, service definition, and launch commands become the default.
- V1 storage stays in place after landing, untouched and readable by a reinstalled V1.
- The pinned V1 binary remains installed through an explicit post-landing confidence period; removing it is a separate choice that changes rollback from a restart operation into a reinstall operation.

## Rollback

- Before landing, rollback is switching back to the default branch with the full restart boundary; the complete V1 workflow and history return immediately.
- After landing, rollback is reverting the merge with the same restart boundary; version control preserves the V1 implementation and V1 storage reopens its history.
- If the pinned V1 binary has already been removed, rollback reinstalls that exact version before restarting.
- Rollback never requires running both runtimes at once.

## Failure behavior

- Any gate failure during the soak switches the daily workflow back to the default branch the same day; the migration resumes only once the failing invariant holds.

## Acceptance

- After landing, the default branch serves the five-tab workflow on V2, and a revert plus restart restores the V1 workflow with its history intact.
