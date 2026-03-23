# Speedtest Next

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=0B1220)
![TypeScript](https://img.shields.io/badge/TypeScript-5.9-3178C6?logo=typescript&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-ready-2496ED?logo=docker&logoColor=white)
![IPv4/IPv6](https://img.shields.io/badge/Network-IPv4%20%2F%20IPv6-0F766E)
![Theme](https://img.shields.io/badge/UI-Light%20%2F%20Dark-7C3AED)
[![GitHub](https://img.shields.io/badge/GitHub-taurusxin%2Fspeedtest--next-181717?logo=github&logoColor=white)](https://github.com/taurusxin/speedtest-next)

一个基于 Go + React 的在线网络测速平台，支持 IPv4 / IPv6 双栈切换、延迟与抖动测试、下载/上传测速，以及前端曲线展示。

项目地址：[github.com/taurusxin/speedtest-next](https://github.com/taurusxin/speedtest-next)

![Speedtest Next Demo](./assets/demo.png)

## 特性

- Go 单服务提供测速 API 与前端静态资源
- React + Mantine 前端，自动跟随浏览器亮色 / 暗色主题
- IPv4 / IPv6 双栈测速切换
- 延迟、抖动、下载、上传分阶段测速
- 下载 / 上传分离图表展示
- 前端资源嵌入 Go 二进制，部署简单
- 运行时环境变量可动态控制测速目标和测速参数

## 部署方式

### 方式一：systemd 守护进程

适合直接使用 GitHub Release 里的二进制，在服务器上通过 systemd 托管。

1. 从 GitHub Releases 下载对应平台的二进制包并解压
2. 上传到服务器，例如：

```bash
sudo mkdir -p /opt/speedtest-next
sudo cp speedtest-next /opt/speedtest-next/
sudo chmod +x /opt/speedtest-next/speedtest-next
```

3. 准备环境变量文件：

```bash
sudo mkdir -p /etc/speedtest-next
sudo cp deploy/systemd/speedtest-next.env.example /etc/speedtest-next/speedtest-next.env
sudo nano /etc/speedtest-next/speedtest-next.env
```

至少填写：

```bash
SPEEDTEST_TARGET_IPV4=speedtest-v4only.taurusxin.com
SPEEDTEST_TARGET_IPV6=speedtest-v6only.taurusxin.com
```

4. 安装并启动服务：

```bash
sudo cp deploy/systemd/speedtest-next.service /etc/systemd/system/speedtest-next.service
sudo systemctl daemon-reload
sudo systemctl enable --now speedtest-next
```

5. 查看运行状态：

```bash
sudo systemctl status speedtest-next
sudo journalctl -u speedtest-next -f
```

相关文件：

- 服务文件：[deploy/systemd/speedtest-next.service](./deploy/systemd/speedtest-next.service)
- 环境变量模板：[deploy/systemd/speedtest-next.env.example](./deploy/systemd/speedtest-next.env.example)

### 方式二：Docker Compose

适合直接使用 Docker Hub 自动发布的镜像。

1. 复制环境变量模板：

```bash
cp .env.example .env
```

2. 编辑 `.env`，至少填写：

```bash
SPEEDTEST_TARGET_IPV4=speedtest-v4only.example.com
SPEEDTEST_TARGET_IPV6=speedtest-v6only.example.com
```

如果想指定某个已发布版本，可以设置：

```bash
IMAGE_TAG=v1.0.0
```

不设置时默认使用：

```bash
IMAGE_TAG=latest
```

3. 启动服务：

```bash
docker compose up -d
```

4. 查看日志：

```bash
docker compose logs -f
```

5. 停止服务：

```bash
docker compose down
```

相关文件：

- Compose 文件：[compose.yaml](./compose.yaml)
- 环境变量模板：[.env.example](./.env.example)

## 运行时环境变量

支持以下环境变量：

| 变量名 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `SPEEDTEST_TARGET_IPV4` | 是 | - | IPv4 测速目标域名或完整地址 |
| `SPEEDTEST_TARGET_IPV6` | 是 | - | IPv6 测速目标域名或完整地址 |
| `SPEEDTEST_ADDR` | 否 | `:8080` | 服务监听地址 |
| `SPEEDTEST_STATIC_DIR` | 否 | - | 指定外部静态目录；设置后覆盖嵌入资源 |
| `SPEEDTEST_LOG_NOISY_API` | 否 | `false` | 是否输出高频测速接口访问日志 |
| `SPEEDTEST_ALLOWED_ORIGINS` | 否 | 空 | 逗号分隔的 CORS 白名单；为空时按请求 `Origin` 动态回显 |
| `SPEEDTEST_SITE_TITLE` | 否 | `SpeedTest Next` | 页面大标题与浏览器标签标题 |
| `SPEEDTEST_GITHUB_URL` | 否 | 项目仓库地址 | 页面右上角 GitHub 链接地址 |
| `SPEEDTEST_LATENCY_SAMPLE_COUNT` | 否 | `10` | 延迟测试采样次数，越多结果越稳定但等待更久 |
| `SPEEDTEST_LATENCY_SAMPLE_GAP_MS` | 否 | `160` | 延迟采样间隔（毫秒），越大抖动观察更充分 |
| `SPEEDTEST_DOWNLOAD_CONCURRENCY` | 否 | `6` | 下载测试并发线程数，越大越容易跑满带宽 |
| `SPEEDTEST_DOWNLOAD_DURATION_MS` | 否 | `9000` | 下载测试持续时间（毫秒） |
| `SPEEDTEST_DOWNLOAD_CHUNK_BYTES` | 否 | `6291456` | 单次下载数据块大小（约 6 MiB） |
| `SPEEDTEST_UPLOAD_CONCURRENCY` | 否 | `4` | 上传测试并发线程数 |
| `SPEEDTEST_UPLOAD_DURATION_MS` | 否 | `7000` | 上传测试持续时间（毫秒） |
| `SPEEDTEST_UPLOAD_CHUNK_BYTES` | 否 | `1048576` | 单次上传数据块大小（约 1 MiB） |
| `SPEEDTEST_SAMPLING_INTERVAL_MS` | 否 | `250` | 前端瞬时速度采样间隔（毫秒） |
| `SPEEDTEST_CHART_POINTS_LIMIT` | 否 | `120` | 图表最大采样点数 |
| `SPEEDTEST_DISPLAY_SMOOTHING_FACTOR` | 否 | `0.35` | 展示平滑系数，越大越实时，越小越平滑 |

说明：

- `SPEEDTEST_TARGET_IPV4` 和 `SPEEDTEST_TARGET_IPV6` 缺失时，服务不会启动
- 其它测速参数均为可选，不传时使用内置默认值
- 前端启动后会从后端 `/api/v1/runtime-config` 读取这些配置，因此即使前端是静态资源，也能在运行时动态生效

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
export SPEEDTEST_TARGET_IPV4=speedtest-v4only.taurusxin.com
export SPEEDTEST_TARGET_IPV6=speedtest-v6only.taurusxin.com
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

## 项目结构

```text
.
├── .github/workflows/release.yml   # GitHub Actions 自动发布
├── main.go                         # Go 服务入口，测速 API、静态资源托管、CORS、日志
├── main_test.go                    # Go 服务基础测试
├── Dockerfile                      # Docker 多阶段构建
├── compose.yaml                    # Docker Compose 部署配置
├── deploy/systemd/                 # systemd 服务配置
├── web/                            # React 前端
│   ├── src/
│   │   ├── App.tsx                 # 页面与交互
│   │   ├── App.css                 # 页面样式
│   │   ├── config.ts               # 默认配置与运行时配置加载
│   │   ├── main.tsx                # Mantine 主题与前端入口
│   │   ├── speedtest.ts            # 前端测速逻辑
│   │   └── index.css               # 全局样式与亮暗主题变量
│   └── package.json
└── docs/ARCHITECTURE.md            # 架构与测速逻辑说明
```

## 说明

- 浏览器无法在同一主机名下强制指定 IPv4 或 IPv6，所以双栈切换依赖不同目标域名
- 曲线与即时速度使用展示层平滑，最终测速结果仍然按累计字节数和总时长计算
- 页面标题、GitHub 链接、测速目标与测速参数都可以通过运行时环境变量下发

更详细的架构和请求流说明见 [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)。
