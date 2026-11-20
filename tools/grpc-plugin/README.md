This module pins `protoc-gen-go-grpc` to v1.3.0, the version previously managed
by bingo. The root `go.mod` selects v1.6.2. Go selects one version of each
module for a module graph, so a tool directive in the root module would run
v1.6.2. That version changes exported gRPC stream interfaces in `pkg/api` and
the method signatures in `pkg/server`, which fails the API compatibility check.
Keeping the root runtime dependencies also prevents pinning v1.3.0 there:
module resolution raises the selected generator module to at least v1.6.1.

`tools/protoc-gen-go-grpc` runs this module's v1.3.0 binary with
`go tool -modfile=tools/grpc-plugin/go.mod` from the repository root.
