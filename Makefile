# b9s Makefile
#
# Build with SQLite FTS5 (full-text search) support enabled

.PHONY: build install clean test

# Enable FTS5 for full-text search in SQLite exports
export CGO_CFLAGS := -DSQLITE_ENABLE_FTS5

build:
	go build -o b9s ./cmd/b9s

install:
	go install ./cmd/b9s

clean:
	rm -f b9s
	go clean

test:
	go test ./...
