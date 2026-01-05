# React & TypeScript Review Patterns

Modern React (18+/19) and TypeScript patterns to check.

## Bugs

### Stale Closures
```tsx
// Bug: count is stale in the interval
useEffect(() => {
  const id = setInterval(() => {
    setCount(count + 1)  // always uses initial count
  }, 1000)
  return () => clearInterval(id)
}, [])  // missing count dependency

// Fix: use functional update
setCount(c => c + 1)
```

### Missing Cleanup
```tsx
// Bug: subscription continues after unmount
useEffect(() => {
  const sub = events.subscribe(handler)
  // missing: return () => sub.unsubscribe()
}, [])
```

### Incorrect Dependency Arrays
```tsx
// Bug: effect references `user` but doesn't list it
useEffect(() => {
  if (user.isAdmin) loadAdminData()
}, [])  // should include user or user.isAdmin
```

### State Updates After Unmount
```tsx
// Bug: setState called after component unmounts
useEffect(() => {
  fetchData().then(data => setData(data))  // may run after unmount
}, [])

// Fix: abort controller or mounted check
```

### Key Prop Issues
```tsx
// Bug: using index as key for dynamic lists
{items.map((item, i) => <Item key={i} {...item} />)}

// Fix: use stable identifier
{items.map(item => <Item key={item.id} {...item} />)}
```

## Anti-Patterns

### useEffect for Derived State
```tsx
// Anti-pattern: effect to compute derived value
const [fullName, setFullName] = useState('')
useEffect(() => {
  setFullName(`${first} ${last}`)
}, [first, last])

// Fix: compute directly
const fullName = `${first} ${last}`
```

### useEffect for Event Responses
```tsx
// Anti-pattern: effect to respond to action
useEffect(() => {
  if (submitted) {
    navigate('/success')
  }
}, [submitted])

// Fix: call in event handler
const submit = () => {
  doSubmit()
  navigate('/success')
}
```

### Prop Drilling for Global State
```tsx
// Anti-pattern: passing props through many layers
<App user={user}>
  <Layout user={user}>
    <Sidebar user={user}>
      <UserBadge user={user} />

// Fix: context for truly global state, or component composition
```

### Overusing useMemo/useCallback
```tsx
// Anti-pattern: memoizing cheap operations
const doubled = useMemo(() => count * 2, [count])

// Just compute it
const doubled = count * 2

// useMemo is for: expensive computations, referential equality for deps
```

### any Types
```tsx
// Anti-pattern
const process = (data: any) => data.value

// Fix: define the shape
interface DataItem { value: string }
const process = (data: DataItem) => data.value
```

### Excessive Defensive Checks
```tsx
// Anti-pattern: checking for impossible states
const getName = (user: User) => {
  if (!user) return ''           // User type doesn't include null
  if (!user.name) return ''      // name is required in User
  return user.name
}

// Fix: trust the types
const getName = (user: User) => user.name
```

## Modernization

### Class Components → Functions
Class components work but function components with hooks are preferred for new code.

### Legacy Context → Context API
Old `contextTypes` and `childContextTypes` should use `createContext`.

### componentWillMount etc → useEffect
Legacy lifecycle methods have modern equivalents.

### String Refs → useRef
```tsx
// Old
<input ref="myInput" />

// Modern
const inputRef = useRef<HTMLInputElement>(null)
<input ref={inputRef} />
```

### defaultProps → Default Parameters
```tsx
// Old
Component.defaultProps = { size: 'medium' }

// Modern
const Component = ({ size = 'medium' }: Props) => ...
```

## Performance

### Unnecessary Re-renders
```tsx
// Problem: new object every render
<Child style={{ color: 'red' }} />

// Fix: stable reference or CSS class
const style = useMemo(() => ({ color: 'red' }), [])
```

### Large Component Trees in Context
Components consuming context re-render on any context change.
Split contexts by update frequency.

### Missing React.memo for Expensive Children
Pure components with expensive render should be memoized when parent re-renders often.

### Inline Functions in Hot Loops
```tsx
// Problem: new function per item per render
{items.map(item => (
  <button onClick={() => handleClick(item.id)}>

// Fix if this is actually causing perf issues:
// useCallback or move handler definition
```

## TypeScript-Specific

### Non-null Assertion Overuse
```tsx
// Risky: suppresses type checking
user!.name

// Better: handle the null case or fix upstream
user?.name ?? 'Unknown'
```

### Type Assertions vs Type Guards
```tsx
// Risky: trust-me assertion
const user = data as User

// Safer: runtime check
function isUser(data: unknown): data is User {
  return typeof data === 'object' && data !== null && 'id' in data
}
```

### Implicit any
Watch for implicit `any` in callbacks, catch blocks, and untyped dependencies.

## React 19 Specific

### use() Hook
New `use()` hook for promises and context - simpler than Suspense wrappers in some cases.

### Actions and useFormStatus
Form handling improvements - `useFormStatus`, `useFormState` for form state.

### useOptimistic
For optimistic UI updates during async operations.

### ref as Prop
No longer need `forwardRef` - ref can be passed as regular prop.
