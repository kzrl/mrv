BINARY  := bin/mrv
MAIN    := main.go
LDFLAGS := -ldflags="-s -w"

.PHONY: all build run clean test lint tidy install config

all: build

build:
	go build $(LDFLAGS) -o $(BINARY) $(MAIN)

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

install:
	go install $(LDFLAGS) .

config:
	@echo "Config search order:"
	@echo "  1. ./mrv.cue"
	@echo "  2. $${XDG_CONFIG_HOME:-$$HOME/.config}/mrv/config.cue"
	@echo ""
	@echo "Sample mrv.cue:"
	@echo '  model: {'
	@echo '    url:  "http://localhost:8080/v1"'
	@echo '    name: "local-model"'
	@echo '  }'
	@echo '  agent: {'
	@echo '    maxRetries:    3'
	@echo '    maxIterations: 0'
	@echo '  }'
	@echo '  tools: {'
	@echo '    shell:     {requireConfirmation: true}'
	@echo '    writeFile: {requireConfirmation: true}'
	@echo '  }'

clean:
	rm -f $(BINARY)
