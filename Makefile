.PHONY: build test lint clean install

build:
	go build -o bws .

test:
	go vet ./...
	go test ./...

lint:
	staticcheck ./...

install:
	./tools/install

clean:
	rm -f bws
	go clean