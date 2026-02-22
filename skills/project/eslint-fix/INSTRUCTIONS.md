# eslint-fix

Fix ESLint and TypeScript errors in **leadpierui/** only. All commands must run from the `leadpierui/` project root. Ignore any non-TS/TSX errors (e.g., Go files, other repos).

## Subcommands

### Default (no args) — Fix staged ESLint errors

1. **Get staged files:**

```sh
git diff --cached --name-only --diff-filter=ACMR -- '*.ts' '*.tsx'
```

If no staged TS/TSX files, nothing to do.

2. **Run ESLint** on those files only:

```sh
yarn eslint <files> 2>&1
```

Do NOT run `yarn lint` or lint the whole repo — only the staged files.

3. **Fix each error** by reading the file and editing the code. Read surrounding code to understand intent before fixing.

4. **Re-run ESLint** on the fixed files to verify — repeat until clean.

### `push` — Push with automatic error fixing

1. **Attempt push:**

```sh
git push 2>&1
```

Never force push. If push succeeds, done.

2. **On failure**, parse the error output. The pre-push hook runs `tsc -b --noEmit` (type check) and on `dev` branch also `yarn test`. Only process TypeScript errors (`.ts`/`.tsx` files) — **ignore** Go errors, go.mod warnings, or any output from other repos. TS errors follow this format:

```
path/to/file.tsx:LINE:COL - error TSXXXX: message
```

3. **Fix each error** by reading the file and editing the code. Group fixes by file. Common patterns:
   - `TS2307` (module not found) — fix import path
   - `TS2322` (type mismatch) — fix prop types or type annotations
   - `TS2339` (property doesn't exist) — add to type/interface or fix access
   - `TS2367` (unintentional comparison) — add value to enum/union type
   - `TS7006` (implicit any) — add type annotation
   - `TS7031` (binding element implicit any) — add type to destructured param

4. **Run type check** to verify fixes:

```sh
yarn lint:types 2>&1
```

Repeat fix cycle until clean.

5. **Commit fixes and retry push** — stage changed files, commit with a message like "fix: resolve type errors", then `git push` again.

## Notes

- Only fix errors in relevant files — don't touch unrelated code.
- Do NOT use `@ts-ignore`, `eslint-disable`, or other suppressions unless the user explicitly asks.
- Prefer minimal, targeted fixes — don't refactor surrounding code.
- If a fix is ambiguous, ask the user.
- Never use `--no-verify` to skip hooks.
