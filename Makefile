.PHONY: generate-all
generate-all:
	@echo "Running go generate in all modules..."
	@find . -name "go.mod" -type f | while read -r modfile; do \
		dir=$$(dirname "$$modfile"); \
		echo "Generating in $$dir..."; \
		(cd "$$dir" && go generate ./...); \
	done

.PHONY: generate-readmes
generate-readmes:
	@echo "Generating READMEs for all tools with doc.go files..."
	@find . -name "doc.go" -type f | grep -E "/(cmd/[^/]+|[^/]+)/doc.go$$" | while read -r docfile; do \
		dir=$$(dirname "$$docfile"); \
		if grep -q "//go:generate.*gocmddoc" "$$docfile" 2>/dev/null; then \
			echo "Generating README in $$dir..."; \
			(cd "$$dir" && go generate -run gocmddoc); \
		fi; \
	done

.PHONY: list-tools
list-tools:
	@echo "Tools with documentation:"
	@find . -name "doc.go" -type f | grep -v node_modules | sort

.PHONY: check-missing-docs
check-missing-docs:
	@echo "Main packages without doc.go files:"
	@for mainfile in $$(find . -name "main.go" -type f | grep -v node_modules | grep -v "/testdata/" | sort); do \
		dir=$$(dirname "$$mainfile"); \
		if [ ! -f "$$dir/doc.go" ]; then \
			echo "  $$dir"; \
		fi; \
	done