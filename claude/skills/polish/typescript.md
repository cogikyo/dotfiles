# TypeScript & React Patterns

## Directory Structure

### Directory Naming

- **One word preferred**: `documents/`, `users/`, `billing/`, `hooks/`
- **Compound words sparingly**: for specific nouns (`FormFields/`)

**Casing by content:**

- **PascalCase** if contains `.tsx` (components): `DocumentList/`, `UserProfile/`
- **camelCase** if only `.ts` (utils/logic): `formatting/`, `validation/`, `hooks/`

Avoid multi-word with separators: not `lead-details/` or `lead_details/`

### Directory Size (Polish Phase)

During development, verbose structure is fine. When polishing, compress to final form:

- **Minimum ~2 files**: Single-file directories usually belong in parent
- **Maximum ~6 files**: More suggests the directory is doing too much - split by concern
- **Exceptions**: `components/` roots, `hooks/` collections, barrel directories

These are soft guidelines - use judgment based on cohesion and clarity.

### File Naming Patterns

| Type        | Pattern                                | Example               |
| ----------- | -------------------------------------- | --------------------- |
| Components  | `PascalCase.tsx`                       | `DocumentList.tsx`    |
| Hooks       | `use{Name}.ts`                         | `useDocument.ts`      |
| Utilities   | `camelCase.ts`                         | `formatDate.ts`       |
| Types       | `{name}.types.ts`                      | `document.types.ts`   |
| Constants   | `{name}.constants.ts`                  | `routes.constants.ts` |
| API/queries | `{name}.api.ts` or `{name}.queries.ts` | `documents.api.ts`    |

### Feature Directory Structure

```
features/
  documents/                 # One word
    Document.tsx             # Main component
    DocumentList.tsx         # Related component
    useDocument.ts           # Feature hook
    documents.api.ts         # API calls
    documents.types.ts       # Types (if many)
```

When a feature grows beyond ~6 files, split by concern:

```
features/
  documents/
    components/              # UI components
      Document.tsx
      DocumentList.tsx
      DocumentCard.tsx
    hooks/                   # Feature hooks
      useDocument.ts
      useDocumentFilters.ts
    documents.api.ts
    documents.types.ts
```

---

## Legacy Codebase Considerations

The frontend has evolved over time with multiple patterns. When unclear which pattern is correct:

1. **Ask the user** for guidance
2. **Suggest the best modern approach** (possibly creating a new pattern to replace old abstractions)

## Event Handlers

### Avoid Generic `handle{}` Naming

```tsx
// Bad - generic naming
const handleClick = () => { ... }
const handleChange = () => { ... }

<button onClick={handleClick}>Save</button>
```

```tsx
// Good - descriptive of what it does
const saveDoc = () => { ... }
const updateQuery = (query: string) => { ... }

<button onClick={saveDoc}>Save</button>
<input onChange={(e) => updateQuery(e.target.value)} />
```

The handler name should describe the action, not the event.

## useEffect Guidelines

**Avoid useEffect unless critical.** Most use cases have better alternatives:

### Instead of useEffect for...

**Derived state:**

```tsx
// Bad
const [fullName, setFullName] = useState("");
useEffect(() => {
  setFullName(`${firstName} ${lastName}`);
}, [firstName, lastName]);

// Good - derive directly
const fullName = `${firstName} ${lastName}`;
```

**Transforming data:**

```tsx
// Bad
useEffect(() => {
  setFilteredItems(items.filter((i) => i.active));
}, [items]);

// Good - useMemo or derive
const filteredItems = useMemo(() => items.filter((i) => i.active), [items]);
```

**Responding to events:**

```tsx
// Bad
useEffect(() => {
  if (submitted) {
    sendAnalytics();
  }
}, [submitted]);

// Good - call in the event handler
const submit = () => {
  doSubmit();
  sendAnalytics();
};
```

### When useEffect IS appropriate

- Synchronizing with external systems (subscriptions, DOM APIs)
- Fetching data on mount (though prefer React Query/SWR)
- Setting up/cleaning up timers or listeners

## Type Safety

### Eliminate `any`

```tsx
// Bad
const processData = (data: any) => { ... }

// Good - define proper types
interface UserData {
    id: string
    name: string
}
const processData = (data: UserData) => { ... }
```

### Narrow Types Over Defensive Checks

```tsx
// Bad - overly cautious
const getName = (user: User | null | undefined) => {
  if (!user) return "";
  if (!user.name) return "";
  return user.name;
};

// Good - ensure type safety upstream
const getName = (user: User) => user.name;

// Or use proper narrowing
const getName = (user: User | null) => user?.name ?? "";
```

## Component Organization

### Co-locate Related Code

```
// Good structure for a feature
features/
  documents/
    Document.tsx        # Main component
    DocumentList.tsx    # List component
    useDocument.ts      # Hook for document logic
    document.types.ts   # Types for this feature
    document.utils.ts   # Helpers specific to documents
```

### Split Large Components

Signs a component needs splitting:

- Multiple distinct responsibilities
- Large amount of local state for different concerns
- Reusable UI sections

## Imports & Exports

### Prefer Named Exports

```tsx
// Good - explicit, searchable
export const DocumentList = () => { ... }
export const useDocument = () => { ... }

// Avoid default exports when possible
```

### Clean Up Barrel Files

If `index.ts` re-exports become unwieldy, consider direct imports to the source file.

## State Management

### Lift State Appropriately

- State used by one component: keep local
- State used by siblings: lift to parent
- State used across features: context or global store

### Context Refactoring

Signs context needs reorganization:

- Context providing unrelated data
- Many consumers only need part of the context
- Frequent re-renders from context changes

Split into focused contexts based on update frequency and consumer needs.
