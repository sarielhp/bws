.PHONY: build test lint clean install audit

build:
	go build -o bws .

test:
	go vet ./...
	go test ./...

lint:
	staticcheck ./...

audit:
	./tools/audit_lines.rb

install:
	./tools/install

clean:
	rm -f bws
	go clean