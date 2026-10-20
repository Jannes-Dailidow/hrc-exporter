set windows-shell := ["pwsh", "-NoLogo", "-NoProfile", "-Command"]

bin     := "bin/hrc-exporter" + if os_family() == "windows" { ".exe" } else { "" }
version := env("VERSION", "dev")
ldflags := "-s -w -X main.version=" + version

# list recipes
default:
    @just --list

# build the binary into ./bin
build: vet
    go build -trimpath -ldflags "{{ldflags}}" -o {{bin}} ./cmd/hrc-exporter

# install into $GOBIN
install: vet
    go install -trimpath -ldflags "{{ldflags}}" ./cmd/hrc-exporter

# run from source, e.g. `just run download --dry-run`
run *args:
    go run ./cmd/hrc-exporter {{args}}

fmt:
    gofmt -w .

vet:
    go vet ./...

test:
    go test ./...

# core must not depend on cli/ui libraries
check-deps:
    go test ./internal/archtest

# fmt + vet + test
check: fmt vet test

tidy:
    go mod tidy

clean:
    {{ if os_family() == "windows" { "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue bin" } else { "rm -rf bin" } }}
