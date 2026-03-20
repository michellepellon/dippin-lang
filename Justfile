# dippin-lang development tasks

# List available recipes
default:
    @just --list

# Build the dippin binary
build:
    go build -o dippin ./cmd/dippin/

# Install dippin globally to $GOBIN
install:
    go install ./cmd/dippin/

# Run all tests
test:
    go test ./... -count=1

# Run all tests with race detector
test-race:
    go test ./... -count=1 -race

# Run tests for a specific package (e.g., just test-pkg validator)
test-pkg pkg:
    go test ./{{pkg}}/... -count=1 -v

# Run go vet
vet:
    go vet ./...

# Check formatting (exit 1 if unformatted)
fmt-check:
    @unformatted=$(gofmt -l . 2>&1); \
    if [ -n "$unformatted" ]; then \
        echo "Files not gofmt'd:"; \
        echo "$unformatted"; \
        exit 1; \
    fi

# Format all Go files
fmt:
    gofmt -w .

# Validate all example .dip files
validate-examples: build
    @for f in examples/*.dip; do \
        ./dippin validate "$f" > /dev/null 2>&1 || { \
            echo "FAIL: $f"; \
            ./dippin validate "$f" 2>&1; \
            exit 1; \
        }; \
    done
    @echo "All examples valid."

# Lint all example .dip files
lint-examples: build
    @for f in examples/*.dip; do \
        echo "--- $f ---"; \
        ./dippin lint "$f" 2>&1 || true; \
    done

# Run the full pre-commit check suite (build, vet, fmt, test, validate examples)
check: build vet fmt-check test-race validate-examples
    @echo "All checks passed."

# Generate test coverage report
cover:
    go test ./... -coverprofile=cover.out
    go tool cover -func=cover.out

# Open coverage in browser
cover-html:
    go test ./... -coverprofile=cover.out
    go tool cover -html=cover.out

# Install the pre-commit hook
setup-hooks:
    ln -sf ../../scripts/pre-commit .git/hooks/pre-commit
    chmod +x .git/hooks/pre-commit
    @echo "Pre-commit hook installed."

# Clean build artifacts
clean:
    rm -f dippin cover.out cover_check.out
    @echo "Cleaned."
