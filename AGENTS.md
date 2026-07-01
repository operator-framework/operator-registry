# AGENTS.md

This document provides guidance for AI agents working with the Operator Registry project.

## Project Overview

The Operator Registry is a Kubernetes/OpenShift component that provides operator catalog data to the Operator Lifecycle Manager (OLM). It manages operator bundles, indexes, and catalogs that enable operators to be discovered and installed in Kubernetes clusters.

## Key Components

### Binaries
- **`opm`**: Main CLI tool for rendering and serving file-based catalogs

### Libraries
- **`pkg/client`**: High-level client interface for gRPC API
- **`pkg/api`**: Low-level client libraries for gRPC interface
- **`pkg/registry`**: Core registry types (Packages, Channels, Bundles)
- **`pkg/lib`**: External interfaces and standards for operator bundles
- **`pkg/containertools`**: Container tooling integration
- **`pkg/cache`**: Caching backends for file-based catalogs

### Alpha Features
- **`alpha/declcfg`**: Declarative configuration format
- **`alpha/template`**: Template system for generating catalogs
- **`alpha/action`**: Action framework for registry operations

## Development Guidelines

### Code Structure
- **Go 1.24.4**: Minimum Go version required
- **Cobra CLI**: Command-line interface framework
- **gRPC**: Primary API communication protocol
- **Declarative Config (FBC)**: File-based catalog format for registry data
- **OCI Images**: Container image format for bundles

### Testing
- Unit tests: `go test ./...`
- Integration tests: Located in `test/e2e/`
- Linting: `make lint` (uses golangci-lint)
- Coverage: `make coverage`

### Build System
- **Makefile**: Primary build configuration
- **GoReleaser**: Release automation
- **Docker**: Container image builds

## Common Tasks for AI Agents

### 1. Adding New Template Types
When adding new template types to `alpha/template/`:

```go
// 1. Implement the Template interface
type MyTemplate struct {
    renderBundle template.BundleRenderer
}

func (t *MyTemplate) RenderBundle(ctx context.Context, image string) (*declcfg.DeclarativeConfig, error) {
    return t.renderBundle(ctx, image)
}

func (t *MyTemplate) Render(ctx context.Context, reader io.Reader) (*declcfg.DeclarativeConfig, error) {
    // Implementation
}

func (t *MyTemplate) Schema() string {
    return "olm.template.mytype"
}

// 2. Implement the TemplateFactory interface
type Factory struct{}

func (f *Factory) CreateTemplate(renderBundle template.BundleRenderer) template.Template {
    return &MyTemplate{renderBundle: renderBundle}
}

func (f *Factory) Schema() string {
    return "olm.template.mytype"
}

// 3. Register in cmd/opm/alpha/template/render.go
registry.Register(&mytype.Factory{})
```

### 2. Working with Bundle Images
```bash
# Build a bundle image
podman build -t quay.io/my-namespace/my-bundle:latest -f bundle.Dockerfile .

# IMPORTANT: Bundle images must be published to a registry before they can be consumed
podman push quay.io/my-namespace/my-bundle:latest

# Render bundle to declarative config
opm render quay.io/my-namespace/my-bundle:latest --output yaml > catalog.yaml

# Generate catalog from multiple bundles
opm render quay.io/my-namespace/my-bundle:v1.0.0 quay.io/my-namespace/my-bundle:v1.1.0 --output yaml
```

**⚠️ Critical Requirements:**
- **Bundle images must be published** to an image registry before they can be referenced in catalogs
- **FBC content requires published images** - `opm render` and templates can only reference bundle images that exist in registries
- **Catalog images must be built and published** before they can be consumed by OLM

### 3. Working with Declarative Config
```go
// Load declarative config
cfg, err := declcfg.LoadFS(os.DirFS("path/to/catalog"))

// Convert to model
model, err := declcfg.ConvertToModel(cfg)

// Write declarative config
err = declcfg.Write(cfg, "output.yaml")
```

### 4. Serving Catalog Content with `opm serve`
The `opm serve` command exposes operator catalog data via a gRPC interface for consumption by OLM.

```bash
# Serve declarative configs with default settings
opm serve ./catalog-directory

# Serve with custom port and caching
opm serve ./catalog-directory --port 8080 --cache-dir /tmp/cache

# Serve with cache integrity enforcement
opm serve ./catalog-directory --cache-dir /tmp/cache --cache-enforce-integrity

# Cache-only mode (build cache without serving)
opm serve ./catalog-directory --cache-dir /tmp/cache --cache-only
```

**⚠️ FBC Requirements:**
- **Bundle images must be published** to image registries before they can be referenced in FBC
- **Image references must be valid** - `opm serve` will fail if bundle images don't exist
- **Catalog content is loaded at startup** - changes to FBC files after startup won't be reflected
- **Use `--cache-enforce-integrity`** to ensure cache validity before serving

#### gRPC Interface
The server exposes the following gRPC services:

**Registry Service Methods:**
- `ListPackages()` - Stream all package names
- `GetPackage(name)` - Get package details including channels
- `GetBundle(pkgName, channelName, csvName)` - Get specific bundle
- `GetBundleForChannel(pkgName, channelName)` - Get latest bundle in channel (deprecated)
- `GetChannelEntriesThatReplace(csvName)` - Stream channel entries that replace a bundle
- `GetBundleThatReplaces(csvName, pkgName, channelName)` - Get bundle that replaces another
- `GetChannelEntriesThatProvide(group, version, kind)` - Stream entries providing an API
- `GetLatestChannelEntriesThatProvide(group, version, kind)` - Stream latest entries providing an API
- `GetDefaultBundleThatProvides(group, version, kind)` - Get default bundle providing an API
- `ListBundles()` - Stream all bundles

**Health Service:**
- Standard gRPC health check service for monitoring

#### Cache Types and Backends

**Available Cache Backends:**

1. **PogrebV1 Backend** (Default)
   - Format: `pogreb.v1`
   - Uses embedded key-value database
   - Optimized for read performance
   - Stores bundles as protobuf-encoded data
   - File permissions: 0770 (dirs), 0660 (files)

2. **JSON Backend**
   - Format: `json`
   - Human-readable JSON files
   - Easier debugging and inspection
   - Stores bundles as JSON files
   - File permissions: 0750 (dirs), 0640 (files)

**Cache Configuration:**
```go
// Cache options
type CacheOptions struct {
    Log    *logrus.Entry
    Format string  // "pogreb.v1" or "json"
}

// Create cache with specific backend
cache, err := cache.New("/cache/dir", 
    cache.WithLog(logger),
    cache.WithFormat("pogreb.v1"))
```

**Cache Integrity:**
- Automatic digest computation for cache validation
- Integrity checks compare source content with cached data
- Cache rebuilds when integrity check fails
- `--cache-enforce-integrity` flag ensures cache is valid before serving

**Performance Considerations:**
- PogrebV1: Better for production (faster reads, smaller footprint)
- JSON: Better for development (human-readable, easier debugging)
- Cache directory should be persistent for production deployments
- Use `--cache-only` for pre-building cache in CI/CD pipelines

## Removed SQLite Support

⚠️ **NOTICE**: SQLite-based catalogs and their related commands (`opm registry`, `opm index`) have been removed from this project. All catalog workflows now use the file-based catalog (FBC) format exclusively.

**Migration**: For older versions that supported SQLite-to-FBC migration, refer to previous releases or documentation versions.

### Creating File-Based Catalogs

#### Using `opm render`
The `opm render` command converts various sources to FBC format:

```bash
# Render from bundle images to FBC
opm render quay.io/my/bundle:v1.0.0 quay.io/my/bundle:v1.1.0 --output json

# Render with image reference template for missing bundle references
opm render ./bundle-directory --alpha-image-ref-template "quay.io/my-registry/{{.Package}}:{{.Version}}" --output yaml
```

#### Using Templates with `opm alpha render-template`
Templates can generate FBC content from structured input files:

```bash
# Generate FBC from basic template
opm alpha render-template basic ./my-template.yaml --output yaml

# Generate FBC from semver template
opm alpha render-template semver ./semver-template.yaml --output json

# Auto-detect template type from schema field
opm alpha render-template ./template-file.yaml --output yaml
```

## File Organization

### Key Directories
- **`cmd/`**: CLI applications and main functions
- **`pkg/`**: Reusable Go packages
- **`alpha/`**: Experimental features (may change)
- **`test/`**: Test files and test data
- **`docs/`**: Documentation
- **`manifests/`**: Example operator manifests
- **`bundles/`**: Test bundle data

### Important Files
- **`go.mod`**: Go module dependencies
- **`Makefile`**: Build and test commands
- **`README.md`**: Project overview and usage
- **`OWNERS`**: Code ownership and review requirements

## Common Patterns

### Error Handling
```go
if err != nil {
    return fmt.Errorf("context: %w", err)
}
```

### Context Usage
```go
func (r *Renderer) Run(ctx context.Context) error {
    // Always pass context to long-running operations
    return r.processBundles(ctx)
}
```

### Logging
```go
import "github.com/sirupsen/logrus"

logrus.Info("Processing bundle", "image", image)
logrus.Debug("Bundle details", "bundle", bundle)
```

## Testing Patterns

### Unit Tests
```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid input", "test", "expected"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            require.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Tests
- Use `test/e2e/` directory
- Test with real bundle images
- Verify end-to-end workflows

## Debugging Tips

### Common Issues
1. **Bundle validation errors**: Check bundle format and annotations
2. **Image pull failures**: Verify image references and registry access
3. **Template rendering errors**: Check template syntax and schema
4. **Cache integrity failures**: Rebuild cache or disable integrity checks
5. **gRPC connection issues**: Verify port availability and firewall settings
6. **Memory issues with large catalogs**: Use persistent cache directory
7. **Slow startup times**: Pre-build cache with `--cache-only` flag

### Debug Commands
```bash
# Validate bundle
opm alpha bundle validate ./bundle

# Debug template rendering
opm alpha render-template --help

# Serve with debug logging
opm serve ./catalog --debug

# Serve with profiling enabled
opm serve ./catalog --pprof-addr localhost:6060 --pprof-capture-profiles

# Test gRPC connectivity
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 api.Registry/ListPackages
```

## Contributing Guidelines

### Code Style
- Follow Go standard formatting (`gofmt`)
- Use `golangci-lint` for additional checks
- Write comprehensive tests
- Document public APIs

### Commit Messages
- Use conventional commit format
- Include context and reasoning
- Reference issues when applicable

### Pull Request Process
1. Ensure all tests pass
2. Run `make lint` and fix issues
3. Update documentation if needed
4. Request review from OWNERS

## Resources

### Documentation
- [Operator Bundle Design](docs/design/operator-bundle.md)
- [OPM Tooling](docs/design/opm-tooling.md)
- [Bundle Sources](docs/design/bundle-sources.md)

### Related Projects
- [Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager)
- [Operator SDK](https://github.com/operator-framework/operator-sdk)
- [OLM Book](https://operator-framework.github.io/olm-book/)

### Community
- [Operator Framework Slack](https://kubernetes.slack.com/channels/operator-framework)
- [GitHub Discussions](https://github.com/operator-framework/operator-registry/discussions)

## Security Considerations

### Image Security
- Always verify bundle image signatures
- Use trusted registries
- Scan images for vulnerabilities

### Data Handling
- Validate all input data
- Sanitize user-provided content
- Use secure defaults

### Network Security
- Use TLS for gRPC connections
- Implement proper authentication
- Validate network policies

## Performance Considerations

### Database Optimization
- Use appropriate indexes
- Batch operations when possible
- Monitor query performance

### Memory Management
- Use streaming for large datasets
- Implement proper cleanup
- Monitor memory usage

### Caching
- Cache frequently accessed data
- Implement cache invalidation
- Use appropriate cache sizes

---

This document should be updated as the project evolves. For specific implementation details, refer to the source code and inline documentation.

