.PHONY: build test lint clean install

build:
	go build -o bws .
	ln -sf bws bw

test:
	go vet ./...
	go test ./...

lint:
	staticcheck ./...

install:
	./tools/install

clean:
	rm -f bws bw
	go clean