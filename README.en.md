# Speedtest Next

[简体中文](./README.md) | English

[![Docker](https://img.shields.io/badge/Docker-ready-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://hub.docker.com/r/taurusxin/speedtest-next)
[![License](https://img.shields.io/github/license/taurusxin/speedtest-next?style=for-the-badge&logo=opensourceinitiative&logoColor=white&color=3DA639)](https://github.com/taurusxin/speedtest-next/blob/main/LICENSE)
[![Release](https://img.shields.io/github/v/release/taurusxin/speedtest-next?style=for-the-badge&logo=github&logoColor=white&color=181717)](https://github.com/taurusxin/speedtest-next/releases)
[![Stars](https://img.shields.io/github/stars/taurusxin/speedtest-next?style=for-the-badge&logo=github&logoColor=white&color=181717)](https://github.com/taurusxin/speedtest-next/stargazers)

A self-hosted network speed test platform powered by Go and React. It features IPv4/IPv6 dual-stack testing, latency and jitter measurement, download/upload throughput testing, and real-time front-end charting.

![Speedtest Next Demo](./assets/demo.png)

## Features

- A single Go service serving both the speed test API and front-end static assets
- React + Mantine front-end with automatic dark/light mode detection
- IPv4 / IPv6 dual-stack testing switching
- Phased testing: latency, jitter, download, and upload
- Distinct chart visualizations for download and upload phases
- Front-end assets embedded in the Go binary for effortless deployment
- Dynamic configuration of target URLs and test parameters via runtime environment variables

## Deployment

### Method 1: Docker Compose

Ideal for deploying directly using the automated images on Docker Hub. Please refer to the [Compose configuration](./deploy/docker/compose.yaml).

1. Copy the environment variables template:

```bash
cp .env.example .env
```

2. Edit `.env` and fill in at least the following variables:

```bash
SPEEDTEST_TARGET_IPV4=speedtest-v4only.example.com
SPEEDTEST_TARGET_IPV6=speedtest-v6only.example.com
```

If you wish to deploy a specific version, you can set:

```bash
IMAGE_TAG=v1.0.0
```

By default, it uses:

```bash
IMAGE_TAG=latest
```

3. Start the service:

```bash
docker compose up -d
```

4. View the logs:

```bash
docker compose logs -f
```

5. Stop the service:

```bash
docker compose down
```

Related files:

- Compose File: [compose.yaml](./compose.yaml)
- Environment Variables Template: [.env.example](./.env.example)

### Method 2: systemd Daemon

Ideal for running the pre-compiled binary from GitHub Releases on your server, managed by systemd.

1. Download and extract the binary package for your platform from GitHub Releases.
2. Upload it to your server, for example:

```bash
sudo mkdir -p /opt/speedtest-next
sudo cp speedtest-next /opt/speedtest-next/
sudo chmod +x /opt/speedtest-next/speedtest-next
```

3. Prepare the environment variables file:

```bash
sudo mkdir -p /etc/speedtest-next
sudo cp deploy/systemd/speedtest-next.env.example /etc/speedtest-next/speedtest-next.env
sudo nano /etc/speedtest-next/speedtest-next.env
```

Fill in at least the following:

```bash
SPEEDTEST_TARGET_IPV4=speedtest-v4only.example.com
SPEEDTEST_TARGET_IPV6=speedtest-v6only.example.com
```

4. Install and start the service:

```bash
sudo cp deploy/systemd/speedtest-next.service /etc/systemd/system/speedtest-next.service
sudo systemctl daemon-reload
sudo systemctl enable --now speedtest-next
```

5. Check the status:

```bash
sudo systemctl status speedtest-next
sudo journalctl -u speedtest-next -f
```

Related files:

- Service File: [deploy/systemd/speedtest-next.service](./deploy/systemd/speedtest-next.service)
- Environment Variables Template: [deploy/systemd/speedtest-next.env.example](./deploy/systemd/speedtest-next.env.example)

## Runtime Environment Variables

The following environment variables are supported:

| Variable | Required | Default | Description |
| --- | --- | --- | --- |
| `SPEEDTEST_TARGET_IPV4` | Yes | - | IPv4 target domain or full URL |
| `SPEEDTEST_TARGET_IPV6` | Yes | - | IPv6 target domain or full URL |
| `SPEEDTEST_ADDR` | No | `:8080` | Service listening address |
| `SPEEDTEST_STATIC_DIR` | No | - | Custom external static directory; overrides embedded assets if set |
| `SPEEDTEST_LOG_NOISY_API` | No | `false` | Enable access logs for high-frequency test APIs |
| `SPEEDTEST_ALLOWED_ORIGINS` | No | Empty | Comma-separated CORS whitelist; echoes the request `Origin` dynamically if empty |
| `SPEEDTEST_SITE_TITLE` | No | `SpeedTest Next` | Main page title and browser tab title |
| `SPEEDTEST_GITHUB_URL` | No | Repository URL | GitHub link in the top right corner of the page |
| `SPEEDTEST_LATENCY_SAMPLE_COUNT` | No | `10` | Latency test sample count; higher values yield more stable results but take longer |
| `SPEEDTEST_LATENCY_SAMPLE_GAP_MS` | No | `160` | Latency sampling gap (ms); higher values provide better jitter observation |
| `SPEEDTEST_DOWNLOAD_CONCURRENCY` | No | `6` | Download test concurrent threads; higher values saturate bandwidth more easily |
| `SPEEDTEST_DOWNLOAD_DURATION_MS` | No | `9000` | Download test duration (ms) |
| `SPEEDTEST_DOWNLOAD_CHUNK_BYTES` | No | `6291456` | Single download chunk size (approx. 6 MiB) |
| `SPEEDTEST_UPLOAD_CONCURRENCY` | No | `4` | Upload test concurrent threads |
| `SPEEDTEST_UPLOAD_DURATION_MS` | No | `7000` | Upload test duration (ms) |
| `SPEEDTEST_UPLOAD_CHUNK_BYTES` | No | `1048576` | Single upload chunk size (approx. 1 MiB) |
| `SPEEDTEST_SAMPLING_INTERVAL_MS` | No | `250` | Real-time speed sampling interval on the front-end (ms) |
| `SPEEDTEST_CHART_POINTS_LIMIT` | No | `120` | Maximum number of sample points for the chart |
| `SPEEDTEST_DISPLAY_SMOOTHING_FACTOR` | No | `0.35` | Display smoothing factor; larger values are more real-time, smaller values are smoother |

Notes:

- The service will fail to start if both `SPEEDTEST_TARGET_IPV4` and `SPEEDTEST_TARGET_IPV6` are missing.
- All other test parameters are optional and will fallback to built-in defaults if omitted.
- The front-end fetches these configurations from the `/api/v1/runtime-config` endpoint on startup, ensuring they take effect dynamically at runtime even though the front-end is static.

## Development and Building

Please refer to the [Development Guide](./DEVELOPMENT.md) to learn how to develop and build the project locally.

## Project Structure

```text
.
├── .github/workflows/release.yml   # Automated release via GitHub Actions
├── main.go                         # Go service entrypoint, speed test API, static assets, CORS, and logging
├── main_test.go                    # Go service basic tests
├── Dockerfile                      # Docker multi-stage build
├── DEVELOPMENT.md                  # Development and building guide
├── deploy/                         # Deployment configurations
│   ├── docker/                     # Docker Compose configuration
│   │   └── compose.yaml
│   └── systemd/                    # systemd service configuration
├── web/                            # React front-end
│   ├── src/
│   │   ├── App.tsx                 # Page components and interactions
│   │   ├── App.css                 # Page styles
│   │   ├── config.ts               # Default configs and runtime config loading
│   │   ├── main.tsx                # Mantine theme and front-end entrypoint
│   │   ├── speedtest.ts            # Front-end speed testing logic
│   │   └── index.css               # Global styles and dark/light mode variables
│   └── package.json
└── docs/ARCHITECTURE.md            # Architecture and testing logic documentation
```

## Notes

- Browsers cannot strictly force IPv4 or IPv6 under the same hostname, so dual-stack switching relies on different target domains.
- Curves and instantaneous speeds utilize presentation-layer smoothing, while final speed test results are still calculated based on cumulative bytes and total duration.
- The page title, GitHub link, test targets, and test parameters can all be configured dynamically via runtime environment variables.

For a more detailed explanation of the architecture and request flows, please see [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md).