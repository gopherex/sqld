

.PHONY: build
build: ## Build the sqld, sqld-gen-go, sqld-gen-bob, and sqld-migrate binaries
	go build -o bin/sqld ./cmd/sqld
	go build -o bin/sqld-gen-go ./cmd/sqld-gen-go
	go build -o bin/sqld-gen-bob ./cmd/sqld-gen-bob
	go build -o bin/sqld-migrate ./cmd/sqld-migrate

.PHONY: build-wasm
build-wasm: ## Build sqld-gen-go as a wasip1 WASM plugin
	GOOS=wasip1 GOARCH=wasm go build -o bin/sqld-gen-go.wasm ./cmd/sqld-gen-go

.PHONY: example
example: build ## Generate code + dump IR for example/
	./bin/sqld generate -c example/sqld.yaml
	./bin/sqld collect -c example/sqld.yaml -format json > example/gen/catalog.json
	go build ./example/...

.PHONY: example-wasm
example-wasm: build build-wasm ## Generate the example via the WASM plugin
	./bin/sqld generate -c example/sqld.wasm.yaml
	go build ./example/...

.PHONY: protocols
protocols: # Generate Go + TS code from proto
	cd proto && easyp mod update && easyp mod vendor
	rm -rf $(CURDIR)/pkg/proto
	cd proto && easyp generate
