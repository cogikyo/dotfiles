# Review

Reusable code review workflow for current chats and review agents.

## Prime Directive

Find real risks first.
Prefer falsifiable findings over broad opinions.
Return partial results instead of stalling on missing permission, unclear scope, or unavailable tools.

## Commands

### `/review`

Review the inferred or specified scope.

1. Determine scope before reviewing.
2. Use the smallest scope that can answer the request.
3. Ask one short scope question only when inference is ambiguous and the choice changes review work.
4. Inspect code, diffs, and nearby context as needed.
5. Return findings first, ordered by severity, with file and line references when available.
6. Include open questions only when they would change the finding or fix.
7. Include a concise fix plan only for findings worth fixing.

Default scope options:
- **Branch**: review commits or diff ahead of upstream/base branch.
- **Dirty**: review staged and unstaged working-tree changes.
- **Blast radius**: review dirty and/or branch changes plus nearby callers, owners, configs, tests, docs, and runtime seams affected by those changes.
- **Module**: review a specific package, directory, component, or file as it exists now, regardless of dirty or branch state.

Scope inference:
- If the user names paths, packages, modules, or components, use **Module** unless they ask for changed code only.
- If the user says staged, unstaged, dirty, working tree, or WIP, use **Dirty**.
- If the user says branch, PR, commits, ahead of origin, merge base, or compare to main, use **Branch**.
- If the user asks for risk, regression, integration, architecture, or broad review of a change, use **Blast radius**.
- If both branch commits and dirty files exist and the user just says `/review`, ask whether to review Branch, Dirty, or Blast radius.
- If only dirty changes exist, default to **Dirty**.
- If only branch-ahead changes exist, default to **Branch**.
- If neither dirty nor branch-ahead changes exist, ask for a module/path unless the current conversation already supplies one.

Use `scripts/review-scope.sh` when shell access is available.
It performs deterministic git scope inspection and prints the suggested follow-up commands for dirty, branch, or base comparison review.

### `/review orchestrate`

Manage a broad review using focused passes when subagents or separate review roles are available.

1. Determine scope using `/review` scope rules.
2. Announce the chosen scope and review axes before launching focused passes.
3. Choose only relevant roles; do not run every role for tiny or specific scopes.
4. Relay each focused pass result back to the user because subagent runs may be opaque.
5. Synthesize findings by evidence, not majority vote.
6. Re-run only relevant focused passes after fixes.

Progress checkpoints for orchestrator agents:
- Scope selected.
- Focused passes selected or skipped with reason.
- Synthesis started.
- Fix plan ready or no findings found.

Orchestrator summary requirements:
- List which roles ran, what each inspected, and whether each found actionable issues.
- Aggregate duplicate findings into one canonical finding with supporting roles noted.
- Preserve disagreements or uncertainty instead of flattening them.
- Relay blocked permissions, missing tools, and suggested future permission rules.
- If fixes were applied, summarize what changed, which findings were addressed, and what verification or follow-up review ran.

Anti-stall rule:
- If a focused pass needs a blocked command, edit, network request, LSP query, or missing permission, it must return the blocked action and why it matters instead of waiting silently.
- Classify blocked actions before asking: one-off risky action, recurring safe friction, or unclear.
- If the same permission would likely be needed in future reviews and is recurring safe friction, apply the smallest skill, script, prompt, or permission improvement when the task scope authorizes dotfiles agent edits.
- If the task scope does not authorize agent-system edits, suggest the exact permission rule or instruction change to add.
- Prefer read-only deterministic helpers in `skills/user/review/scripts/` over ad hoc shell pipelines that cause permission churn.
- For Go tests blocked by go.work module exclusion errors, use the debugger helper or retry once with `GOWORK=off` when the command is otherwise the same and does not cross module boundaries.

### `/review fix`

Apply fixes from review findings only when the user clearly requested fixes or approved a proposed plan.

1. Fix the highest-severity real issue first.
2. Make the smallest correct change.
3. Do not broaden scope into rewrites unless the user approved that tradeoff.
4. Run the relevant formatter, test, build, or focused review when available.
5. Report what changed and what could not be verified.

Ask before:
- Broad rewrites.
- Behavior removal.
- Production-risky changes.
- Data migrations.
- Anything requiring product intent.

### `/review debugger`

Run the debugger role against the inferred or supplied scope.
The debugger owns `scripts/debugger.sh`.

Use when correctness is the main concern, or when a change touches state transitions, retries, concurrency, parsing, persistence, or error handling.

Look for broken assumptions, edge cases, races, error handling gaps, incorrect control flow, nil/empty cases, boundary conditions, and partial failure behavior.

Do not spend review budget on style unless it hides a bug.

### `/review auditor`

Run the auditor role against the inferred or supplied scope.
The auditor owns `scripts/auditor.sh`.

Use when changes touch credentials, shell commands, permissions, system config, network exposure, user data, deployment, rollback, or production blast radius.

Look for secrets, credential exposure, destructive operations, broad filesystem writes, privacy leaks, unsafe defaults, permission mistakes, and rollback hazards.

Most auditor reviews should be boring.
Do not invent risk without a plausible path to harm.

### `/review profiler`

Run the profiler role against the inferred or supplied scope.
The profiler owns `scripts/profiler.sh`.

Use when changes touch hot paths, loops, IO, queries, rendering, polling, caching, invalidation, startup, or runtime resource use.

Look for wasted work, bad asymptotics, N+1 queries, excessive IO, avoidable allocations, blocking work, over-broad invalidation, and costs shifted elsewhere.

Separate real bottlenecks from theoretical micro-optimizations.

### `/review janitor`

Run the janitor role against the inferred or supplied scope.
The janitor owns `scripts/janitor.sh`.

Use when changes add abstractions, spread behavior across files, duplicate logic, cross module seams, alter ownership, or feel patchwork.

Look for locality failures, duplication, coupling, low cohesion, unclear state ownership, leaky seams, vague helpers, and unnecessary indirection.

Prefer deletion, consolidation, and simpler ownership over new abstractions.
Do not request architecture purity unless it reduces actual future error.

### `/review architect`

Run the architect role against the inferred or supplied scope.
The architect owns `scripts/architect.sh`.

Focus on big-picture readability, system shape, module boundaries, conceptual names, abstraction level, and whether the design tells the truth.
Do not do line-level naming lint unless it reveals a structural clarity problem.

Use selectively.
Architect is for conceptual shape, not every review.

Look for hidden concepts, misleading abstractions, bad boundaries, missing vocabulary, unclear module responsibilities, and designs that make the wrong thing easy.

### `/review modernize`

Run the modernizer role against the inferred or supplied scope.
The modernizer owns `scripts/modernize.sh`.

Use when changes touch old APIs, dependencies, compatibility paths, migrations, fallbacks, language idioms, or version-specific behavior.

Look for deprecated APIs, legacy fallbacks, compatibility cruft without concrete need, weak migrations, obsolete idioms, and shortcuts that should become explicit invariants.

Do not recommend churn for novelty.
Modernization must reduce future error or remove obsolete complexity.

### `/review simplify`

Run the simplifier role against the inferred or supplied scope.
The simplifier owns `scripts/simplify.sh`.

Use when changes increase cognitive load, add large files, deepen nesting, spread simple behavior across too many places, or duplicate concepts.

Look for accidental complexity, huge files, deep branching, excessive indirection, duplicate logic, weak names, needless state, and code that can be made easier to read by deleting or collapsing structure.

Prefer fewer lines, fewer concepts, flatter control flow, and obvious data flow.
Do not trade explicitness for clever terseness.

### `/review scribe`

Run the scribe role against the inferred or supplied scope.
Load the `scribe` skill for documentation and comment rules.

Focus on stale comments, missing contracts, noisy prose, package/file docs, and whether documentation tells the truth.

## Skill Maintenance

Review agents should notice repeated friction and improve the review system when authorized.
If the task is about dotfiles skills, agents, prompts, scripts, or permissions, agents should edit the source-of-truth dotfiles directly when the path is in scope.
If review scope explicitly includes dotfiles skills or agents, agents may edit their owned script, relevant role prompt, and relevant review instructions to remove repeated friction.
During unrelated code reviews, report proposed self-improvements instead of editing skills unless the user authorizes it.

Look for areas of self-improvement, suggest ways to improve review script functionality under `skills/user/review/scripts/`, and raise script, skill, or permission improvements to the orchestrator or user when they would make future reviews easier.

Each focused role owns `scripts/<role>.sh` except scribe, which delegates automation to the `scribe` skill.
Scripts are executable helpers only.
Do not put agent instructions, role definitions, or review prose in scripts.

Agents manage their own script.
When a role repeatedly needs a deterministic check, the agent should propose a script change that would have helped.
If authorized, the agent may edit its own script, role prompt, and the relevant review skill instructions.
If the script change needs new permissions, include those permissions in the proposal.

Role scripts are stubs until a real command earns its place.
A useful script command should be deterministic, small, and easier to verify than model judgment.
It should be read-only by default, avoid likely secret paths, and prefer narrow git diffs or file-name scans over broad filesystem reads.

Suggested improvements should name:
- The skill or script to change.
- The observed friction.
- The smallest proposed instruction or script addition.
- The evidence needed before making it permanent.

## Reporting Format

When findings exist:
- Severity, file:line, issue.
- Why it matters.
- Smallest fix or verification.

When no findings exist:
- State that no actionable findings were found.
- Mention residual risks such as unrun tests, missing runtime context, or limited scope.

Keep summaries secondary to findings.
