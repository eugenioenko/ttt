# Makefile for ttt - terminal text editor

.PHONY: all test build run clean fmt lint chaos chaos-docker chaos-docker-build profiler

all: build

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/ttt ./cmd/ttt

test:
	go test ./...

run: build
	./bin/ttt

fmt:
	gofmt -w .

lint:
	golangci-lint run

vet:
	go vet ./...

chaos: chaos-docker-build
	mkdir -p chaos-output
	docker run --rm -v $(PWD)/chaos-output:/output --entrypoint /chaos-test ttt-chaos \
		-test.run TestChaosMonkey -test.v -test.timeout 15m

chaos-docker-build:
	docker build -t ttt-chaos -f tests/chaos/Dockerfile .

chaos-docker:
	mkdir -p chaos-output
	docker run --rm -v $(PWD)/chaos-output:/output ttt-chaos

# Usage: CHAOS_REPLAY=chaos-output/crash-<seed>-<iter>.json make chaos-replay
chaos-replay: chaos-docker-build
	docker run --rm -v $(PWD)/chaos-output:/output \
		-e CHAOS_REPLAY=/output/$(notdir $(CHAOS_REPLAY)) \
		--entrypoint /chaos-test ttt-chaos -test.run TestChaosReplay -test.v

profiler:
	go build -tags profiler -ldflags="-X main.version=$(VERSION)" -o bin/ttt-profiler ./cmd/ttt

clean:
	rm -rf bin/
	find . -name '*.test' -delete
