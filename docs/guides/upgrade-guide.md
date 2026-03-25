# kube-llmops Upgrade Guide

## Generic Upgrade Flow

1. **Backup databases** (if running production workloads)
   ```bash
   kubectl create job pg-backup-pre-upgrade --from=cronjob/kube-llmops-pg-backup
   ```

2. **Review CHANGELOG.md** for breaking changes

3. **Run upgrade with quality gate**
   ```bash
   helm upgrade kube-llmops charts/kube-llmops-stack -f values-single-node.yaml
   ```
   The quality gate pre-upgrade hook checks Ragas metrics before allowing upgrade.

4. **Verify**
   ```bash
   kubectl get pods -l app.kubernetes.io/part-of=kube-llmops --watch
   uv run tests/e2e/test_dify_rag_e2e.py
   ```

## Rollback

```bash
helm rollback kube-llmops <REVISION>
helm history kube-llmops  # to see revision numbers
```

## Version-Specific Notes

### v0.1 → v0.2 (PostgreSQL Split)
- New: `operator-pg` and `app-pg` replace single `litellm-pg`
- Migration: data migration script in `docs/guides/postgresql-migration.md`
- Breaking: `litellm.postgresql.*` values moved to `postgresql.operatorPg.*`

### v0.2 → v0.3 (Enterprise Features)
- New: Milvus, LightRAG, Presidio, multi-tenant namespaces
- No breaking changes (all new components default disabled)
