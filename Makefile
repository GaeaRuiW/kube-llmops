CHART_DIR := charts/kube-llmops-stack

.PHONY: lint test template build package e2e clean help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

lint: lint-helm lint-yaml lint-shell lint-markdown ## Run all linters

lint-helm: ## Lint Helm charts
	helm lint $(CHART_DIR)

lint-yaml: ## Lint YAML files
	yamllint -c .yamllint.yml $(CHART_DIR)/values*.yaml alerting/ otel/ || true

lint-shell: ## Lint shell scripts
	shellcheck scripts/*.sh || true

lint-markdown: ## Lint markdown files
	markdownlint '*.md' 'docs/**/*.md' || true

lint-python: ## Lint Python code
	ruff check images/ || true

test: test-template test-python ## Run all tests

test-template: ## Test Helm template rendering
	helm template kube-llmops $(CHART_DIR) > /dev/null

test-python: ## Run Python unit tests
	cd images/model-resolver && python -m pytest tests/ -v || true

template: ## Render Helm templates
	helm template kube-llmops $(CHART_DIR)

template-minimal: ## Render with minimal values
	helm template kube-llmops $(CHART_DIR) -f $(CHART_DIR)/values-minimal.yaml

build: ## Build Docker images locally
	docker build -t kube-llmops/model-loader:dev images/model-loader/
	docker build -t kube-llmops/model-resolver:dev images/model-resolver/

package: ## Package Helm chart
	helm package $(CHART_DIR)

e2e: ## Run E2E tests on kind
	./scripts/e2e-test.sh

clean: ## Clean build artifacts
	rm -f *.tgz
	rm -rf dist/ build/

release: ## Prepare a release (usage: make release VERSION=0.1.0)
	@test -n "$(VERSION)" || (echo "Usage: make release VERSION=x.y.z" && exit 1)
	sed -i 's/^version:.*/version: $(VERSION)/' $(CHART_DIR)/Chart.yaml
	sed -i 's/^appVersion:.*/appVersion: "$(VERSION)"/' $(CHART_DIR)/Chart.yaml
