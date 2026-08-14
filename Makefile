VERSION ?= $(shell tr -d '[:space:]' < VERSION)
BIN ?= $(HOME)/.local/bin/agent-auto-model
ALIAS ?= $(HOME)/.local/bin/cursor-mode-model

.PHONY: test build install smoke

test:
	go test ./...
	AGENT_AUTO_MODEL_UNIT_TEST=1 CURSOR_MODE_MODEL_UNIT_TEST=1 node --test ./internal/assets/register.test.mjs

build:
	go build -ldflags="-X main.version=$(VERSION)" -o bin/agent-auto-model ./cmd/agent-auto-model
	go build -ldflags="-X main.version=$(VERSION)" -o bin/cursor-mode-model ./cmd/cursor-mode-model

install: build
	install -m 755 bin/agent-auto-model $(BIN)
	install -m 755 bin/cursor-mode-model $(ALIAS)
	$(BIN) install

smoke: install
	$(BIN) status
	$(BIN) config show --json >/dev/null
	PATH="$(HOME)/.local/share/agent-auto-model/bin:$$PATH" agent --help >/dev/null
	@echo smoke ok
