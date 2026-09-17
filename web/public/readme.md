# 前端构建说明

本 fork 使用 [mghts/komari-web](https://github.com/mghts/komari-web)，开发分支为 `komari-1.4.3`。实际构建以 Server 的 `build/frontend.json` 中完整提交为准，不能使用上游或 fork 默认分支的最新提交代替。

推荐从 Server 根目录使用 `Dockerfile` 构建，它会获取锁定源码，以 Node.js 22、`npm ci` 构建前端并嵌入 Server。完整命令与发布步骤见 [FORK.md](../../FORK.md)。

手工调试时，同样检出锁定提交，执行 `npm ci --no-audit --no-fund`、`npm run check:fork` 和 `npm run build`。将产物 `dist` 和 `komari-theme.json` 放入此目录中的 `defaultTheme`；覆盖旧产物前将其移动到独立备份位置，避免旧 chunk 混入。Server 编译前需要 `defaultTheme/dist/index.html`。

前端修改必须先提交并推送，再更新 Server 锁定文件；升级 Agent 时同时核对 `build/agent.json` 和前端 `AGENT_VERSION`。默认主题随 Server 镜像升级，不通过上游主题 Release 替换。
