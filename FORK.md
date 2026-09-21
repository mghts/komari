# 独立构建、发布和升级

## 版本基线

| 组件 | 上游基线 | 开发分支 |
| --- | --- | --- |
| Server | `1.4.3` / `bf6b45ec3abfc56bba5e9223650a47a72f665371` | `mghts/komari:komari-1.4.3` |
| 前端 | `1.4.3` / `4a74e8a81e2e4b1c3da8ad795f9523151efb6b56` | `mghts/komari-web:komari-1.4.3` |
| Agent | `1.2.60` / `8cd92149a845c12917e42acb1a296c836822758d` | `mghts/komari-agent:komari-agent-1.2.60` |

首个测试发行版命名为 Server `1.4.4-rc.1` 和 Agent `1.2.61-rc.1`。这是本 fork 的递增版本，并不表示采用了上游后续版本的功能。保留原始 tags、许可及作者信息。

`build/frontend.json` 固定前端仓库与完整提交；`build/agent.json` 固定集成测试所用 Agent 镜像。以后修改前端，需要先提交并推送，再更新 Server 的锁定文件。基础镜像使用 digest，语言依赖使用现有 lock 文件；发行产物用 SHA256SUMS 和镜像 digest 标识。

## 开发与验证

```bash
docker build --target test -t komari-tests .
docker build --build-arg VERSION=0.0.0-dev --build-arg REVISION="$(git rev-parse HEAD)" -t komari:test .
python3 scripts/smoke.py komari:test 0.0.0-dev
```

Dockerfile 会自行取回锁定的前端并编译，不需要手工复制 dist。集成测试创建独立 Docker 网络、随机测试账号和临时数据目录，验证首次安装、登录、前端 HTML、版本信息、Agent 指标、远程测试命令、Agent 重启、Server 容器替换及节点持久化。测试结束停止容器并保留数据、备份和网络以供检查，不触碰生产数据。

可用 `KOMARI_SMOKE_AGENT_IMAGE=本地镜像` 测试尚未发布的 Agent。指标精确性、网页终端交互和实际 VPS 的多磁盘场景仍需部署验收，容器测试不等同于生产验收。

## 发布

1. 默认分支 CI 通过后，在 GitHub Actions 选择 **Publish release**，输入一个未使用的版本。
2. 工作流在原生 amd64/arm64 runner 上重新测试、构建并导出产物。
3. 只有两种架构全部成功，才推送这批已测试的镜像、二进制文件和校验和。
4. Release 附带 `build-manifest.json`，记录基线、源码提交、前端提交、Agent 组合和镜像 digest。
5. `-rc.N` 等带连字符版本是 Pre-release，不更新 `latest`。生产采用经过 VPS 验收的明确版本或 digest。

Server 镜像：`ghcr.io/mghts/komari:<version>`。Agent 镜像：`ghcr.io/mghts/komari-agent:<version>`。首次发布后必须确认 GitHub Packages 的包可见性为 public，并验证匿名拉取。GitHub 仓库 public 不等于镜像自动 public。

不覆盖已发布版本；失败的发布若已推送部分镜像，应检查状态并使用新的版本。旧工作流已移至 `.github/legacy-workflows` 保存，不自动合并、不直连 VPS，也不清理历史镜像。

## Server 的 Docker 部署

将 `deploy/compose.yaml` 与 `deploy/env.example` 放到部署目录，将后者复制为 `.env`。核对数据目录、原有反向代理和端口设置。

```bash
docker compose config --quiet
docker compose pull
docker compose up -d
docker compose ps
docker compose logs --tail=100
```

默认只监听 `127.0.0.1:25774`，供同机反向代理访问；需要直接访问时按原有部署设置 `KOMARI_BIND`。迁移已有 Server 时，`KOMARI_DATA_DIR` 必须指向原容器 `/app/data` 对应的真实目录，不能误建空目录。先停止原 Server，避免两份实例同时打开同一个 SQLite 数据目录。

### 升级和回退

1. 保存旧镜像版本/digest、Compose、`.env`、反向代理配置。
2. 停止 Server，对完整数据目录做一致性备份，包括主库、指标库、插件和主题。若配置了外部数据库，还要单独备份对应数据库。
3. 修改 `.env` 的 `KOMARI_VERSION`，执行 `docker compose pull` 和 `docker compose up -d`。
4. 检查登录、节点上线、历史指标、主题、通知和日志；确认正常后再迁移其他节点。

仅运行 `docker compose restart` 不会换用新镜像。挂载的数据目录会保留，但容器保留数据不等于数据库迁移可逆。需要恢复备份时，先停止 Server，把失败后的数据目录移到独立保留位置，再将备份恢复到原位置，使用旧镜像启动；不得直接覆盖仍在使用的数据。

## Agent 迁移

Agent 发布说明、安装器和 Compose 见 [mghts/komari-agent](https://github.com/mghts/komari-agent/blob/komari-agent-1.2.60/FORK.md)。现有 VPS 上的 `1.5.10` 不会因 fork 自动切换；首次切换到本 fork 是一次显式降级。

先各选一台 amd64、arm64 VPS 验收，保留原节点 token。systemd 和 Docker Agent 不能同时用同一个 token 连接。容器模式通过镜像升级且禁用程序自更新；默认关闭远程控制，宿主机网页终端需求继续用 systemd 方式。

## 首版范围

此阶段不修改监控业务、数据库结构或页面设计，只建立独立构建、发布和升级流程。前端的安装来源和版本提示已切换到本 fork。实际生产迁移需使用你自己的数据目录、域名、token 和现有服务设置。

## 安装与资源来源核对

- Server/Agent 镜像和二进制来自 `mghts` 的明确版本；RC 不使用 `latest` 或旧 Snapshot 通道。
- `install-komari.sh` 的 systemd 安装要求输入明确版本，仅支持 Linux amd64/arm64；下载二进制和 `SHA256SUMS` 并验证后才停止旧服务，保留旧二进制。二进制回退不撤销数据库迁移，升级前仍需一致性数据备份。Docker 部署继续按本文步骤操作。
- 导航、帮助、关于页面和默认主题信息指向 fork。Go module/import、许可证作者、上游基线和历史工作流保留原信息。
- 主题市场 `komari-monitor/theme-market` 是独立的公共目录，继续使用；它不是 Server/Agent 的安装或自动更新源。插件市场已移除，第三方主题及已有数据库中的自定义主题来源保留。
- 前端 CI 的 `npm run check:fork` 检查运行代码中的上游地址和安装命令。Server 构建额外核对前端 `AGENT_VERSION` 与 `build/agent.json` 一致；`python3 scripts/check_fork.py` 检查 Server 活跃入口。

## 插件与 JavaScript 通知移除

本 fork 自 Server `1.4.6` 起移除插件系统、插件市场、插件页面和 `Javascript` 通知发送器，以及仅供它们使用的 JavaScript 运行时。主题系统、其他通知渠道、通知模板、内置流量报告、网页终端、远程命令、网络探测与指标存储保留。

- 旧插件 REST 接口返回 404，插件 RPC 方法不再注册，分片上传不再接受 `plugin` 类型。磁盘上的旧插件不会被加载或执行。
- 旧插件目录、数据表、市场配置和通知脚本不自动删除；备份继续保留历史插件数据，便于回退。新安装不再创建插件目录或插件配置表。
- 原通知渠道若为 `Javascript`，该渠道将停止发送通知，后台通知渠道页会提示重新选择受支持的渠道；日志记录不可用状态。原渠道选择及脚本保留，普通通知模板与其他渠道不变。
- 升级前按前文备份完整数据目录；回退使用原版本镜像及升级前的一致性备份。本次不执行删除旧表或旧字段的数据迁移。

## Agent 安装修复与自动更新选项

Server `1.4.7` 的内置前端使用 Agent `1.2.62`。Linux 一键安装默认不勾选“禁用自动更新”，生成命令显式带上 `--disable-auto-update=false`；Docker 仍通过更换镜像升级。现有节点的配置不会因面板升级自动改变。

Agent 安装器修正 systemd 服务路径的转义；升级时会备份并修复 `1.2.61` 安装器生成的标准错误 unit，保留配置和节点 token。首次失败后可重试相同节点命令；不同凭据或自定义服务不会被覆盖。已有失败安装也可下载 `1.2.62` Release 的 `install.sh`，执行 `sudo bash install.sh --install-version 1.2.62`，自定义路径和服务名需继续传入对应选项。

安装器发布检查包含两个架构上的真实 systemd 安装、重复执行、损坏下载、失败升级回滚及旧配置修复。发布成功后仍需按前文升级实际 Server 容器，并在 VPS 上确认服务运行和节点上线。
