# Backend service

The V2 backend is one long-lived shared service owned by a systemd user unit, preserving the existing resource and restart policy with honest semantics for what a crash destroys.

## Lifecycle ownership

- The systemd user unit is the sole backend lifecycle owner because the native managed service does not provide the required scheduling and resource controls.
- Native managed-service startup stays disabled, so no second owner can spawn or replace the backend.
- The unit provides restart-on-failure, deprioritized scheduling, and user-session activation, matching the V1 service contract.
- The service definition lives in the repository, so each branch owns the unit for its own runtime.

## Resource policy

- The backend runs deprioritized relative to interactive work, mirroring the V1 unit's nice level, CPU and IO weights, and quota, unless a measured need justifies divergence.

## Restart semantics

- Graceful restarts suspend and continue existing sessions through the runtime's native continuation behavior.
- A hard crash destroys process-local job state; in-flight background children do not survive, and the runtime is not assumed to deliver any error to their parents.
- After any crash, work that was in flight is unknown until reconciled: durable child output proves completion, while absence of output never does.
- A client-side reconstruction that labels a vanished child complete without durable completion output is treated as unknown work rather than success.
- Session data persists across restarts; only volatile job state is forfeited.

## Failure behavior

- Repeated crashes trip the same restart policy as the V1 unit.
- The unit passes an explicit loopback address and port, so a bind collision is a hard startup error rather than an automatic scan to another port.

## Acceptance

- Kill the backend during an in-flight background child and restart it: sessions reopen, durable completed output remains distinguishable from vanished work, and the unknown child can be re-dispatched without duplicate success being inferred.
