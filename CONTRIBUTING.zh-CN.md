[English](CONTRIBUTING.md) | **中文**

# 贡献指南

感谢你对本项目的关注！本指南将帮助你快速上手。

## 开发环境搭建

### 前置条件

- [Helm 3.x](https://helm.sh/docs/intro/install/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [kind](https://kind.sigs.k8s.io/)（用于本地测试）
- [yamllint](https://github.com/adrienverber/yamllint)（`pip install yamllint`）
- [chart-testing](https://github.com/helm/chart-testing)（可选但推荐）
- Python 3.11+（用于 model-resolver 开发）

### 快速开始

```bash
git clone https://github.com/GaeaRuiW/kube-llmops.git
cd kube-llmops

# 验证环境
make lint
make test
```

## 进行修改

### Helm Charts

所有 Helm chart 位于 `charts/kube-llmops-stack/` 目录下。各组件的子 chart 位于 `charts/kube-llmops-stack/charts/` 目录下。

```bash
# 对你的修改进行 lint 检查
make lint

# 渲染模板以检查输出
make template

# 使用指定的 values 配置进行测试
helm template test charts/kube-llmops-stack/ -f charts/kube-llmops-stack/values-minimal.yaml
```

### values.yaml 约定

`values.yaml` 中的顶层键是面向用户的 API，请将其视为稳定接口：

- **新增**键是允许的（非破坏性变更）
- **重命名/移除**键属于破坏性变更——需要在上一版本中发布弃用通知
- 为每个键添加注释说明

### Docker 镜像

镜像源码位于 `images/` 目录。每个镜像都有独立的 Dockerfile。

```bash
# 构建所有镜像
make build

# 构建单个镜像
docker build -t kube-llmops/model-resolver:dev images/model-resolver/
```

### Grafana 仪表盘

仪表盘 JSON 文件位于 `dashboards/` 目录。编辑时请注意：

- 使用描述性的 `title`
- 将数据源设置为 `${DS_PROMETHEUS}`（使用变量，不要硬编码）
- 通过导入到运行中的 Grafana 实例进行测试

## Pull Request 流程

1. Fork 本仓库并从 `main` 分支创建新分支
2. 进行修改
3. 确保 `make lint` 和 `make test` 通过
4. 编写清晰的提交信息（我们遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范）
5. 向 `main` 分支发起 PR

### 提交信息格式

```
feat(vllm): add support for tensor parallelism configuration
fix(litellm): correct PostgreSQL connection string template
docs: update quick start guide for v0.2.0
chore(ci): add Python lint step to lint workflow
```

### PR 标签

| 标签 | 含义 |
|---|---|
| `bug` | 存在问题需要修复 |
| `feature` | 新功能请求或实现 |
| `docs` | 仅文档变更 |
| `good-first-issue` | 适合新手参与 |
| `help-wanted` | 需要额外关注 |
| `breaking-change` | 破坏向后兼容性的变更 |

## 项目结构

```
charts/kube-llmops-stack/   # Umbrella Helm chart（核心交付物）
  charts/                   # 各组件的子 chart
  templates/                # 共享模板
  values.yaml               # 默认配置值
  values-*.yaml             # 配置覆盖文件
dashboards/                 # Grafana 仪表盘 JSON 文件
alerting/                   # Prometheus 告警规则
otel/                       # OpenTelemetry Collector 配置
images/                     # Docker 镜像源码
  model-resolver/           # 引擎自动选择逻辑
  model-loader/             # 模型权重下载器
scripts/                    # 自动化脚本
docs/                       # 文档
examples/                   # 使用示例
```

## 有问题？

- 在 [GitHub Discussions](https://github.com/GaeaRuiW/kube-llmops/discussions) 中提问
- 在 [Issues](https://github.com/GaeaRuiW/kube-llmops/issues) 中报告 bug 或提交功能请求
