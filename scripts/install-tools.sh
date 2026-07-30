#!/bin/bash

# format & lint tools
if ! command -v gofumpt &> /dev/null; then
    echo "Installing gofumpt..."
    go install mvdan.cc/gofumpt@latest
else
    echo "gofumpt already installed"
fi

if ! command -v goimports &> /dev/null; then
    echo "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
else
    echo "goimports already installed"
fi

if ! command -v golangci-lint &> /dev/null; then
    echo "Installing golangci-lint..."
    # must be built with a Go >= the go directive in go.mod, else it refuses to load the config
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(go env GOPATH)/bin v2.12.2
else
    echo "golangci-lint already installed"
fi

if ! command -v deadcode &> /dev/null; then
    echo "Installing deadcode..."
    go install golang.org/x/tools/cmd/deadcode@latest
else
    echo "deadcode already installed"
fi

if ! command -v govulncheck &> /dev/null; then
    echo "Installing govulncheck..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
else
    echo "govulncheck already installed"
fi

# golang-migrate — DB migration runner used by `task migrate_up RECIPE=...`
if ! command -v migrate &> /dev/null; then
    echo "Installing golang-migrate..."
    go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
else
    echo "migrate already installed"
fi

# air — hot-reload loop used by `task run RECIPE=...`
if ! command -v air &> /dev/null; then
    echo "Installing air..."
    go install github.com/air-verse/air@latest
else
    echo "air already installed"
fi

# k6 — load testing, used by `task load_test RECIPE=...`
if ! command -v k6 &> /dev/null; then
    echo "k6 not found. Install via your package manager, e.g.:"
    echo "  macOS: brew install k6"
    echo "  Linux (Debian/Ubuntu):"
    echo "    sudo gpg -k && sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69"
    echo "    echo 'deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main' | sudo tee /etc/apt/sources.list.d/k6.list"
    echo "    sudo apt-get update && sudo apt-get install k6"
else
    echo "k6 already installed"
fi

# install node.js tools
pnpm install

# install lefthook (git hooks manager) if missing — pinned via @vX.Y.Z
if ! command -v lefthook >/dev/null 2>&1; then
  if command -v go >/dev/null 2>&1; then
    go install github.com/evilmartians/lefthook/v2@v2.1.8
  else
    echo "go not found on PATH; skipping lefthook install (git hooks will NOT be wired)"
  fi
fi

# wire git hooks via lefthook (config lives in lefthook.yml)
if command -v lefthook >/dev/null 2>&1; then
  lefthook install
fi
