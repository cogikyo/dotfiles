# TUI plugins

Each TUI plugin is either re-anchored to a verified V2 surface or deleted because V2 renders the feature natively; native rendering always wins over carried-over patching.

## Port policy

- Every anchor point is an interface verified present on the pinned revision; names the beta still moves are treated as hypotheses until probed in source.
- The usage sidebar keeps its provider headroom display and its muted presentation of stale windows.
- The statusline keeps working directory, git status, and context pressure.
- Sidebar sections keep their slot ownership and ordering, including deactivation of the conflicting internal section.
- Modified files, Markdown context, and media context remain available through ported plugins or verified native equivalents with the same user-visible information.
- The Kitty context writer keeps publishing which tab owns each session.
- Code block readability and language support remain equivalent; custom patching survives only when it re-anchors to a verified V2 rendering surface with visible drift detection, otherwise a demonstrated native renderer owns the behavior.
- TUI runtime dependencies move to V2-compatible versions.

## Failure behavior

- A plugin that fails to load, or whose anchor point moved under it, degrades only its own chrome and announces itself visibly; silent absence is a defect.

## Acceptance

- Every surviving plugin renders in a V2 tab, and every deleted plugin's native replacement is demonstrated once.
