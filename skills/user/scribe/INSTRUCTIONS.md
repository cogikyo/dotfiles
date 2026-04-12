# Scribe

Comment and documentation management. Read `references/style-guide.md` before making any changes.

## Commands

### `/scribe review`

Audit comments across a scope (file, directory, or repo).

1. Read the scope (user explains what to review, or defaults to staged+modified files)
2. Read `references/style-guide.md` for conventions
3. Check each file for:
   - **Drift**: comments that no longer match the code
   - **Redundancy**: duplicated information across comments
   - **Navigation**: do section headers and doc comments tell a coherent story?
   - **Conciseness**: verbose comments, unnecessary words
   - **Style violations**: formatting, voice, punctuation per the style guide
4. Present findings as a plan with suggested fixes
5. Ask for confirmation before applying changes
6. Leave TODO/FIXME markers (with user confirmation) where implementation is unclear

### `/scribe update`

Add or update comments for specific files. User provides paths or context about what changed.

1. Read the target files and `references/style-guide.md`
2. Apply the style guide directly — no confirmation needed
3. Determine comment tier per function/block (see style guide: thorough vs intentional vs minimal)
4. Add/update doc comments, inline comments, and section headers as needed
5. If better naming or file organization would reduce comment needs, suggest it

### `/scribe question`

Answer a question about code using comments + source as context.

1. Read relevant code and comments to answer the question
2. Provide a clear, concise answer
3. **Leave it cleaner than you found it**: if comments were unclear or insufficient for answering, update them so the next person doesn't need to ask
4. Minor comment fixes don't need confirmation; significant additions do

## Principles

- **Architecture over comments**: if better naming, organization, or structure eliminates the need for a comment, prefer that
- **Token-efficient**: every comment should justify its existence. Brief, direct, no filler
- **Never wrap mid-sentence**: a comment line is one complete thought. Soft limit ~120 chars; if longer, the comment is too verbose
- **Markers are tools**: TODO, FIXME, HACK are grep-able breadcrumbs. Use them when scribe finds issues that need user input. NOTE is also valid for non-actionable context
- **Language-aware**: apply the correct doc convention per language (see `references/style-guide.md`)
