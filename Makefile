VERSION ?= dev

.PHONY: build run test clean

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o telvar ./cmd/telvar

run: build
	./telvar serve

test:
	go test ./... -v

clean:
	rm -f telvar telvar.db
