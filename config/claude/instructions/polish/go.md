# Go Idioms and Patterns

## Tooling

Uses `gofmt` with modernize flag. Format on save is enabled - don't fight the formatter.

---

## Microservice Organization

### File Naming Prefixes

| Prefix      | Purpose                            | Example                 |
| ----------- | ---------------------------------- | ----------------------- |
| `init.*.go` | Service setup, migrations, seeding | `init.feedback.go`      |
| `s.*.go`    | [S]ervice handlers (API endpoints) | `s.feedback.config.go`  |
| `c.*.go`    | [C]ore logic (simple services)     | `c.stats.fetch.go`      |
| `r.*.yaml`  | Route registry (auto-generated)    | `r.feedback.yaml`       |
| `db.*.go`   | [D]ata[b]ase models (in models/)   | `db.feedback.go`        |
| `io.*.go`   | [I]nput/[o]utput DTOs (in models/) | `io.feedback.config.go` |

> **Choose one:** Use `c.*.go` for simple services OR `internal/` for complex services needing sub-packages. Not both.

### Complex Service Structure

```
services/partner/feedback/
├── init.feedback.go           # Service init, Migrate(), Seed()
├── s.feedback.config.go       # Config handlers (Create, List, Patch, Delete)
├── s.feedback.files.go        # File handlers
├── s.feedback.results.go      # Query handlers
├── s.feedback.sftp.go         # SFTP handlers
├── r.feedback.yaml            # Auto-generated routes
├── models/
│   ├── db.feedback.go         # Main entity
│   ├── db.feedback_config.go  # Config entity
│   ├── db.feedback_files.go   # File tracking entity
│   ├── io.feedback.config.go  # Config DTOs (matches s.feedback.config.go)
│   ├── io.feedback.files.go   # File DTOs
│   ├── io.feedback.sftp.go    # SFTP DTOs
│   └── config.*.go            # Enums, constants
└── internal/
    ├── ingest/                # Data processing pipeline
    │   ├── build/
    │   ├── match/
    │   ├── preprocess/
    │   └── transform/
    ├── query/                 # Query builders
    └── sftp/                  # External system integration
```

### Simple Service Structure

When complexity doesn't warrant `internal/`:

```
services/simple/
├── c.logic.go                # specific logic to keep handler lean
├── init.simple.go
├── r.simple.yaml
├── s.simple.go               # Single handler file for CRUD
├── models/
│   ├── db.entity.go
│   └── io.entity.go
```

### Key Rules

**models/ directory:**

- `db.*.go` - GORM models, database schemas
- `io.*.go` - Request/response DTOs, API params
- **Never import from other packages** - models are exporters only
- io._ files typically map 1:1 with s._.go handlers

**s.\*.go handlers:**

- Should be **lean and procedural**
- Easy to read, minimal logic
- Business logic belongs in `internal/` packages
- Group by feature domain, not HTTP method

**internal/ directory:**

- Only when complex sub-packages are needed
- Non-exported business logic
- Hierarchical: `internal/ingest/transform/` is valid

**init.\*.go:**

- Service struct definitions
- `SetConfig()`, `Migrate()`, `AutoMigrate()`, `Seed()`
- Register with `ms.Services.Register()`

---

## Error Handling

### Errors as Objects

```go
// Good - structured errors
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Usage
return ValidationError{Field: "email", Message: "invalid format"}
```

### Sentinel Errors for Common Cases

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrInvalidInput = errors.New("invalid input")
)
```

### Wrap Errors with Context (when appropriate)

```go
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

Caution: this can get excessive, make sure only at final return point, unique enough to fin code with search via error grep.

## Guard Clauses / Early Returns

### Happy Path Pattern

```go
func process(ctx context.Context, req Request) (Response, error) {
    if req.ID == "" {
        return Response{}, ErrMissingID
    }

    if !req.Valid() {
        return Response{}, ErrInvalidInput
    }

    user, err := s.repo.Get(ctx, req.ID)
    if err != nil {
        return Response{}, fmt.Errorf("get user: %w", err)
    }

    // Happy path - clean and linear
    result := transform(user)
    return Response{Data: result}, nil
}
```

## Naming

### Package Names

- Short, lowercase, single word
- Package name provides context for exported names

```go
package user
type Service struct{}  // user.Service
```

### Function Names

- Important functions: single word when package provides context
- Helpers: verb + noun, max 2 words (`parseInput`, `loadUser`)
- Getters don't use `Get` prefix

```go
user.Name()      // not user.GetName()
user.SetName(n)  // setter uses Set
```

### Acronyms

Go convention: acronyms fully capitalized in names.

```go
// Good
func ParseJSON(data []byte) error
func LoadCSV(path string) error
type HTTPClient struct{}
var userID string  // ID not Id

// At start of unexported name, lowercase
jsonParser := NewParser()
csvLoader := NewLoader()
```

## Interfaces

### Small Interfaces

```go
type Reader interface {
    Read(p []byte) (n int, err error)
}
```

### Accept Interfaces, Return Structs

```go
func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}
```

## Package Organization

Related code lives together (locality of behavior):

```
internal/
  billing/
    billing.go      # Core types and service
    invoice.go      # Invoice-related code
    payment.go      # Payment processing
    repository.go   # Data access for billing
```

**Never create LOCAL generic catch-all packages:**

- ❌ `utils/`, `helpers/`, `common/`, `shared/` (in service directories)
- ✅ Generic names ARE fine globally (`core/`)

**When you find utility-like code:**

1. Check if `shared/` or `core/` or `utility` or other similar packages already have it
2. Search for similar patterns elsewhere - enhance existing or hoist from another location
3. If truly reusable, hoist to global core packages
4. If local only, keep next to the code that uses it

## Performance

### Pre-allocate Slices When Size is Known

```go
users := make([]User, 0, len(ids))
for _, id := range ids {
    users = append(users, fetchUser(id))
}
```
