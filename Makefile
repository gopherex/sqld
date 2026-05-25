

.PHONY: build
build: ## Build the sqld and sqld-gen-go binaries
	go build -o bin/sqld ./cmd/sqld
	go build -o bin/sqld-gen-go ./cmd/sqld-gen-go

.PHONY: example
example: build ## Generate code + dump IR for example/
	./bin/sqld generate -c example/sqld.yaml
	./bin/sqld collect -c example/sqld.yaml -format json > example/gen/catalog.json
	go build ./example/...

.PHONY: protocols
protocols: # Generate Go + TS code from proto
	cd proto && easyp mod update && easyp mod vendor
	rm -rf $(CURDIR)/pkg/proto
	cd proto && easyp generate
