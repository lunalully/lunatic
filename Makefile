BIN     := bin/lunatic
VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo 0.1.0)
LDFLAGS := -s -w -X github.com/lunalully/lunatic/internal/version.Version=$(VERSION)
PREFIX  ?= $(shell if [ -w /usr/local/bin ]; then echo /usr/local/bin; else echo $(HOME)/go/bin; fi)

.PHONY: build test race vet fmt check install clean smoke

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/lunatic

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w cmd internal

check: vet test race
	@test -z "$$(gofmt -l cmd internal)" || (echo "gofmt needed"; gofmt -l cmd internal; exit 1)

install: build
	install -d $(PREFIX)
	install -m 0755 $(BIN) $(PREFIX)/lunatic

clean:
	rm -rf bin dist coverage.out

# live smoke test of no-key sources (needs network; see scripts/live-smoke.sh)
smoke:
	scripts/live-smoke.sh
