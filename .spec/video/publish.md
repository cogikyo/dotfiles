# Publishing and feedback

Publishing is a manifest-driven distribution stage, and analytics close the loop that makes the channel's style hypotheses falsifiable.
YouTube is the first distribution membrane; the same script's sibling phenotypes (essay, audio, explorable) are candidate members of this stage once the multi-phenotype decision lands.
The platform boundary is sharp: uploads, metadata, captions, and thumbnails are API territory; A/B tests, cards, end screens, and auto-dub review are Studio-only human checklist items.

## Upload automation

- A small Go publisher over the official Data API owns upload, metadata, thumbnail, captions, and playlist from a publish manifest; resumable sessions persist and survive interruption.
- Quota reality (verified 2026-07): uploads live in their own default 100-per-day bucket at one unit each.
- Privacy reality (verified 2026-07): uploads from unverified API projects created after 2020-07-28 are restricted to private viewing until the project passes a YouTube API audit.
- The publish manifest records audit status, intended privacy, observed privacy after upload, and any private-to-public promotion; confirmed upload actions become publication records attached to the release.
- Public or scheduled publication is always a human-confirmed external effect.
- An existing Go uploader CLI covers the interim; owning the thin client is the durable end state.

## Packaging

- Chapters render from timeline markers into the description (ascending timestamps, ≥3 chapters, ≥10s each); no API object exists and none is needed.
- Captions upload as sidecar SRT/VTT derived from the program caption projection; burned-in styling is reserved for stylized segments, sidecars carry accessibility.
- Thumbnail candidates generate locally against the genome's locked spec; one uploads at publish, and real experiments run through Studio's native Test & Compare (three variants, watch-time share) which the API cannot reach.
- Disclosure flags set at upload: synthetic-media when realistic generated content appears, made-for-kids (almost always false here), license.

## Shorts derivatives

- Shorts are ordinary vertical uploads clipped from markers or transcript segments, reframed and re-captioned locally; no Shorts-specific API metadata exists.
- Semantic best-moment selection stays editorial: agents propose clip candidates, the human promotes.

## Analytics loop

- The Analytics API returns 100-point normalized retention curves per video (verified, no channel-size gate); authored segment transitions map into the same coordinates, so mode-switch effects are measurable within a video from video one.
- Resolution honesty: 100 points on a ten-minute video is ~6 seconds per point, which blurs 1–3s inserts, and small-channel view counts add noise; early findings are exploratory indicators, never verdicts.
- This remains the falsification instrument for the genome hypotheses, within-video comparisons first, cross-video arms dormant until an audience floor exists.
- Snapshot cadence: day 1, 3, 7, 28 per video; the bulk Reporting API becomes a local warehouse only when longitudinal volume justifies it.
- Small-channel privacy suppression limits traffic-source detail early; retention and watch time remain usable from video one.
- Organic impressions/CTR appear API-inaccessible; Studio export covers that gap until a live test proves otherwise.

## SEO reality

- Watch time, retention, and title/thumbnail promise are the levers; tags are vestigial and metadata tooling suites are idea-search conveniences, never pipeline dependencies.
- Platform policy pressure targets mass-produced inauthentic content; a heavily automated pipeline publishing genuinely authored videos is fine, and the distinction lives in the writing stage, not the upload stage.

## Open questions

- When to trigger the API project audit: before the first video, or after a handful of manual publishes prove the pipeline.
- Whether retention-versus-segment analysis lives in the orchestrator or stays a notebook exercise until H1 has data.
- Auto-dub posture once eligible: review-and-publish, or ignore entirely.
