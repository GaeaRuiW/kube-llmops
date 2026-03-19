CHART_DIR := charts/kube-llmops-stack
CHART_NAME := kube-llmops-stack
VERSION ?= $(shell grep '^version:' $(CHART_DIR)/Chart.yaml | awk '{print $$2}')

.PHONY: lint test template build package e2e clean help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

lint: ## Run all linters (helm, yaml, shell, markdown)
	helm lint $(CHART_DIR)
	@if command -v ct > /dev/null 2>&1; then ct lint --config ct.yaml --charts $(CHART_DIR); fi
	@if command -v yamllint > /dev/null 2>&1; then yamllint -c .yamllint.yml $(CHART_DIR)/values.yaml $(CHART_DIR)/values-*.yaml 2>/dev/null || true; fi
	@if command -v shellcheck > /dev/null 2>&1; then find scripts -name '*.sh' -exec shellcheck {} + 2>/dev/null || true; fi
	@if command -v markdownlint > /dev/null 2>&1; then markdownlint '**/*.md' --ignore node_modules 2>/dev/null || true; fi

test: ## Run tests (helm template, python unit tests)
	helm template test-release $(CHART_DIR) > /dev/null
	@if [ -f $(CHART_DIR)/values-minimal.yaml ]; then helm template test-release $(CHART_DIR) -f $(CHART_DIR)/values-minimal.yaml > /dev/null; fi
	@if [ -f $(CHART_DIR)/values-ci.yaml ]; then helm template test-release $(CHART_DIR) -f $(CHART_DIR)/values-ci.yaml > /dev/null; fi
	@if command -v pytest > /dev/null 2>&1 && [ -d images/model-resolver/tests ]; then pytest images/model-resolver/tests/ -v; fi

template: ## Render Helm templates to stdout
	helm template test-release $(CHART_DIR)

build: ## Build Docker images (local, no push)
	@for img_dir in images/*/; do \
		if [ -f "$$img_dir/Dockerfile" ]; then \
			img_name=$$(basename $$img_dir); \
			echo "Building $$img_name..."; \
			docker build -t kube-llmops/$$img_name:dev $$img_dir; \
		fi; \
	done

package: ## Package Helm chart
	helm package $(CHART_DIR) -d dist/

e2e: ## Run E2E tests on kind cluster
	@echo "Creating kind cluster..."
	kind create cluster --name kube-llmops-e2e 2>/dev/null || true
	helm install kube-llmops $(CHART_DIR) -f $(CHART_DIR)/values-ci.yaml --wait --timeout 10m
	@echo "Running smoke tests..."
	scripts/health-check.sh || true
	helm uninstall kube-llmops
	kind delete cluster --name kube-llmops-e2e

clean: ## Clean build artifacts
	rm -rf dist/ build/ tmp/
	find . -name '*.tgz' -not -path './.git/*' -delete

release: ## Prepare a release (update chart version)
ifndef VERSION
	$(error VERSION is not set. Usage: make release VERSION=0.1.0)
endif
	@sed -i 's/^version:.*/version: $(VERSION)/' $(CHART_DIR)/Chart.yaml
	@sed -i 's/^appVersion:.*/appVersion: "$(VERSION)"/' $(CHART_DIR)/Chart.yaml
	@echo "Updated Chart.yaml to version $(VERSION)"
