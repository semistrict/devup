.PHONY: build test install clean

build:
	go build -o bin/devup .

test:
	go test ./...

install:
	go install .

clean:
	rm -rf bin/
