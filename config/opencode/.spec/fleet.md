# Fleet expansion: learn mode, alt-provider leaves, council doctrine

Scope approved by the user's 2026-07-04 message; user AFK, this doc is the durable brief if the session dies.
Goal: fourth primary `learn`, alt-provider leaves, council doctrine in the primaries, invented-leaf proposals.
End state: phase F critique rounds done, queued items handed to the user, this doc deleted.

## Decisions log

- Harness/agent-file edits approved by the 2026-07-04 message; drive authors directly with critic/simplify rounds before commit (`scribe/agents` not spawnable).
- Learn synthesizes the teach skill (mattpocock/skills `skills/productivity/teach/SKILL.md`, MIT) rather than copying it.
- Learn color is theme `success` green; the schema rejects raw "green".
- `.learn/` write-surface convention mirrors `.spec/`.
- Alt-provider leaves supplement mainline search; they never replace it, accepted as likely slightly less smart.
- Council restricted to read-only review/verify leaves; agreement counts only with independent evidence.
- `verify/x` uses session search tools for now: opencode 1.17.13 hardcodes xai to the Responses path and no plugin hook reaches `streamText.tools`, so native X-search is upstream-blocked (verified against anomalyco/opencode v1.17.13 source).
- Phase E declined: no new invented leaves; the 26-leaf fleet sits at the cognitive ceiling and no gap was felt across a full drive session. Revisit only when a gap is felt twice.

## Phases

### A. Research + synthesize learn mode --- done

Teach skill found and extracted; see decisions log.

### B. Author learn.md + wire config --- done

Landed as `5199aa3d` after review/critic (gpt-5.5 xhigh) + review/simplify (fable) rounds.
Blockers fixed in review: shell-mutation guard, weakened commit-sweep claim, direct-answer escape hatch, diagnose-first output, stable-theory carve-out, record shape, topic-switch guard.

### C. Alt-provider leaves --- done

Availability verified live: `opencode-go/glm-5.2` spawns with effort high; `xai/grok-build-0.1` spawns effort-less with no variants; `xai/grok-4.3` listed.
`scout/web` (pinned glm-5.2) and `verify/x` (pinned grok-4.3) landed as `902cfbec`.

### D. Council doctrine into primaries --- done

Landed in all four primaries as `afda3b9c`, refined by a two-provider council critique (gpt-5.5 xhigh + glm-5.2 high copies): pin-aware model/effort rules, council scope, evidence rule, `.learn/` sweep scoped to current thread.
Delegate tool description corrected for pinned-agent resolution.

### E. Invented-leaf proposals --- declined

See decisions log.

### F. Critique/simplify rounds --- in progress

Owns: no new files; review passes over plugins and cmds per the token-budget note (liberal claude-fable + gpt-5.5, high effort, rounds of review/critic + review/simplify).

## Next steps

1. Run phase F critique/simplify rounds across `config/opencode/` plugins and `cmds/`.
2. When F completes and queued items are handed off, delete this doc.

## Queued for user (record only, do not do)

- Restart opencode: loads usage-plugin server-path fixes, Grok CLI refresh automation, delegate permission hardening, managed-session doctrine, the learn/scout-web/verify-x agents, and the delegate description tweak.
- xAI row recovered in the user's screenshot; after restart, the usage adapter should run one noninteractive `grok models` refresh before showing `no auth` or `expired`.
- Review `config/opencode/.spec/compaction.md` for Drive managed-session, self-compaction, and `scout/session` route selection.
- Upstream opencode: X-search needs a hook or config surface for provider server-side tools; watch releases or file the ask.
- Shared-doctrine duplication (now 4 primaries + 26 leaves) wants a sync ritual through `scribe/agents`.
- Carried from `.spec/delegate.md`: build-mode guidance on xai/opencode-go models, pending real usage signals.
