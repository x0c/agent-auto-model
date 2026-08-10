VERSION ?= $(shell tr -d '[:space:]' < VERSION)
BIN ?= $(HOME)/.local/bin/cursor-mode-model

.PHONY: test build install smoke

test:
	go test ./...
	CURSOR_MODE_MODEL_UNIT_TEST=1 node --test ./internal/assets/register.test.mjs

build:
	go build -ldflags="-X main.version=$(VERSION)" -o bin/cursor-mode-model ./cmd/cursor-mode-model

install: build
	install -m 755 bin/cursor-mode-model $(BIN)
	$(BIN) install

smoke: install
	$(BIN) status
	$(BIN) config show --json >/dev/null
	PATH="$(HOME)/.local/share/cursor-mode-model/bin:$$PATH" agent --help >/dev/null
	@echo smoke ok
