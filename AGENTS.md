# AGENTS.md

This document provides guidance for AI agents working with the Operator Registry project.

## Project Overview

The Operator Registry is a Kubernetes/OpenShift component that provides operator catalog data to the Operator Lifecycle Manager (OLM). It manages operator bundles, indexes, and catalogs that enable operators to be discovered and installed in Kubernetes clusters.

## Key Components

### Binaries
- **`opm`**: Main CLI tool for generating and updating registry databases and index images
- **`initializer`**: **(Deprecated)** Converts operator manifests to SQLite database format
- **`registry-server`**: **(Deprecated)** Exposes gRPC interface to SQLite databases
- **`configmap-server`**: Parses ConfigMaps into SQLite databases and exposes gRPC interface

### Libraries
- **`pkg/client`**: High-level client interface for gRPC API
- **`pkg/api`**: Low-level client libraries for gRPC interface
- **`pkg/registry`**: Core registry types (Packages, Channels, Bundles)
- **`pkg/sqlite`**: **(Deprecated)** SQLite database interfaces for manifests
- **`pkg/lib`**: External interfaces and standards for operator bundles
- **`pkg/containertools`**: Container tooling integration

### Alpha Features
- **`alpha/declcfg`**: Declarative configuration format
- **`alpha/template`**: Template system for generating catalogs
- **`alpha/action`**: Action framework for registry operations

## Development Guidelines

### Code Structure
- **Go 1.24.4**: Minimum Go version required
- **Cobra CLI**: Command-line interface framework
- **gRPC**: Primary API communication protocol
- **SQLite**: Database backend for registry data
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

# Add bundle to index (deprecated - use file-based catalogs instead)
opm index add --bundles quay.io/my-namespace/my-bundle:latest --from-index quay.io/my-namespace/my-index:latest --tag quay.io/my-namespace/my-index:latest

# Generate index image (deprecated - use file-based catalogs instead)
opm index add --bundles quay.io/my-namespace/my-bundle:latest --tag quay.io/my-namespace/my-index:latest
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

### 4. Database Operations **(Deprecated)**
```go
// Create SQLite database
db, err := sqlite.Open("registry.db")

// Add bundle to database
err = db.AddBundle(bundle)

// Query packages
packages, err := db.ListPackages()
```

### 5. Serving Catalog Content with `opm serve`
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

## Deprecated SQLite Commands and Operations

⚠️ **IMPORTANT DEPRECATION NOTICE**: SQLite-based catalogs and their related subcommands are deprecated. Support for them will be removed in a future release. Please migrate your catalog workflows to the new file-based catalog format.

### Deprecated Commands

#### `opm registry` Commands (All Deprecated)
- **`opm registry serve`** - **(Deprecated)** Serve SQLite database via gRPC
- **`opm registry add`** - **(Deprecated)** Add bundles to SQLite database
- **`opm registry rm`** - **(Deprecated)** Remove packages from SQLite database
- **`opm registry prune`** - **(Deprecated)** Prune SQLite database
- **`opm registry prune-stranded`** - **(Deprecated)** Prune stranded bundles from SQLite database
- **`opm registry deprecatetruncate`** - **(Deprecated)** Deprecate bundles in SQLite database
- **`opm registry mirror`** - **(Deprecated)** Mirror SQLite-based catalogs

#### `opm index` Commands (All Deprecated)
- **`opm index add`** - **(Deprecated)** Add bundles to SQLite-based index
- **`opm index rm`** - **(Deprecated)** Delete operators from SQLite-based index
- **`opm index export`** - **(Deprecated)** Export SQLite-based index
- **`opm index prune`** - **(Deprecated)** Prune SQLite-based index
- **`opm index prune-stranded`** - **(Deprecated)** Prune stranded bundles from SQLite-based index
- **`opm index deprecatetruncate`** - **(Deprecated)** Deprecate bundles in SQLite-based index

### Deprecated Libraries and Interfaces

#### `pkg/sqlite` Package (Deprecated)
- **`SQLQuerier`** - **(Deprecated)** Query interface for SQLite databases
- **`SQLLoader`** - **(Deprecated)** Load bundles into SQLite databases
- **`DeprecationAwareLoader`** - **(Deprecated)** Handle bundle deprecations in SQLite
- **`SQLDeprecator`** - **(Deprecated)** Deprecate bundles in SQLite databases

#### `pkg/lib/registry` Package (Deprecated)
- **`RegistryUpdater`** - **(Deprecated)** Update SQLite-based registries
- **`RegistryDeleter`** - **(Deprecated)** Delete from SQLite-based registries
- **`RegistryDeprecator`** - **(Deprecated)** Deprecate bundles in SQLite-based registries

### Migration Path

**From SQLite to File-Based Catalogs:**

1. **Replace `opm registry serve`** → **Use `opm serve`**
   ```bash
   # Old (deprecated)
   opm registry serve --database bundles.db
   
   # New (recommended)
   opm serve ./catalog-directory
   ```

2. **Replace `opm index add`** → **Use `opm alpha generate dockerfile` + `opm serve`**
   ```bash
   # Old (deprecated)
   opm index add --bundles quay.io/my/bundle:latest --tag quay.io/my/index:latest
   
   # New (recommended)
   opm alpha generate dockerfile ./catalog-directory
   # Build and serve the generated Dockerfile
   ```

3. **Replace SQLite database operations** → **Use declarative config files**
   ```bash
   # Old (deprecated) - SQLite database manipulation
   opm registry add --database bundles.db --bundles quay.io/my/bundle:latest
   
   # New (recommended) - File-based catalog
   # Create catalog files directly in ./catalog-directory/
   ```

4. **Convert existing SQLite catalogs** → **Use migration tools**
   ```bash
   # Convert SQLite index image to FBC
   opm migrate quay.io/my-namespace/my-index:latest ./fbc-output --output yaml
   
   # Convert SQLite database to FBC
   opm render ./bundles.db --output yaml > catalog.yaml
   
   # Convert FBC to a basic catalog template for simpler maintenance
   opm alpha render-template basic ./catalog.yaml
   ```

### Deprecation Warnings

When using deprecated commands, you will see warnings like:
```
DEPRECATION NOTICE:
Sqlite-based catalogs and their related subcommands are deprecated. Support for
them will be removed in a future release. Please migrate your catalog workflows
to the new file-based catalog format.
```

**Action Required**: Plan your migration to file-based catalogs to avoid future compatibility issues.

### Migration from SQLite to File-Based Catalogs (FBC)

#### Using `opm migrate`
The `opm migrate` command converts SQLite-based index images or database files to file-based catalogs:

```bash
# Migrate from SQLite index image to FBC
opm migrate quay.io/my-namespace/my-index:latest ./fbc-output --output yaml

# Migrate from SQLite database file to FBC
opm migrate ./bundles.db ./fbc-output --output json

# Migrate with specific migration level
opm migrate quay.io/my-namespace/my-index:latest ./fbc-output
```

#### Using `opm render`
The `opm render` command can convert various sources to FBC format:

```bash
# Render from SQLite database to FBC
opm render ./bundles.db --output yaml > catalog.yaml

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

#### Combined Migration Workflow
For complex migrations, combine multiple tools:

```bash
# Step 1: Migrate SQLite to FBC
opm migrate quay.io/my-namespace/my-index:latest ./fbc-output --output yaml

# Step 2: Convert to basic template format (if needed)
opm alpha render-template basic ./fbc-output/package.yaml --output yaml > template.yaml

# Step 3: Modify template as needed, then render final FBC
opm alpha render-template basic ./template.yaml --output yaml > final-catalog.yaml

# Step 4: Serve the new FBC catalog
opm serve ./final-catalog.yaml
```

#### Migration Considerations
- **Bundle images must exist** in registries before they can be referenced in FBC
- **Image references are required** - use `--alpha-image-ref-template` if bundle images don't exist yet
- **Migration levels** control which schema transformations are applied
- **Output formats** include JSON (streamable) and YAML (human-readable)
- **Templates provide flexibility** for custom catalog generation workflows

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
3. **Database corruption**: Rebuild database from source bundles
4. **Template rendering errors**: Check template syntax and schema
5. **Cache integrity failures**: Rebuild cache or disable integrity checks
6. **gRPC connection issues**: Verify port availability and firewall settings
7. **Memory issues with large catalogs**: Use persistent cache directory
8. **Slow startup times**: Pre-build cache with `--cache-only` flag
9. **SQLite deprecation warnings**: Migrate to file-based catalogs using `opm serve`
10. **Hidden commands**: Some deprecated commands are hidden but still accessible

### Debug Commands
```bash
# Validate bundle
opm alpha bundle validate ./bundle

# Inspect index
opm index export --from-index quay.io/my-namespace/my-index:latest

# Debug template rendering
opm alpha render-template --help

# Serve with debug logging
opm serve ./catalog --debug

# Serve with profiling enabled
opm serve ./catalog --pprof-addr localhost:6060 --pprof-capture-profiles

# Test gRPC connectivity
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 api.Registry/ListPackages

# Deprecated SQLite commands (avoid using these)
opm registry serve --database bundles.db  # Use 'opm serve' instead
opm index add --bundles quay.io/my/bundle:latest --tag quay.io/my/index:latest  # Use file-based catalogs
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

