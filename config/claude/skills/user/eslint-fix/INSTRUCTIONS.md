# eslint-fix

Fix ESLint errors that `eslint --fix` can't auto-resolve.

## Workflow

1. **Get staged files** — only lint what's being committed:

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

## Notes

- Only fix errors in staged files — don't touch unstaged files.
- Do NOT use `@ts-ignore`, `eslint-disable`, or other suppressions unless the user explicitly asks.
- Prefer minimal, targeted fixes — don't refactor surrounding code.
- If a fix is ambiguous, ask the user.
