.PHONY: build install test vet clean release release-dry

# Build the binary into ./sesh with commit SHA embedded.
build:
	go build -o sesh ./cmd/sesh

# Install to $GOPATH/bin (commit SHA embedded automatically via runtime/debug).
install:
	go install ./cmd/sesh

# Run all tests.
test:
	go test ./...

# Run go vet across all packages.
vet:
	go vet ./...

# Remove build artifacts.
clean:
	rm -f sesh
	go clean

# Interactive release: prompts for version bump type, tags, and pushes.
# GoReleaser + CI handle the rest.
release:
	@CURRENT=$$(git tag --sort=-v:refname | head -1 | sed 's/^v//'); \
	if [ -z "$$CURRENT" ]; then \
		echo "No existing tags found."; \
		exit 1; \
	fi; \
	MAJOR=$$(echo $$CURRENT | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | cut -d. -f2); \
	PATCH=$$(echo $$CURRENT | cut -d. -f3); \
	NEXT_PATCH="$$MAJOR.$$MINOR.$$((PATCH + 1))"; \
	NEXT_MINOR="$$MAJOR.$$((MINOR + 1)).0"; \
	NEXT_MAJOR="$$((MAJOR + 1)).0.0"; \
	echo "Current version: v$$CURRENT"; \
	echo ""; \
	echo "  1) patch  → v$$NEXT_PATCH"; \
	echo "  2) minor  → v$$NEXT_MINOR"; \
	echo "  3) major  → v$$NEXT_MAJOR"; \
	echo "  4) custom"; \
	echo ""; \
	printf "Choice [1]: "; \
	read CHOICE; \
	CHOICE=$${CHOICE:-1}; \
	case $$CHOICE in \
		1) VERSION=$$NEXT_PATCH ;; \
		2) VERSION=$$NEXT_MINOR ;; \
		3) VERSION=$$NEXT_MAJOR ;; \
		4) printf "Version (without v prefix): "; read VERSION ;; \
		*) echo "Invalid choice"; exit 1 ;; \
	esac; \
	echo ""; \
	echo "Releasing v$$VERSION"; \
	echo ""; \
	git tag "v$$VERSION" && git push && git push origin "v$$VERSION"; \
	echo ""; \
	echo "Tagged and pushed v$$VERSION. GoReleaser will handle the rest."

# Dry-run GoReleaser locally (no publish).
release-dry:
	goreleaser release --snapshot --clean
