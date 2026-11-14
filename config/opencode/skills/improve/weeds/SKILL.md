---
name: weeds
description: Use when the user invokes /weeds or asks to hunt the code problems models keep writing and make them impossible with lints, types, or reshapes. Collab owns it. It learns across repositories through one shared checklist in dotfiles, its only write besides approved fixes, even when the examined repository is dotfiles.
---

# Weeds

Apply `mechanize` in Propose: a problem seen again gets an enforcer, not another reminder.

A weed is a class of code problem that models write again and again, across repositories.
A run finds weeds in the current repository, proposes the enforcement that makes each one impossible, and records what it learned in one checklist that every repository shares.

## Authority

Collab owns this procedure and every write it makes.
Treat `$ARGUMENTS` as extra instructions, such as one entry to focus on or a path to hunt in.
Invoking `/weeds` approves only the checklist write and a one-file commit of it in the dotfiles checkout, as `/improve` approves its tag writes.
Fixes in the examined repository need the user's yes on each proposal; besides them, the run writes only the shared checklist, even when the examined repository is dotfiles.
In a sensitive repository, as `COLLAB.md`'s Boundaries defines it, show the checklist diff with the proposals and write it only after the user's yes.

## Checklist

The checklist at `~/.config/opencode/skills/improve/weeds/checklist.md` is the only memory, and a `/weeds` run is its only writer.
Reviewers read it by path from any repository, because `COLLAB.md` puts the path in every code-review brief.

An entry is a heading and five field lines:

- The **heading** holds the kebab-case name, which is also the reviewer label, and the languages.
- **Problem:** one sentence.
- **Spot:** a search pattern or tool where one exists, otherwise one sentence.
- **Kill:** strongest first: reshape the code, then a lint or type, then a test, naming known rules per language.
- **Seen:** each model tag with its last-seen date, plus an `untagged` date when a sighting added no tag.
- **Judged:** the date of the last seen-or-quiet verdict, and the count of quiet runs in a row.

A candidate is one line: its name, languages, one sentence, and the date a run first saw it.

The file names no repository and stores no repository count.
_Private repositories cannot be named, so a stored count could never tell whether the current repository was already counted._
No code, identifier, path, or business term from a private repository goes in it, because dotfiles is public.

## Rules that keep it short

- **Admit** an entry only when it was seen in two or more repositories and has a kill above prose.
  - A problem that only a sentence can address is not a weed for this list.
- Keep each entry to **six lines**: its heading and five field lines.
  - Learn replaces spot and kill text; it never appends, and records nothing specific to one repository.
- Cap entries at **12**; at the cap, a new entry displaces the one seen longest ago.
  - Break ties by name order.
- Cap candidates at **8**; at the cap, drop the one first seen longest ago.
- **Promote** a candidate at its second repository, once it meets the admission rule.
- **Expire** a candidate 30 days after it was first seen; 30 days is a first guess.
- **Retire** an entry after two quiet runs in a row.

## Verdicts

Each run gives every entry one verdict:

- **Seen:** a review report since the entry's `judged` date carried its label.
- **Quiet:** at least 20 code reviews in the entry's languages carried no label for it.
  - Each counted review was created after the entry's `judged` date, so two runs on one day never count the same reviews.
  - Each counted review was briefed with the checklist path.
  - Each reviewed code whose code-writing lanes all ran the current code model.
  - The floor of 20 is a first guess.
- **No evidence:** fewer than 20 such reviews; write nothing, so the next run counts from the same `judged` date.

A review counts as briefed when its first user message contains `weeds/checklist.md`.
Judge a review's languages from its brief and report.

## Model tags

A tag means the entry was seen in a review of code whose code-writing lanes all ran that model.
It does not claim the model wrote the flagged line.
A review's code-writing lanes are the `build/general`, `build/patch`, and `build/owner` sessions anywhere under its top-level session, created before the review.
Walk the review's parent links up to that top-level session.
_A `verify/decide` checkpoint or `build/owner` launches most reviews, so the review's own parent usually has no code lanes._
Read each lane's model from its session record.
When those lanes ran more than one model, or the tree has no code lane, the sighting counts as seen and updates `untagged`.

The current code model is the model on the `build/general` line of `ROUTING.md`.
When it changes, entries keep their old tags and are judged under the new model by the same rule, so the list turns over with the model.

Every first entry was seen under one model, `claude-opus-5-5`, at lane level, so the tags cannot yet show which problems belong to a model.
That becomes testable when the builder route changes model.

## Collect

Gather `weed: <name>` labels from every repository with paged `sessions` `sql` queries, 20 rows per call.
Read assistant-role text only, so a brief that quotes a label does not count as a sighting.
Read from each entry's `judged` date, and from each candidate's first-seen date.

- Entry verdicts and tags come from `review/*` reports.
- A candidate's repositories come from labelled sightings in any session, including a `/weeds` run's own report.
- Resolve each sighting's session directory with `git rev-parse --path-format=absolute --git-common-dir`; distinct common directories are distinct repositories, so worktrees of one repository count once.
- Count briefed reviews for each entry's quiet test in the same pass.

A `scout/session` one-shot can run the pull when it would take more than about ten calls; brief it with this section and Model tags.

## Hunt

Hunt read-only in the current repository.

- For each entry whose languages the repository uses, run its Spot patterns.
- Look for new repeated smells: fix and revert commits, growing lint suppressions, workaround comments, and near-duplicate helpers.
- `review/entropy` can run the hunt with its own scouts; give it the checklist path.
- Write `weed: <name>` in the run's report for each find, candidates included, so later runs can count this repository.
- Add a new class to the candidates when no entry or candidate covers it and a kill above prose looks possible.

## Propose

For each find, name the strongest enforcement this repository lacks, preferring a tool it already uses.

- A new check must fail on a real instance in the repository.
- Where instances are widespread, the check fails only on additions.

Return ranked proposals and a workflow, as `improve` does.
Builders implement only the proposals the user approves.

## Learn

Re-read the checklist immediately before writing it, then apply the verdicts:

- **Seen:** update the tag's last-seen date or `untagged`, set `judged` to today, and reset the quiet count to 0.
- **Quiet:** set `judged` to today and add one quiet run; retire the entry at two.
- **No evidence:** change nothing for that entry.
- Promote, expire, and displace by the rules above.
- Replace an entry's spot or kill text with any that worked better in this run.

Commit the checklist alone in the dotfiles checkout.

## Report

- Scope, the review counts behind each verdict, and coverage gaps.
- Each entry's verdict, and candidates added, promoted, or expired.
- Ranked proposals, kept apart from approved changes actually applied.
- The checklist diff, or the commit that holds it.

Report any of these signs that the method is wrong:

- The list sits at its cap with no retirements.
- Reviewers' `weed:` flags are rejected more often than accepted.
- Candidates rarely reach a second repository.
- After a builder model change, the same entries stay seen, which makes the tags noise.
