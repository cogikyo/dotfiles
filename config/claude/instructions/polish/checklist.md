# Polish Checklist

## Quick Mode Checklist

For `/polish quick` - lightweight pass on small scopes:

- [ ] Read all files in scope
- [ ] **Naming**: variables, functions follow conventions
- [ ] **Locality**: related code grouped together
- [ ] **Idioms**: language-specific patterns applied
- [ ] **Interfaces**: consistent signatures between files
- [ ] Present changes to user
- [ ] Execute after approval
- [ ] Run ESLint fix twice on modified TS files (`npx eslint --fix`)
- [ ] Brief summary

---

## Full Mode Checklist

## Phase 1: Initial Planning

- [ ] Entered plan mode (`EnterPlanMode`)
- [ ] Identified scope (files/modules to polish)
- [ ] Each proposed change includes:
  - [ ] File references with what to read/modify
  - [ ] Dependencies on other changes
  - [ ] Whether it can parallelize
  - [ ] Exploration questions needed
- [ ] Defined execution order
- [ ] Identified parallel opportunities
- [ ] **Got user confirmation**

## Phase 2: Exploration

- [ ] Spawned exploration agents in parallel
- [ ] Checked shared utilities (pkg/, lib/, utils/)
- [ ] Found all usages of code being moved
- [ ] Identified imports that need updating
- [ ] Looked for similar patterns to consolidate
- [ ] Checked for dead code paths
- [ ] Synthesized findings into Plan v2
- [ ] **Got user confirmation on Plan v2**

## Phase 3: Execution

### Order of Operations
- [ ] 1. Directory creation (parallel)
- [ ] 2. File moves/renames (parallel where independent)
- [ ] 3. Content edits - Batch A (parallel)
- [ ] 4. Content edits - Batch B (parallel after A)
- [ ] 5. Import/reference updates (parallel)
- [ ] 6. Shared utility updates (sequential)

### During Execution
- [ ] Tracked all changes per agent
- [ ] Noted deviations from plan
- [ ] Collected issues for review
- [ ] Ignored transient LSP errors

### Tooling (after edits)
- [ ] TypeScript: Ran `npx eslint --fix` twice on modified files
- [ ] TypeScript: Verified no remaining ESLint errors
- [ ] Go: Ran `gofmt -w -s` on modified files

## Phase 4: Review

- [ ] Spawned fresh review agent
- [ ] Verified no new TS/Go errors
- [ ] Verified no broken imports
- [ ] Verified functionality preserved
- [ ] Verified naming conventions followed
- [ ] Verified code passes linters

### Review Results
- [ ] **PASS**: Completed with detailed summary
- [ ] **ISSUES FOUND**: Reported, proposed fixes, executed, re-reviewed
- [ ] **NEEDS NEW PLAN**: Reported side effects, proposed new plan

## Final Verification

### Functionality
- [ ] No behavior changes (unless dead code removed)
- [ ] All tests still pass
- [ ] Edge cases still handled

### Structure
- [ ] Files split appropriately
- [ ] Related code co-located
- [ ] Consistent organization frontend/backend
- [ ] No single-file directories (move to parent)
- [ ] No oversized directories (>6 files suggests split needed)
- [ ] Directory names are one word or compound noun
- [ ] Directory casing matches content (PascalCase if .tsx, camelCase if .ts only)
- [ ] No LOCAL generic catch-all names (utils/, helpers/, interfaces/ in features)
- [ ] Checked global shared for existing utilities before creating new ones
- [ ] Searched for similar patterns elsewhere (enhance existing or hoist)

### Naming
- [ ] Important functions have single-word names
- [ ] Helpers use verb + noun pattern (max 2 words)
- [ ] Acronyms capitalized (except at word start: `csvParser` but `parseCSV`)
- [ ] Consistency between frontend/backend
- [ ] No 3+ word names (sign of missing context)

### Code Quality
- [ ] No duplicate implementations
- [ ] Shared utilities used/updated
- [ ] Guard clauses for early returns
- [ ] Errors as objects
- [ ] Type safety improved

### TypeScript Specific
- [ ] No unnecessary useEffect
- [ ] Event handlers named by action
- [ ] No `any` types
- [ ] Proper type narrowing
- [ ] File naming follows pattern (PascalCase components, use*.ts hooks, *.types.ts, *.api.ts)
- [ ] ESLint `--fix` run twice on all modified files
- [ ] No remaining ESLint errors

### Go Specific
- [ ] Follows gofmt/modernize
- [ ] Small focused interfaces
- [ ] Package names provide context
- [ ] Errors wrapped with context

### Go Microservice Organization
- [ ] File prefixes correct (init.*, s.*, db.*, io.*, r.*)
- [ ] s.* handlers lean/procedural (logic in internal/)
- [ ] internal/ used for complex business logic
- [ ] c.*.go only for simple cases (legacy pattern - prefer internal/)
- [ ] models/ contains only db.* and io.* files
- [ ] models/ files don't import from other packages (exporters only)
- [ ] io.* files map 1:1 with s.* handlers
- [ ] init.*.go has Migrate(), SetConfig(), Register()

### Performance
- [ ] No re-render issues (frontend)
- [ ] No poor time/space complexity
- [ ] Pre-allocated slices where size known

## Output Delivered

- [ ] Execution summary (files created/modified/moved)
- [ ] Patterns applied with reasoning
- [ ] Shared utilities updated
- [ ] Review results
- [ ] Follow-up suggestions
