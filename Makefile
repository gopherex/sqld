

.PHONY: build
build: ## Build the core binaries (sqld, sqld-gen-go, sqld-migrate)
	go build -o bin/sqld ./cmd/sqld
	go build -o bin/sqld-gen-go ./cmd/sqld-gen-go
	go build -o bin/sqld-migrate ./cmd/sqld-migrate

.PHONY: build-bob
build-bob: ## Build the sqld-gen-bob plugin (nested module cmd/sqld-gen-bob; needs go.work)
	go build -o bin/sqld-gen-bob ./cmd/sqld-gen-bob

.PHONY: build-wasm
build-wasm: ## Build sqld-gen-go as a wasip1 WASM plugin
	GOOS=wasip1 GOARCH=wasm go build -o bin/sqld-gen-go.wasm ./cmd/sqld-gen-go

.PHONY: example
example: build ## Generate code + dump IR for example/ (core: sqld-gen-go)
	./bin/sqld generate -c example/sqld.yaml
	./bin/sqld collect -c example/sqld.yaml -format json > example/gen/catalog.json
	go build ./example/...

.PHONY: example-wasm
example-wasm: build build-wasm ## Generate the example via the WASM plugin
	./bin/sqld generate -c example/sqld.wasm.yaml
	go build ./example/...

.PHONY: example-bob
example-bob: build build-bob ## Generate the bob ORM (cmd/sqld-gen-bob module) + build it
	./bin/sqld generate -c cmd/sqld-gen-bob/example/sqld.yaml
	go build ./cmd/sqld-gen-bob/...

.PHONY: protocols
protocols: # Generate Go + TS code from proto
	cd proto && easyp mod update && easyp mod vendor
	rm -rf $(CURDIR)/pkg/proto
	cd proto && easyp generate
