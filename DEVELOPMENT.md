# Speedtest Next Development Guide

## 本地开发

环境要求：

- Go 1.26+
- Node.js 20+
- `pnpm`

依赖管理约定：

- Node.js 项目使用 `pnpm`
- 如果后续引入 Python 辅助脚本，使用 `uv`

安装前端依赖：

```bash
cd web
pnpm install
```

启动前端开发环境：

```bash
cd web
pnpm dev
```

启动 Go 服务：

```bash
export SPEEDTEST_TARGET_IPV4=speedtest-v4only.example.com
export SPEEDTEST_TARGET_IPV6=speedtest-v6only.example.com
go run .
```

默认情况下：

- 前端开发服务器运行在 `http://localhost:5173`
- Go 服务运行在 `http://localhost:8080`

## 本地构建

先构建前端：

```bash
cd web
pnpm build
```

再构建 Go 二进制：

```bash
cd ..
go build
```

说明：

- Go 会通过 `embed` 将 `web/dist` 打包进最终二进制
- 如果前端未先构建，最终二进制将不会包含最新页面资源
