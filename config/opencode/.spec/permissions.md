# Permission parity

V2 enforces the fleet's security and unattended-execution boundaries with behavior proven against recorded V1 outcomes, so renamed tools or changed matching semantics never silently widen authority.

## Policy mapping

- Recursive deletion, privilege escalation, publication, history rewriting, ownership changes, package installation, secret paths, and external-directory boundaries retain their allow, ask, and deny intent.
- New or renamed V2 permission surfaces are mapped explicitly; any surface without a deliberate rule is denied.
- The effective V2 policy has a mechanically verified default-deny boundary for unrecognized tools and permission names.

## Behavioral probes

- Representative allowed, asked, and denied operations are recorded under V1 and replayed under V2 after the branch switch.
- Rule ordering and overlapping patterns are probed directly because successful schema loading is not evidence of equivalent resolution.
- Primary-mode overrides and child-profile restrictions are included in the probe set.

## Failure behavior

- Any probe that becomes more permissive blocks landing.
- A V1 rule with no enforceable V2 expression remains a named blocker rather than becoming advisory policy.

## Acceptance

- The probe set produces outcomes identical to or stricter than the V1 reference, and every stricter result is reviewed for intentional usability impact.
