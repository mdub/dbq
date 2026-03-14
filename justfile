# Build the project
build:
    go build ./...

# Run the test suite
test:
    go test ./...

# Apply formatting and linting
format *paths:
    gofmt -w {{ if paths == "" { "." } else { paths } }}

# Run lint checks
lint:
    golangci-lint run

# Run all CI checks
ci: lint test build
