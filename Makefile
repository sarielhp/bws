.PHONY: build test lint clean install

build:
	go build -o bw .

test:
	go vet ./...
	go test ./...

lint:
	staticcheck ./...

install:
	./tools/install

clean:
	rm -f bw
	go clean