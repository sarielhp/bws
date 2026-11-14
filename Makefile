BIN_NAME := bin/bws

# Copy or include; use tools/commit directly for messages with shell metacharacters.
.PHONY: all build gate check ci lint snapshot commit bump install release test audit review static-analysis clean long
all: build
build:
	tools/build
gate:
	tools/gate
check:
	tools/check
ci:
	tools/gate --full
lint:
	tools/lint
snapshot:
	tools/snapshot $(ARGS)
commit:
	tools/commit "$(msg)"
bump:
	tools/bump
install:
	tools/install
release:
	tools/release $(ARGS)
test:
	go test -timeout 180s ./...
audit:
	go-audit --strict --quiet .
review: static-analysis
static-analysis:
	go-static-analysis
clean:
	go clean
	rm -f $(BIN_NAME)

long:
	tools/test_long
