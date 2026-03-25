RELEASE_NAME ?= kube-llmops
CHART_DIR     ?= .
VALUES_FILE   ?= values-single-node.yaml
NAMESPACE     ?= default
PROFILES      := single-node multi-gpu ha

.PHONY: dev dep-update lint test-infra bench screenshots

## ── Development ──────────────────────────────────────────────────
dev:
	helm upgrade --install $(RELEASE_NAME) $(CHART_DIR) \
		-f $(VALUES_FILE) \
		-n $(NAMESPACE) --create-namespace \
		--no-hooks

dep-update:
	rm -f charts/*.tgz Chart.lock
	helm dependency update $(CHART_DIR)

## ── Quality ─────────────────────────────────────────────────────
lint:
	helm lint $(CHART_DIR) -f $(VALUES_FILE)
	@for p in $(PROFILES); do \
		echo "--- template: values-$$p.yaml ---"; \
		helm template $(RELEASE_NAME) $(CHART_DIR) -f values-$$p.yaml > /dev/null || exit 1; \
	done
	@echo "All profiles passed."

## ── Testing ─────────────────────────────────────────────────────
test-infra:
	cd tests/e2e && npx playwright test

bench:
	@echo "Usage: python tests/load/llm-inference.py --concurrency 8 --requests 200"
	@echo "       python tests/load/embedding.py      --concurrency 8 --requests 200"
	@echo "       python tests/load/rag-e2e.py        --concurrency 4 --requests 100"

## ── Docs ────────────────────────────────────────────────────────
screenshots:
	./scripts/capture-screenshots.sh
