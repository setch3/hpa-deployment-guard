# Project Structure

## Directory Layout

```
├── cmd/webhook/           # Application entry point
│   └── main.go           # Main server executable
├── internal/             # Private application code
│   ├── cert/            # TLS certificate management
│   │   ├── manager.go   # Certificate loading/validation
│   │   └── *_test.go    # Unit and integration tests
│   ├── validator/       # Core validation logic
│   │   ├── validator.go # HPA/Deployment validation
│   │   ├── types.go     # Validation types and constants
│   │   └── *_test.go    # Unit tests
│   └── webhook/         # HTTP server and admission logic
│       ├── server.go    # Webhook server implementation
│       └── *_test.go    # Server tests
├── manifests/           # Kubernetes deployment manifests
├── scripts/             # Utility scripts
└── .kiro/              # Kiro configuration and specs
```

## Code Organization Patterns

### Package Structure
- **cmd/**: Contains main applications (single responsibility)
- **internal/**: Private packages not intended for external use
- Each package has a clear, single responsibility

### File Naming Conventions
- `*_test.go` - Unit tests alongside source files
- `integration_test.go` - Integration tests with build tags
- `types.go` - Type definitions and constants
- `manager.go` - Main implementation for management logic

### Testing Strategy
- Unit tests in same package as source code
- Integration tests with build tags (`// +build integration`)
- Test files follow `*_test.go` naming convention

### Error Handling
- Custom error constants defined in `types.go`
- Structured error messages with context
- Graceful degradation for non-critical failures

### Logging
- Structured logging with context
- Japanese language support for certificate operations
- Different log levels for development vs production