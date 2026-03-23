import { useEffect, useRef, useState, type ReactNode } from 'react'
import {
  Alert,
  Badge,
  Button,
  Card,
  Group,
  Progress,
  SegmentedControl,
  SimpleGrid,
  Stack,
  Text,
  ThemeIcon,
  Title,
} from '@mantine/core'
import { IconActivityHeartbeat, IconArrowsDownUp, IconBrandSpeedtest, IconChartLine, IconClockHour4, IconCloudUpload, IconDownload, IconInfoCircle, IconWorld } from '@tabler/icons-react'
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import './App.css'
import { speedtestConfig, type StackMode } from './config'
import { runSpeedtest, type SpeedtestResult, type TestStatus } from './speedtest'

const statusLabels: Record<TestStatus, string> = {
  idle: '准备开始',
  preparing: '连接节点',
  latency: '测试延迟',
  download: '下载测速中',
  upload: '上传测速中',
  completed: '测速完成',
  failed: '测速失败',
}

const statusProgress: Record<TestStatus, number> = {
  idle: 0,
  preparing: 10,
  latency: 28,
  download: 56,
  upload: 84,
  completed: 100,
  failed: 100,
}

function formatSpeed(value: number) {
  if (!Number.isFinite(value)) {
    return '0.00'
  }

  return value.toFixed(value >= 100 ? 1 : 2)
}

function formatMetric(value: number, digits = 1) {
  if (!Number.isFinite(value)) {
    return '--'
  }

  return value.toFixed(digits)
}

async function fetchStackIP(baseUrl: string) {
  try {
    const response = await fetch(`${baseUrl}/api/v1/ip`, {
      cache: 'no-store',
      mode: 'cors',
    })

    if (!response.ok) {
      return '无法获取'
    }

    const payload = (await response.json()) as { ip?: string }
    return payload.ip?.trim() ? payload.ip : '无法获取'
  } catch {
    return '无法获取'
  }
}

function buildPhaseChartData(series: Array<{ timeSeconds: number; mbps: number }>) {
  return series.map((sample) => ({
    second: Number(sample.timeSeconds.toFixed(2)),
    speed: sample.mbps,
  }))
}

function App() {
  const [stack, setStack] = useState<StackMode>('ipv4')
  const [status, setStatus] = useState<TestStatus>('idle')
  const [currentMbps, setCurrentMbps] = useState(0)
  const [statusMessage, setStatusMessage] = useState('等待开始测速')
  const [result, setResult] = useState<Partial<SpeedtestResult> | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [ipInfo, setIpInfo] = useState<{ ipv4: string; ipv6: string }>({
    ipv4: '加载中',
    ipv6: '加载中',
  })
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    let active = true

    void Promise.all([
      fetchStackIP(speedtestConfig.apiTargets.ipv4),
      fetchStackIP(speedtestConfig.apiTargets.ipv6),
    ]).then(([ipv4, ipv6]) => {
      if (!active) {
        return
      }

      setIpInfo({ ipv4, ipv6 })
    })

    return () => {
      active = false
      abortRef.current?.abort()
    }
  }, [])

  async function handleStart() {
    abortRef.current?.abort()
    const controller = new AbortController()
    abortRef.current = controller
    setStatus('preparing')
    setCurrentMbps(0)
    setStatusMessage('检查测速节点可用性')
    setResult(null)
    setError(null)

    try {
      const measured = await runSpeedtest(stack, controller.signal, (event) => {
        setStatus(event.status)
        setCurrentMbps(event.currentMbps ?? 0)
        setStatusMessage(event.message ?? statusLabels[event.status])
        if (event.snapshot) {
          setResult((previous) => ({
            ...(previous ?? {}),
            ...event.snapshot,
          }))
        }
      })

      setResult(measured)
      setCurrentMbps(
        measured.uploadMbps > 0 ? measured.uploadMbps : measured.downloadMbps,
      )
      setStatus('completed')
      setStatusMessage('下载、上传、延迟和抖动测试均已完成')
    } catch (caughtError) {
      if (controller.signal.aborted) {
        return
      }

      const message =
        caughtError instanceof Error ? caughtError.message : '测速过程中发生未知错误'
      setStatus('failed')
      setError(message)
      setStatusMessage(message)
    }
  }

  const activeMetricLabel =
    status === 'upload' ? '当前上传速度' : status === 'download' ? '当前下载速度' : '即时速度'
  const isRunning = ['preparing', 'latency', 'download', 'upload'].includes(status)
  const hasLatencyResult =
    result !== null &&
    typeof result.latencyMs === 'number' &&
    typeof result.jitterMs === 'number'
  const hasDownloadResult =
    result !== null &&
    typeof result.downloadMbps === 'number' &&
    typeof result.downloadPeakMbps === 'number'
  const hasUploadResult =
    result !== null &&
    typeof result.uploadMbps === 'number' &&
    typeof result.uploadPeakMbps === 'number'
  const downloadChartData = buildPhaseChartData(result?.downloadSeries ?? [])
  const uploadChartData = buildPhaseChartData(result?.uploadSeries ?? [])

  return (
    <main className="app-shell">
      <section className="hero-panel">
        <div className="hero-copy">
          <Badge className="hero-badge" size="lg" radius="xl">
            TaurusXin Network Intelligence
          </Badge>
          <Title order={1}>TaurusXin 网络测速平台</Title>
          <Text className="hero-description">
            通过浏览器即可测试您与全球多个节点之间的网络性能，全面展示下载/上传速度、延迟、抖动等关键指标，以及实时速度曲线，帮助您深入了解当前网络状况。
          </Text>
          <div className="ip-block">
            <Group gap="xs" className="ip-header">
              <ThemeIcon radius="xl" size={32} variant="light" color="cyan">
                <IconWorld size={18} />
              </ThemeIcon>
              <Text className="section-label">您的 IP</Text>
            </Group>
            <div className="ip-lines">
              <Text className="ip-line">
                <span>IPv4</span>
                <strong>{ipInfo.ipv4}</strong>
              </Text>
              <Text className="ip-line">
                <span>IPv6</span>
                <strong>{ipInfo.ipv6}</strong>
              </Text>
            </div>
          </div>
          <Group gap="md" className="hero-actions">
            <Text className="switch-hint">测试类型</Text>
            <SegmentedControl
              value={stack}
              onChange={(value) => setStack(value as StackMode)}
              data={[
                { label: 'IPv4', value: 'ipv4' },
                { label: 'IPv6', value: 'ipv6' },
              ]}
              radius="xl"
              size="md"
            />
            <Button
              size="lg"
              radius="xl"
              className="primary-button"
              leftSection={<IconBrandSpeedtest size={18} />}
              onClick={handleStart}
              loading={isRunning}
            >
              {isRunning ? '测速进行中' : '开始测速'}
            </Button>
          </Group>
        </div>

        <Card className="live-card" radius="xl" padding="xl">
          <Stack gap="md">
            <Group justify="space-between" align="flex-start">
              <div>
                <Text className="section-label">{activeMetricLabel}</Text>
                <div className="speed-readout">
                  <span>{formatSpeed(currentMbps)}</span>
                  <small>Mbps</small>
                </div>
              </div>
              <Badge color={status === 'failed' ? 'red' : 'blue'} variant="light" radius="xl">
                {statusLabels[status]}
              </Badge>
            </Group>
            <Text className="status-message">{statusMessage}</Text>
            <Progress value={statusProgress[status]} radius="xl" size="lg" color="cyan" />
            <SimpleGrid cols={{ base: 1, sm: 3 }} spacing="md">
              <MetricCard
                icon={<IconClockHour4 size={18} />}
                label="延迟"
                value={hasLatencyResult ? `${formatMetric(result.latencyMs!, 1)} ms` : '--'}
              />
              <MetricCard
                icon={<IconActivityHeartbeat size={18} />}
                label="抖动"
                value={hasLatencyResult ? `${formatMetric(result.jitterMs!, 1)} ms` : '--'}
              />
              <MetricCard
                icon={<IconArrowsDownUp size={18} />}
                label="并发线程"
                value={`${speedtestConfig.download.concurrency}/${speedtestConfig.upload.concurrency}`}
              />
            </SimpleGrid>
          </Stack>
        </Card>
      </section>

      {error ? (
        <Alert
          color="red"
          variant="light"
          radius="xl"
          icon={<IconInfoCircle size={18} />}
          className="error-banner"
        >
          {error}
        </Alert>
      ) : null}

      <SimpleGrid cols={{ base: 1, lg: 3 }} spacing="lg" className="summary-grid">
        <SummaryCard
          title="延迟与抖动"
          icon={<IconClockHour4 size={18} />}
          value={hasLatencyResult ? `${formatMetric(result.latencyMs!, 1)} / ${formatMetric(result.jitterMs!, 1)} ms` : '-'}
          accent="neutral"
          detail={hasLatencyResult ? '延迟阶段完成后立即展示，便于先确认链路质量。' : '等待延迟与抖动测试完成。'}
        />
        <SummaryCard
          title="下载均速"
          icon={<IconDownload size={18} />}
          value={hasDownloadResult ? `${formatSpeed(result.downloadMbps!)} Mbps` : '-'}
          accent="download"
          detail={
            hasDownloadResult
              ? `峰值 ${formatSpeed(result.downloadPeakMbps!)} Mbps · ${result.downloadSeries?.length ?? 0} 个采样点`
              : `默认 ${speedtestConfig.download.concurrency} 线程 / ${speedtestConfig.download.durationMs / 1000}s`
          }
        />
        <SummaryCard
          title="上传均速"
          icon={<IconCloudUpload size={18} />}
          value={hasUploadResult ? `${formatSpeed(result.uploadMbps!)} Mbps` : '-'}
          accent="upload"
          detail={
            hasUploadResult
              ? `峰值 ${formatSpeed(result.uploadPeakMbps!)} Mbps · ${result.uploadSeries?.length ?? 0} 个采样点`
              : `默认 ${speedtestConfig.upload.concurrency} 线程 / ${speedtestConfig.upload.durationMs / 1000}s`
          }
        />
      </SimpleGrid>

      <SimpleGrid cols={{ base: 1, lg: 2 }} spacing="lg" className="dual-chart-grid">
        <ChartCard
          title="下载速度曲线"
          badge={downloadChartData.length > 0 ? `${downloadChartData.length} 个采样点` : '等待下载测速'}
          color="#0087ff"
          data={downloadChartData}
          emptyTitle="完成下载测速后显示曲线"
          emptyText="下载阶段结束后会先在这里展示吞吐趋势。"
        />
        <ChartCard
          title="上传速度曲线"
          badge={uploadChartData.length > 0 ? `${uploadChartData.length} 个采样点` : '等待上传测速'}
          color="#14b789"
          data={uploadChartData}
          emptyTitle="完成上传测速后显示曲线"
          emptyText="上传阶段结束后会在这里展示完整趋势。"
        />
      </SimpleGrid>
    </main>
  )
}

function ChartCard({
  title,
  badge,
  color,
  data,
  emptyTitle,
  emptyText,
}: {
  title: string
  badge: string
  color: string
  data: Array<{ second: number; speed: number }>
  emptyTitle: string
  emptyText: string
}) {
  return (
    <Card className="chart-card" radius="xl" padding="xl">
      <Group justify="space-between" align="center" mb="md">
        <div>
          <Text className="section-label">速度曲线</Text>
          <Title order={2}>{title}</Title>
        </div>
        <Badge variant="dot" color="teal">
          {badge}
        </Badge>
      </Group>
      {data.length > 0 ? (
        <div className="chart-wrap">
          <ResponsiveContainer width="100%" height={360}>
            <LineChart data={data} margin={{ top: 12, right: 8, left: -12, bottom: 0 }}>
              <CartesianGrid strokeDasharray="4 4" stroke="rgba(102, 127, 171, 0.18)" />
              <XAxis
                dataKey="second"
                tickFormatter={(value: number) => `${value.toFixed(1)}s`}
                tickLine={false}
                axisLine={false}
                stroke="#6f7c92"
              />
              <YAxis
                tickFormatter={(value: number) => `${value.toFixed(0)}`}
                tickLine={false}
                axisLine={false}
                stroke="#6f7c92"
                width={56}
              />
              <Tooltip
                formatter={(value) => [`${Number(value ?? 0).toFixed(2)} Mbps`, '速度']}
                labelFormatter={(value) => `时间 ${Number(value).toFixed(2)}s`}
              />
              <Line
                type="monotone"
                dataKey="speed"
                name={title}
                stroke={color}
                strokeWidth={3}
                dot={false}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      ) : (
        <div className="chart-placeholder">
          <ThemeIcon size={56} radius="xl" variant="light" color="cyan">
            <IconChartLine size={28} />
          </ThemeIcon>
          <Text size="lg" fw={600}>
            {emptyTitle}
          </Text>
          <Text c="dimmed">{emptyText}</Text>
        </div>
      )}
    </Card>
  )
}

interface SummaryCardProps {
  title: string
  value: string
  detail: string
  accent: 'download' | 'upload' | 'neutral'
  icon: ReactNode
}

function SummaryCard({ title, value, detail, accent, icon }: SummaryCardProps) {
  return (
    <Card className={`summary-card ${accent}`} radius="xl" padding="xl">
      <Stack gap="lg">
        <Group justify="space-between">
          <Text className="section-label">{title}</Text>
          <ThemeIcon radius="xl" size={40} variant="light">
            {icon}
          </ThemeIcon>
        </Group>
        <Text className="summary-value">{value}</Text>
        <Text className="summary-detail">{detail}</Text>
      </Stack>
    </Card>
  )
}

interface MetricCardProps {
  label: string
  value: string
  icon: ReactNode
}

function MetricCard({ label, value, icon }: MetricCardProps) {
  return (
    <div className="metric-chip">
      <ThemeIcon radius="xl" size={34} variant="light" color="cyan">
        {icon}
      </ThemeIcon>
      <div>
        <Text className="metric-label">{label}</Text>
        <Text className="metric-value">{value}</Text>
      </div>
    </div>
  )
}

export default App
