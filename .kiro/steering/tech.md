# Technology Stack

## Language & Runtime
- **Go 1.24.2** - Primary language
- Standard library with minimal external dependencies

## Kubernetes Integration
- **k8s.io/client-go v0.28.0** - Kubernetes API client
- **k8s.io/api v0.28.0** - Kubernetes API types
- **k8s.io/apimachinery v0.28.0** - Kubernetes API machinery

## Build System
- **Go modules** (`go.mod`) for dependency management
- No additional build tools required

## Common Commands

### Development
```bash
# Run tests
go test ./...

# Run integration tests
go test ./internal/cert -tags=integration

# Build binary
go build -o webhook ./cmd/webhook

# Run locally (requires certs)
./webhook -port=8443 -cert-file=./certs/tls.crt -key-file=./certs/tls.key
```

### Certificate Management
```bash
# Generate TLS certificates
./scripts/generate-certs.sh

# Verify webhook connectivity
./scripts/verify-webhook.sh

# Verify RBAC permissions
./scripts/verify-rbac.sh
```

### Deployment
```bash
# Apply manifests
kubectl apply -f manifests/

# Check webhook logs
kubectl logs -l app=k8s-deployment-hpa-validator
```

## Security
- TLS 1.2+ with strong cipher suites
- Certificate validation and chain verification
- Graceful certificate reloading support