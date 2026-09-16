# Komari Server 协作指南

## 开始工作

- 使用简体中文沟通；代码标识符、命令、日志、配置项和文件名保持原文。
- 先阅读本文件、`FORK.md` 和当前任务涉及的代码，再执行修改。先检查 `git status --short --branch`、`git remote -v` 和已有差异，保留用户改动。
- 询问、方案和审阅任务先回答问题；要求实现或修复时，持续完成已授权工作及必要验证。
- 区分文件证据、测试结果和推断；无法验证的部分明确说明。涉及外部版本、API 等易变信息时核查来源。
- 修改保持集中，沿用项目结构；不要为当前任务顺手重构、升级依赖或更换技术栈。完成后简述改动、验证结果及未验证部分。

## 仓库与源码基线

| 组件 | GitHub 仓库 | 开发分支 | 上游源码基线 |
| --- | --- | --- | --- |
| Server | `mghts/komari` | `komari-1.4.3` | `1.4.3` |
| 前端 | `mghts/komari-web` | `komari-1.4.3` | `1.4.3` |
| Agent | `mghts/komari-agent` | `komari-agent-1.2.60` | `1.2.60` |

- 这是三个独立 Git 仓库。当前本机布局为 Server 在根目录，前端在 `.local/komari-web`，Agent 在 `.local/komari-agent`；进入其他仓库时继续阅读各自的 `AGENTS.md`。
- `.local/` 是本机辅助目录，当前由 Server 的 `.git/info/exclude` 排除；它不会随 Server 克隆或新 worktree 自动出现。需要其他组件时先定位其 checkout，并核对 remote；不要把嵌套仓库、缓存或下载产物加入 Server 提交。
- 源码基线与本 fork 的发行号是两个概念。递增发行号不代表引入了上游后续版本；不要因上游出现新版本而自动切换基线或合并其默认分支。
- Server 基线提交以 `build/baseline.json` 为准，前端源码以 `build/frontend.json` 的完整提交为准，配套 Agent 以 `build/agent.json` 为准。保留上游许可、作者信息和历史 tags。
- Go module 仍为 `github.com/komari-monitor/komari`；GitHub fork 名称不同不构成全局替换 import 路径的理由。

## 开发与验证

- 优先使用 Dockerfile 中的工具链；Server 使用 CGO/SQLite，并在构建时获取、编译锁定的前端。
- 影响前端产物的变更先在前端仓库构建、提交并推送，再更新本仓库的 `build/frontend.json`。单独修改前端分支不会自动改变 Server 镜像。
- 切换配套 Agent 时，更新 `build/agent.json` 中对应的版本、提交和镜像 digest，并运行集成测试。
- 根据改动运行必要检查。仅修改文档时核对内容、路径和 `git diff --check`，不要求重跑完整构建；程序、依赖或构建流程变更使用以下验证：

```bash
docker build --target test -t komari-tests:local .
docker build --build-arg VERSION=0.0.0-dev --build-arg REVISION="$(git rev-parse HEAD)" -t komari:test .
python3 scripts/smoke.py komari:test 0.0.0-dev
```

- `scripts/smoke.py` 使用独立 Docker 网络和临时数据，验证安装、登录、指标、远程测试命令、重启和持久化；可用 `KOMARI_SMOKE_AGENT_IMAGE` 指定本地 Agent 镜像。测试必须与生产数据隔离，保留检查所需的测试数据。
- 涉及协议、鉴权、数据存储或迁移时补充对应验证；数据库迁移需说明兼容性和恢复办法。容器测试不能代替真实 VPS 的指标、多磁盘和网页终端验收。

## 构建、发布与部署

- 发布流程在 `.github/workflows/`，历史工作流在 `.github/legacy-workflows/`；不自动恢复历史合并、发布或清理流程。
- 使用 `Publish release` 工作流和未使用的版本号。Linux `amd64`、`arm64` 必须全部通过检查，再发布同一批已测试产物；带 `-rc.N` 的版本保留为 Pre-release，不更新 `latest`。
- 保持基础镜像 digest、依赖锁文件、`SHA256SUMS` 和 `build-manifest.json` 的追溯能力；不覆盖已发布 tag 或镜像版本。
- 镜像仓库为 `ghcr.io/mghts/komari`。部署模板见 `deploy/`，备份、升级、回退步骤见 `FORK.md`。
- GitHub 发布成功不代表 VPS 已部署。部署范围按当前任务授权执行；升级前核对实际数据挂载并做一致性备份，不能让两份 Server 同时写同一份 SQLite 数据。
- 当前版本、CI 结果和部署状态需重新核对 Git、Release、构建清单及实际环境。本文件保存长期规则，不把对话中的阶段状态当作永久事实。

## 文件与数据保护

- 未经用户逐次明确授权，不执行批量或递归删除、清空目录、删除任何目录或一次删除多个文件；包括通过脚本和工具执行的删除。`rm -rf`、通配符删除和以覆盖、清空、强制重置规避限制的做法同样禁止。
- 确需删除时，先列出确切对象、原因、影响和恢复方式，获得授权后确认路径及范围；优先采用可恢复方式。缓存、生成文件和测试数据也不能仅凭“清理”自行删除。
- 不提交或输出真实 token、密码、`.env`、生产数据库及备份；示例使用占位符，测试使用独立临时凭据。
