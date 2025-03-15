# Contributing to Gopher Cypher

Thank you for taking the time to contribute! Please follow the steps below.

## Development setup

1. Install Go 1.25 or later.
2. Clone the repository and run `go mod download`.
3. Optionally, start Neo4j and Memgraph using `docker-compose up -d`.

## Running tests

Run all tests with:

```
go test ./...
```

Some tests require Neo4j and Memgraph running locally on their default ports.
Tests attempt a short connection when creating a driver and skip with
`database not available` if the services are not reachable.

## Code style

- Format code using `gofmt` (`go fmt ./...`).
- Keep pull requests focused and include tests when possible.

## Submitting changes

1. Fork the repository and create a feature branch.
2. Ensure tests pass: `go test ./...`.
3. Submit a pull request describing your changes.

