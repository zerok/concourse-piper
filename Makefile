all: concourse-piper

concourse-piper: $(shell find . -name '*.go') go.mod
	go build -o $@

test:
	go test -v ./...

clean:
	rm -f concourse-piper

.PHONY: test clean
