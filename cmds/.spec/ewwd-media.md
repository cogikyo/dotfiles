# Event-driven media actions

The Spotify widget reconciles transport, volume, and seek actions through the smallest authoritative MPRIS feedback path, so interaction feels immediate without duplicating full metadata polls.

## Playback authority

The player follow stream owns playback status, player volume, track identity, and metadata.
The one-second position poll remains because MPRIS does not emit continuous progress.
That poll reads position only and derives progress from the cached duration for the current track.

Follow-stream exit publishes stopped or unavailable state as appropriate, reconnects with bounded backoff, and takes a fresh snapshot after recovery.
Only one serialized owner mutates and publishes the last media snapshot.

## Action feedback

Play, pause, toggle, next, previous, and player-volume actions issue one mutation and reconcile through the follow stream.
They do not trigger a redundant full metadata read after success.

Seek is the exception because position has no follow event.
A successful seek performs one targeted position read and publishes the resulting progress while leaving all other metadata untouched.
The regular position poll then resumes authority.

Each track change and seek advances a revision.
Position reads publish only when their captured track and seek revisions still match current state, so an older poll or targeted read cannot undo a newer seek or cross a track boundary.

Action failures preserve the last authoritative snapshot and return an explicit error.
No action predicts track identity, playback status, or volume before MPRIS confirms it.

Album art and Canvas work follows track identity changes from the serialized state owner.
Late downloads from an obsolete track cannot overwrite the current track's visual state.
Player mutations, targeted reads, and follow-process operations inherit cancellation and have bounded execution.

## Interaction acceptance

- Play, pause, next, previous, and player-volume changes appear without waiting for the position tick.
- Seek updates progress immediately after one targeted position read.
- No successful action launches a redundant full-state query.
- The periodic poll reads position only and never reconstructs metadata, playback status, or player volume.
- Rapid volume and seek scrolling converges on authoritative player state without stale snapshots.
- Follow-stream restart recovers current metadata without restarting ewwd.
- Progress continues smoothly at one-second resolution between interactions.
- Track changes cannot display album art or Canvas frames from an older in-flight request.
- Position and seek results from obsolete track or action revisions are discarded.
