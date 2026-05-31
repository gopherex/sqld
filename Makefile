SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c

ROOT_MODULE := github.com/gopherex/sqld
# Max major allowed. v2+ needs semantic import versioning (/vN in module
# paths), which we don't support yet — keep releases on v0/v1.
MAX_MAJOR := 1

# VERSION stamped into the binaries (`sqld --version`). Defaults to the current
# git tag/describe; a release overrides it with the pushed tag.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# Cross-build targets for `make dist` (GOOS/GOARCH).
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: help build build-bob build-wasm dist release test tidy example example-wasm example-bob protocols

help:
	@echo "make build        - build core binaries (sqld [incl. migrate], sqld-gen-go)"
	@echo "make build-bob    - build the optional sqld-gen-bob plugin (nested module)"
	@echo "make build-wasm   - build sqld-gen-go as a wasip1 WASM plugin"
	@echo "make dist         - cross-build all binaries for every platform into dist/"
	@echo "make release      - interactive tag + push (root + sqld-gen-bob); CI builds dist"
	@echo "make test         - build/vet/test every module (isolated, like CI)"
	@echo "make tidy         - go mod tidy in every module"

# --- build ------------------------------------------------------------------
# sqld now bundles the migrator as `sqld migrate ...`; there is no separate
# sqld-migrate binary. sqld-gen-go / sqld-gen-bob stay separate (they are
# plugins invoked over stdio/WASM).
build:
	go build -ldflags "$(LDFLAGS)" -o bin/sqld ./cmd/sqld
	go build -ldflags "$(LDFLAGS)" -o bin/sqld-gen-go ./cmd/sqld-gen-go

build-bob: ## Build the sqld-gen-bob plugin (nested module cmd/sqld-gen-bob; needs go.work)
	go build -ldflags "$(LDFLAGS)" -o bin/sqld-gen-bob ./cmd/sqld-gen-bob

build-wasm: ## Build sqld-gen-go as a wasip1 WASM plugin
	GOOS=wasip1 GOARCH=wasm go build -o bin/sqld-gen-go.wasm ./cmd/sqld-gen-go

# --- release ----------------------------------------------------------------
# Every go.mod in the repo: '.' is the root module (tag vX.Y.Z); cmd/sqld-gen-bob
# is the nested plugin module, tagged with its path prefix (cmd/sqld-gen-bob/vX.Y.Z)
# so `go install .../cmd/sqld-gen-bob@vX.Y.Z` resolves.
MODDIRS = $(shell find . -name go.mod -not -path './.git/*' -printf '%h\n' | sed 's#^\./##' | sort)

test:
	@for d in $(MODDIRS); do
	  echo "== $$d =="
	  if [ "$$d" = "." ]; then
	    # Root is the standalone published artifact: build it without the
	    # workspace to validate module independence.
	    ( cd "$$d" && GOWORK=off go build ./... && GOWORK=off go vet ./... && GOWORK=off go test ./... )
	  else
	    # Nested modules (sqld-gen-bob) depend on the root source via go.work.
	    ( cd "$$d" && go build ./... && go vet ./... && go test ./... )
	  fi
	done

tidy:
	@for d in $(MODDIRS); do ( cd "$$d" && go mod tidy ); done

# dist cross-builds every binary for every PLATFORM and packages one archive per
# platform (tar.gz; .zip for windows) plus a SHA256SUMS file in dist/.
# Each archive bundles: sqld, sqld-gen-go, sqld-gen-bob, sqld-gen-go.wasm, LICENSE, README.
dist:
	@rm -rf dist
	mkdir -p dist
	# Platform-independent WASM plugin, built once and shared by every archive.
	GOOS=wasip1 GOARCH=wasm go build -o dist/sqld-gen-go.wasm ./cmd/sqld-gen-go
	for plat in $(PLATFORMS); do
	  os=$${plat%/*}; arch=$${plat#*/}
	  ext=""; [ "$$os" = "windows" ] && ext=".exe"
	  stage="dist/sqld_$(VERSION)_$${os}_$${arch}"
	  mkdir -p "$$stage"
	  echo "== build $$os/$$arch =="
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "$$stage/sqld$$ext"        ./cmd/sqld
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "$$stage/sqld-gen-go$$ext" ./cmd/sqld-gen-go
	  # Built from the repo root so the workspace (go.work) resolves the root
	  # module from source; the install-facing require is a published version.
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o "$$stage/sqld-gen-bob$$ext" ./cmd/sqld-gen-bob
	  cp dist/sqld-gen-go.wasm LICENSE README.md "$$stage/"
	  if [ "$$os" = "windows" ]; then
	    ( cd dist && zip -qr "sqld_$(VERSION)_$${os}_$${arch}.zip" "sqld_$(VERSION)_$${os}_$${arch}" )
	  else
	    ( cd dist && tar -czf "sqld_$(VERSION)_$${os}_$${arch}.tar.gz" "sqld_$(VERSION)_$${os}_$${arch}" )
	  fi
	  rm -rf "$$stage"
	done
	rm -f dist/sqld-gen-go.wasm
	( cd dist && sha256sum * > SHA256SUMS )
	@echo "✓ dist/ ready:"; ls -1 dist

# Interactive release: bump or recreate a version, tag the root + nested module,
# and push. The pushed vX.Y.Z tag triggers .github/workflows/release.yml, which
# runs CI then `make dist` and uploads the archives to the GitHub release.
release:
	@set -euo pipefail
	cd "$$(git rev-parse --show-toplevel)"

	# 1. everything committed?
	if [ -n "$$(git status --porcelain)" ]; then
	  echo "✗ Working tree is not clean — commit or stash first:"
	  git status --short
	  exit 1
	fi

	mods="$(MODDIRS)"
	cur="$$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' | sed 's/^v//' | sort -t. -k1,1n -k2,2n -k3,3n | tail -1)"
	cur="$${cur:-0.0.0}"
	head="$$(git rev-parse --short HEAD)"
	echo "Latest release: v$$cur    HEAD: $$head"
	echo
	echo "  1) recreate last tag (v$$cur) on HEAD   [force]"
	echo "  2) bump version"
	echo "  3) cancel"
	read -r -p "> " action

	tags_for() { # $1 = version (without v); prints one tag per module
	  local v="$$1" d
	  for d in $$mods; do
	    if [ "$$d" = "." ]; then echo "v$$v"; else echo "$$d/v$$v"; fi
	  done
	}

	case "$$action" in
	1)
	  if [ "$$cur" = "0.0.0" ] && ! git tag -l 'v0.0.0' | grep -q .; then
	    echo "✗ No release tags to recreate."; exit 1
	  fi
	  mapfile -t TAGS < <(tags_for "$$cur")
	  echo
	  echo "Will DELETE and recreate $${#TAGS[@]} tags of v$$cur on $$head, then force-push."
	  read -r -p "Type 'yes' to proceed: " ok
	  [ "$$ok" = "yes" ] || { echo "Aborted."; exit 0; }
	  for t in "$${TAGS[@]}"; do
	    git tag -d "$$t" 2>/dev/null || true
	    git push origin ":refs/tags/$$t" 2>/dev/null || true
	  done
	  for t in "$${TAGS[@]}"; do git tag -a "$$t" -m "$$t"; done
	  git push origin --force "$${TAGS[@]}"
	  echo "✓ Recreated v$$cur on $$head."
	  ;;
	2)
	  IFS=. read -r MA MI PA <<< "$$cur"
	  echo
	  echo "  1) major  -> v$$((MA+1)).0.0"
	  echo "  2) minor  -> v$$MA.$$((MI+1)).0"
	  echo "  3) patch  -> v$$MA.$$MI.$$((PA+1))"
	  read -r -p "> " comp
	  case "$$comp" in
	    1) MA=$$((MA+1)); MI=0; PA=0 ;;
	    2) MI=$$((MI+1)); PA=0 ;;
	    3) PA=$$((PA+1)) ;;
	    *) echo "Aborted."; exit 0 ;;
	  esac
	  if [ "$$MA" -gt "$(MAX_MAJOR)" ]; then
	    echo "✗ v$$MA requires semantic import versioning (/v$$MA in module paths)."
	    echo "  Not supported yet — stay on v0/v1."
	    exit 1
	  fi
	  new="$$MA.$$MI.$$PA"
	  mapfile -t TAGS < <(tags_for "$$new")
	  echo
	  echo "Release v$$new — will pin sqld-gen-bob to root v$$new, commit, then create $${#TAGS[@]} tags:"
	  printf '  %s\n' "$${TAGS[@]}"
	  read -r -p "Type 'yes' to proceed: " ok
	  [ "$$ok" = "yes" ] || { echo "Aborted."; exit 0; }
	  # Pin the nested module to the released root version so
	  # `go install .../cmd/sqld-gen-bob@v$$new` fetches a matching root, and keep
	  # the go.work dev replace in lockstep (it names the same version).
	  go mod edit -require=$(ROOT_MODULE)@v$$new cmd/sqld-gen-bob/go.mod
	  sed -i -E "s#^(replace $(ROOT_MODULE)) v[0-9]+\.[0-9]+\.[0-9]+ => \./#\1 v$$new => ./#" go.work
	  git add cmd/sqld-gen-bob/go.mod go.work
	  git commit -m "release v$$new"
	  for t in "$${TAGS[@]}"; do git tag -a "$$t" -m "$$t"; done
	  git push origin HEAD
	  git push origin "$${TAGS[@]}"
	  echo "✓ Released v$$new ($${#TAGS[@]} tags). CI will build dist/ and attach binaries."
	  ;;
	*)
	  echo "Cancelled."
	  ;;
	esac

# --- examples + codegen -----------------------------------------------------
example: build ## Generate code + dump IR for example/ (core: sqld-gen-go)
	./bin/sqld generate -c example/sqld.yaml
	./bin/sqld collect -c example/sqld.yaml --format json > example/gen/catalog.json
	go build ./example/...

example-wasm: build build-wasm ## Generate the example via the WASM plugin
	./bin/sqld generate -c example/sqld.wasm.yaml
	go build ./example/...

example-bob: build build-bob ## Generate the bob ORM (cmd/sqld-gen-bob module) + build it
	./bin/sqld generate -c cmd/sqld-gen-bob/example/sqld.yaml
	go build ./cmd/sqld-gen-bob/...

protocols: # Generate Go + TS code from proto
	cd proto && easyp mod update && easyp mod vendor
	rm -rf $(CURDIR)/pkg/proto
	cd proto && easyp generate
