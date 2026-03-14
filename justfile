# Build the project
build:
    go build ./...

# Run the test suite with coverage check
test:
    #!/usr/bin/env bash
    set -euo pipefail
    go test ./... -coverpkg=./output/ -coverprofile=/tmp/dbq-coverage.out -covermode=atomic
    total=$(go tool cover -func=/tmp/dbq-coverage.out | awk '/^total:/ {gsub(/%/,""); print $NF}')
    echo "Coverage: ${total}%"
    threshold=80.0
    if awk "BEGIN {exit ($total >= $threshold ? 1 : 0)}"; then
        echo "FAIL: coverage ${total}% is below ${threshold}%"
        exit 1
    fi

# Apply formatting and linting
format *paths:
    gofmt -w {{ if paths == "" { "." } else { paths } }}

# Run lint checks
lint:
    golangci-lint run

# Run all CI checks
ci: lint test build
