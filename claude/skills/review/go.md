# Go Review Patterns

Modern Go (1.22+) patterns to check.

## Bugs

### Nil Pointer Dereference
```go
// Bug: no nil check before use
user, _ := repo.Find(id)
return user.Name  // panics if user is nil

// Fix: check or return error
user, err := repo.Find(id)
if err != nil {
    return "", err
}
return user.Name, nil
```

### Goroutine Leaks
```go
// Bug: channel never read, goroutine blocks forever
go func() {
    result <- compute()  // blocks if nobody reads
}()

// Fix: buffered channel or select with context
```

### Loop Variable Capture (pre-1.22)
```go
// Bug in Go <1.22: all goroutines see last value
for _, item := range items {
    go func() {
        process(item)  // uses last item only
    }()
}

// Note: Fixed in Go 1.22+ with loop variable semantics
// But still a bug if targeting older versions
```

### Shadowed Error
```go
// Bug: err in inner scope, outer err unchecked
err := doFirst()
if condition {
    result, err := doSecond()  // shadows outer err
    use(result)
}
return err  // returns doFirst error, not doSecond

// Fix: use = not := for inner assignment, or handle both
```

### Range Over Nil Slice/Map
```go
// Safe: ranging over nil is fine (zero iterations)
for _, v := range nilSlice {}  // ok

// Bug: indexing nil slice
nilSlice[0]  // panic
```

### Unclosed Resources
```go
// Bug: file never closed on error path
file, err := os.Open(path)
if err != nil {
    return err
}
data, err := io.ReadAll(file)
if err != nil {
    return err  // file leak
}
file.Close()

// Fix: defer close immediately
file, err := os.Open(path)
if err != nil {
    return err
}
defer file.Close()
```

### Data Race
```go
// Bug: concurrent map write
go func() { m["key"] = value }()
go func() { m["key"] = other }()

// Fix: sync.Mutex, sync.Map, or channel
```

## Anti-Patterns

### Naked Returns in Complex Functions
```go
// Anti-pattern: hard to track what's returned
func process() (result Result, err error) {
    // ... 50 lines ...
    return  // what values?
}

// Fix: explicit returns in long functions
return result, nil
```

### Error String Checking
```go
// Anti-pattern: brittle string matching
if err.Error() == "not found" {

// Fix: error types or sentinel errors
if errors.Is(err, ErrNotFound) {
```

### Returning Concrete Types for Interfaces
```go
// Anti-pattern: return type more specific than needed
func NewReader() *BufferedReader {

// Better: return interface if callers don't need concrete type
func NewReader() io.Reader {
```

### Over-Engineering Error Wrapping
```go
// Anti-pattern: wrapping at every level
return fmt.Errorf("processUser: %w",
    fmt.Errorf("validateInput: %w",
        fmt.Errorf("checkEmail: %w", err)))

// Fix: wrap once at meaningful boundary
return fmt.Errorf("process user %s: %w", id, err)
```

### Using panic for Control Flow
```go
// Anti-pattern: panic for expected conditions
if user == nil {
    panic("user required")
}

// Fix: return error
if user == nil {
    return ErrUserRequired
}
```

### Empty Interface Abuse
```go
// Anti-pattern: losing type safety
func Process(data interface{}) interface{} {

// Fix: use generics or specific types
func Process[T any](data T) T {
```

## Modernization (Go 1.21+)

### Use slices Package
```go
// Old
sort.Slice(items, func(i, j int) bool {
    return items[i].Name < items[j].Name
})

// New (1.21+)
slices.SortFunc(items, func(a, b Item) int {
    return cmp.Compare(a.Name, b.Name)
})
```

### Use maps Package
```go
// Old
keys := make([]string, 0, len(m))
for k := range m {
    keys = append(keys, k)
}

// New (1.21+)
keys := maps.Keys(m)
```

### Use log/slog for Structured Logging
```go
// Old
log.Printf("user %s created", userID)

// New (1.21+)
slog.Info("user created", "userID", userID)
```

### Use errors.Join for Multiple Errors
```go
// Old
var errs []error
// ... collect errors ...
return fmt.Errorf("multiple errors: %v", errs)

// New (1.20+)
return errors.Join(errs...)
```

### Use context.WithoutCancel (1.21+)
When you need context values but not cancellation.

### Use cmp Package (1.21+)
```go
// Old
if a < b {
    return -1
} else if a > b {
    return 1
}
return 0

// New
return cmp.Compare(a, b)
```

### Range Over Int (1.22+)
```go
// Old
for i := 0; i < n; i++ {

// New (1.22+)
for i := range n {
```

### Range Over Func (1.23+)
Iterator functions for custom iteration.

## Performance

### String Concatenation in Loops
```go
// Problem: O(n^2) allocations
var s string
for _, item := range items {
    s += item.Name
}

// Fix: use strings.Builder
var b strings.Builder
for _, item := range items {
    b.WriteString(item.Name)
}
```

### Unpreallocated Slices
```go
// Problem: multiple allocations during growth
var result []Item
for _, id := range ids {
    result = append(result, fetch(id))
}

// Fix: preallocate when size is known
result := make([]Item, 0, len(ids))
```

### Unnecessary Allocations in Hot Paths
```go
// Problem: allocates every call
func format(id int) string {
    return fmt.Sprintf("user-%d", id)
}

// Fix if called frequently: sync.Pool or buffer reuse
```

### Value vs Pointer Receivers
Large structs (>64 bytes roughly) should use pointer receivers.
Small structs can use value receivers.

### defer in Tight Loops
```go
// Problem: defer overhead in hot loop
for _, file := range files {
    f, _ := os.Open(file)
    defer f.Close()  // defers accumulate until function returns
}

// Fix: extract to function or manual close
```

## Security

### SQL Injection
```go
// Bug: string interpolation in query
db.Query("SELECT * FROM users WHERE id = " + id)

// Fix: parameterized query
db.Query("SELECT * FROM users WHERE id = ?", id)
```

### Path Traversal
```go
// Bug: user input in file path
path := filepath.Join(baseDir, userInput)  // ../../../etc/passwd

// Fix: validate and clean path
path := filepath.Join(baseDir, filepath.Base(userInput))
```

### Timing Attacks
```go
// Bug: early return reveals info
if password != expected {
    return false
}

// Fix: constant-time comparison for secrets
subtle.ConstantTimeCompare([]byte(password), []byte(expected))
```

### Hardcoded Secrets
```go
// Bug
const apiKey = "sk-12345..."

// Fix: environment variable or secret manager
apiKey := os.Getenv("API_KEY")
```
