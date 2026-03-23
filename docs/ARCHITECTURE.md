# Speedtest Next 架构与逻辑

## 1. 整体架构

项目由两个逻辑层组成：

- 前端：React + Mantine + Recharts
- 后端：Go HTTP 服务

运行时仍然是一个整体服务：

- Go 提供测速 API
- Go 提供前端静态资源
- 前端构建产物默认嵌入到 Go 二进制

整体请求关系如下：

```text
Browser
  ├── 访问页面 -> speedtest.example.com
  ├── 获取 IPv4 地址 -> v4 域名 /api/v1/ip
  ├── 获取 IPv6 地址 -> v6 域名 /api/v1/ip
  ├── 选择 IPv4 后测速 -> v4 域名 /api/v1/*
  └── 选择 IPv6 后测速 -> v6 域名 /api/v1/*

Go Service
  ├── /api/v1/health
  ├── /api/v1/ip
  ├── /api/v1/latency
  ├── /api/v1/download
  ├── /api/v1/upload
  └── SPA static files
```

## 2. 双栈设计

浏览器不能直接指定“同一个域名强制走 IPv4 或 IPv6”，所以这里采用双域名方案：

- IPv4 测速目标：`apiTargets.ipv4`
- IPv6 测速目标：`apiTargets.ipv6`

前端只负责切换目标域名，真正的协议栈选择由 DNS 解析和访问目标决定。

如果配置里只写域名、不带协议，例如：

```ts
apiTargets: {
  ipv4: 'speedtest-v4only.example.com',
  ipv6: 'speedtest-v6only.example.com',
}
```

前端会自动补成：

- 当前页面是 `http` 时，用 `http://...`
- 当前页面是 `https` 时，用 `https://...`

## 3. 后端职责

[main.go](/Users/taurusxin/Workspace/Go/speedtest-next/main.go) 负责以下能力：

### 3.1 API

- `GET /api/v1/health`
  - 用于检测测速节点是否可用

- `GET /api/v1/ip`
  - 返回客户端 IP
  - 优先读取 `X-Forwarded-For`
  - 其次读取 `X-Real-IP`
  - 最后回退到 `RemoteAddr`

- `GET /api/v1/latency`
  - 返回轻量 JSON
  - 前端按请求往返时间计算 RTT
  - 抖动由多次 RTT 差值计算

- `GET /api/v1/download`
  - 按指定大小持续写出字节流
  - 用于下载吞吐测试

- `POST /api/v1/upload`
  - 读取并丢弃请求体
  - 用于上传吞吐测试

### 3.2 静态资源

- 默认从嵌入资源提供前端页面
- 如果设置 `SPEEDTEST_STATIC_DIR`，则优先使用外部目录
- 非 API 请求回退到 `index.html`，保证 SPA 路由可用

### 3.3 横切能力

- CORS 白名单
- 高频测速接口日志降噪
- systemd 部署支持

## 4. 前端职责

前端核心文件：

- [web/src/App.tsx](/Users/taurusxin/Workspace/Go/speedtest-next/web/src/App.tsx)
- [web/src/speedtest.ts](/Users/taurusxin/Workspace/Go/speedtest-next/web/src/speedtest.ts)
- [web/src/config.ts](/Users/taurusxin/Workspace/Go/speedtest-next/web/src/config.ts)

### 4.1 页面层

`App.tsx` 负责：

- 渲染首屏介绍、协议栈切换、开始测速按钮
- 显示延迟 / 抖动 / 下载 / 上传摘要卡
- 显示下载图表和上传图表
- 显示用户 IPv4 / IPv6 地址
- 处理测速状态切换与异常提示

### 4.2 测速引擎

`speedtest.ts` 负责：

- 读取前端配置
- 触发健康检查
- 执行延迟采样
- 执行下载多线程测速
- 执行上传多线程测速
- 回传阶段快照给页面

### 4.3 配置

`config.ts` 定义：

- 双栈目标地址
- 下载 / 上传并发数
- 下载 / 上传块大小
- 测试时长
- 采样间隔
- 图表点数上限
- 展示平滑系数

这些配置都是构建期写死，不提供运行时修改。

## 5. 测速流程

测速流程固定为：

1. 健康检查
2. 延迟 / 抖动测试
3. 下载测速
4. 上传测速
5. 展示结果

详细逻辑：

### 5.1 健康检查

前端向目标栈请求：

```text
GET /api/v1/health
```

如果失败，则直接进入失败态，不继续测速。

### 5.2 延迟与抖动

前端多次请求：

```text
GET /api/v1/latency
```

计算：

- 延迟：所有 RTT 平均值
- 抖动：相邻 RTT 差值绝对值的平均值

### 5.3 下载测速

前端按配置启动多条下载 worker。

每个 worker 都会循环请求：

```text
GET /api/v1/download?bytes=...&chunkSize=...
```

前端按采样间隔统计这一段时间内接收的字节数，并换算为 Mbps。

### 5.4 上传测速

下载结束后，启动上传 worker：

```text
POST /api/v1/upload
```

请求体是前端构造好的二进制块。后端只负责读取并丢弃，用于测上传吞吐。

## 6. 平滑显示逻辑

为了让图表和大数字更稳定，前端对展示层做了平滑处理。

当前策略：

- 原始瞬时速度按采样窗口计算
- 展示值通过 EMA 平滑
- 图表画的是平滑后的点
- 页面实时大数字显示的是平滑后的瞬时值

但最终测速结果不受影响：

- 最终下载速度 = 总下载字节数 / 下载总时长
- 最终上传速度 = 总上传字节数 / 上传总时长

也就是说：

- 图表更平滑
- 结果仍然基于真实累计值

## 7. 图表展示逻辑

当前图表拆成两个独立区域：

- 左侧：下载速度曲线
- 右侧：上传速度曲线

这样做有几个好处：

- 不同阶段不会互相覆盖
- 用户更容易分辨上下行表现
- 在移动端更容易重排

未完成的阶段保持卡片占位，只显示占位文案，不会导致页面突然跳动。

## 8. 主题与适配

前端支持自动亮暗主题切换：

- 读取浏览器 `prefers-color-scheme`
- Mantine 使用 `defaultColorScheme="auto"`
- 页面配色通过 CSS 变量统一切换

移动端适配重点：

- 首屏改为单列布局
- 保持统一的卡片间距和外边距
- 操作按钮避免挤压
- 图表高度收缩但仍保留可读性

## 9. 部署模型

推荐部署方式：

1. `pnpm build`
2. `go build`
3. 上传单个 Go 二进制
4. 用 nginx 做反向代理
5. 用 systemd 托管服务

项目内已有 systemd 配置：

- [deploy/systemd/speedtest-next.service](/Users/taurusxin/Workspace/Go/speedtest-next/deploy/systemd/speedtest-next.service)

## 10. 当前边界

当前版本不包含：

- 用户系统
- 历史测速记录
- 服务端持久化
- 排行榜或分享页
- 自定义运行时测速参数

当前版本重点是：

- 线上测速可用
- 双栈切换明确
- 页面简洁
- 部署简单
