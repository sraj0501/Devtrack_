.PHONY: build build-dev test release bump-patch bump-minor bump-major clean install server-start server-stop

# ----- Build ---------------------------------------------------------------

VERSION  ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE     ?= $(shell date -u +%FT%TZ)
LDFLAGS  := -s -w \
            -X main.Version=$(VERSION) \
            -X main.GitCommit=$(COMMIT) \
            -X main.BuildTime=$(DATE)

## build: compile the devtrack binary
build:
	cd devtrack_client && go build -ldflags="$(LDFLAGS)" -o ../devtrack .
	@echo "✅ Built devtrack $(VERSION) ($(COMMIT))"

## build-dev: fast build for local dev iteration
build-dev: build

## install: build and install to /usr/local/bin
install: build
	sudo cp devtrack /usr/local/bin/devtrack
	@echo "✅ Installed to /usr/local/bin/devtrack"

# ----- Test ----------------------------------------------------------------

## test: run Go and Python tests
test: go-test python-test

## go-test: run Go tests
go-test:
	cd devtrack_client && go test ./... && go vet ./...

## python-test: run Python tests
python-test:
	cd devtrack_server && uv run pytest backend/tests/ -q --tb=short

# ----- Python Server -------------------------------------------------------

## server-start: start the Python backend server locally (requires PROJECT_ROOT set)
server-start:
	cd devtrack_server && uv run python -m backend.webhook_server &
	@echo "✅ Python backend server started"

## server-stop: stop the locally running Python backend server
server-stop:
	-pkill -f webhook_server
	@echo "✅ Python backend server stopped"

# ----- Release -------------------------------------------------------------

## release-dry: dry-run GoReleaser (does not publish)
release-dry:
	goreleaser release --snapshot --clean

## bump-patch: increment patch version and push tag → triggers release
bump-patch:
	@bash scripts/bump-version.sh patch

## bump-minor: increment minor version and push tag → triggers release
bump-minor:
	@bash scripts/bump-version.sh minor

## bump-major: increment major version and push tag → triggers release
bump-major:
	@bash scripts/bump-version.sh major

# ----- Misc ----------------------------------------------------------------

## clean: remove build artifacts
clean:
	rm -f devtrack devtrack_client/devtrack

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
