# Build Instructions

## Version Management

The project uses a `VERSION` file for version management:

```bash
cat VERSION          # Show current version
echo "0.3.0" > VERSION  # Update version
```

## Building

### Standard Build (with version injection)
```bash
make build           # Builds cyq binary with proper version
./cyq version        # Shows: cyq version 0.2.0
```

### Development Build
```bash
go run ./cmd/cyq version   # Shows: cyq version dev
```

### Installation
```bash
make install         # Installs cyq with proper version to $GOPATH/bin
```

## Version Injection Details

The build system automatically injects the version from `VERSION` file into the binary using Go's `-ldflags`:

```makefile
VERSION := $(shell cat VERSION)
LDFLAGS := -ldflags "-X github.com/seuros/gopher-cypher/src/internal/boltutil.LibraryVersion=$(VERSION)"
```

This sets the `LibraryVersion` variable at compile time, which is used in:
- CLI version command: `cyq version`
- Driver user agent: `gopher-cypher::Bolt/0.2.0`
- API functions: `driver.Version()` and `driver.UserAgent()`

## Development vs Production

- **Development**: Version shows as `"dev"` when using `go run`
- **Production**: Version shows actual version when using `make build` or `make install`

This ensures developers can easily distinguish between built releases and development builds.
