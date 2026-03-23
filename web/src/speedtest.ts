import { speedtestConfig, type SpeedPhaseConfig, type StackMode } from './config'

export type TestStatus =
  | 'idle'
  | 'preparing'
  | 'latency'
  | 'download'
  | 'upload'
  | 'completed'
  | 'failed'

export interface PhaseSample {
  timeSeconds: number
  mbps: number
}

export interface SpeedtestResult {
  stack: StackMode
  latencyMs: number
  jitterMs: number
  downloadMbps: number
  uploadMbps: number
  downloadPeakMbps: number
  uploadPeakMbps: number
  downloadSeries: PhaseSample[]
  uploadSeries: PhaseSample[]
  testedAt: string
}

interface StageEvent {
  status: TestStatus
  currentMbps?: number
  message?: string
  snapshot?: Partial<SpeedtestResult>
}

interface PhaseRunResult {
  averageMbps: number
  peakMbps: number
  series: PhaseSample[]
}

interface LatencyResult {
  latencyMs: number
  jitterMs: number
}

const wait = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms))

const smoothValue = (previous: number | null, next: number) => {
  if (previous === null) {
    return next
  }

  const alpha = speedtestConfig.displaySmoothingFactor
  return previous * (1 - alpha) + next * alpha
}

const toMbps = (bytes: number, durationMs: number) => {
  if (durationMs <= 0) {
    return 0
  }

  return (bytes * 8) / durationMs / 1000
}

const clampSeries = (series: PhaseSample[]) => {
  if (series.length <= speedtestConfig.chartPointsLimit) {
    return series
  }

  return series.slice(series.length - speedtestConfig.chartPointsLimit)
}

async function ensureHealthy(baseUrl: string, signal: AbortSignal) {
  const response = await fetch(`${baseUrl}/api/v1/health`, {
    cache: 'no-store',
    mode: 'cors',
    signal,
  })

  if (!response.ok) {
    throw new Error('测速节点不可用')
  }
}

async function measureLatency(baseUrl: string, signal: AbortSignal): Promise<LatencyResult> {
  const samples: number[] = []

  for (let index = 0; index < speedtestConfig.latency.sampleCount; index += 1) {
    const startedAt = performance.now()
    const response = await fetch(`${baseUrl}/api/v1/latency?i=${index}&t=${Date.now()}`, {
      cache: 'no-store',
      mode: 'cors',
      signal,
    })

    if (!response.ok) {
      throw new Error('延迟测试失败')
    }

    await response.json()
    const elapsed = performance.now() - startedAt
    samples.push(elapsed)

    if (index < speedtestConfig.latency.sampleCount - 1) {
      signal.throwIfAborted()
      await wait(speedtestConfig.latency.sampleGapMs)
    }
  }

  const latencyMs = samples.reduce((sum, value) => sum + value, 0) / samples.length
  const deltas = samples.slice(1).map((value, index) => Math.abs(value - samples[index]))
  const jitterMs =
    deltas.length > 0
      ? deltas.reduce((sum, value) => sum + value, 0) / deltas.length
      : 0

  return {
    latencyMs,
    jitterMs,
  }
}

async function runDownloadWorker(
  baseUrl: string,
  phase: SpeedPhaseConfig,
  deadline: number,
  signal: AbortSignal,
  onBytes: (count: number) => void,
) {
  while (performance.now() < deadline && !signal.aborted) {
    const response = await fetch(
      `${baseUrl}/api/v1/download?bytes=${phase.chunkBytes}&chunkSize=${Math.min(262144, phase.chunkBytes)}&t=${Date.now()}-${Math.random()}`,
      {
        cache: 'no-store',
        mode: 'cors',
        signal,
      },
    )

    if (!response.ok || !response.body) {
      throw new Error('下载测试失败')
    }

    const reader = response.body.getReader()

    while (!signal.aborted) {
      const { done, value } = await reader.read()

      if (done) {
        break
      }

      onBytes(value.byteLength)
      if (performance.now() >= deadline) {
        await reader.cancel()
        break
      }
    }
  }
}

async function runUploadWorker(
  baseUrl: string,
  deadline: number,
  signal: AbortSignal,
  onBytes: (count: number) => void,
  payloadByteLength: number,
  uploadBody: ArrayBuffer,
) {
  while (performance.now() < deadline && !signal.aborted) {
    const response = await fetch(`${baseUrl}/api/v1/upload?t=${Date.now()}-${Math.random()}`, {
      method: 'POST',
      mode: 'cors',
      cache: 'no-store',
      headers: {
        'Content-Type': 'application/octet-stream',
      },
      body: uploadBody.slice(0),
      signal,
    })

    if (!response.ok) {
      throw new Error('上传测试失败')
    }

    onBytes(payloadByteLength)
  }
}

async function runPhase(
  status: 'download' | 'upload',
  baseUrl: string,
  phase: SpeedPhaseConfig,
  signal: AbortSignal,
  emit: (event: StageEvent) => void,
): Promise<PhaseRunResult> {
  const phaseController = new AbortController()
  const stopPhase = () => phaseController.abort()
  signal.addEventListener('abort', stopPhase, { once: true })
  let totalBytes = 0
  let lastBytes = 0
  let peakMbps = 0
  const startedAt = performance.now()
  const deadline = startedAt + phase.durationMs
  let timerId = 0
  let lastSampleAt = startedAt
  let series: PhaseSample[] = []
  let smoothedMbps: number | null = null

  const onBytes = (count: number) => {
    totalBytes += count
  }

  const payload =
    status === 'upload'
      ? new Uint8Array(
          Array.from({ length: phase.chunkBytes }, (_, index) => (index * 31) % 251),
        )
      : null
  const uploadBody =
    payload === null
      ? null
      : (() => {
          const body = new ArrayBuffer(payload.byteLength)
          new Uint8Array(body).set(payload)
          return body
        })()

  const workerPromises = Array.from({ length: phase.concurrency }, () => {
    const worker =
      status === 'download'
        ? runDownloadWorker(baseUrl, phase, deadline, phaseController.signal, onBytes)
        : runUploadWorker(
            baseUrl,
            deadline,
            phaseController.signal,
            onBytes,
            payload!.byteLength,
            uploadBody!,
          )

    return worker.catch((error: unknown) => {
      if (phaseController.signal.aborted) {
        return
      }

      throw error
    })
  })

  try {
    await new Promise<void>((resolve, reject) => {
    timerId = window.setInterval(() => {
      const now = performance.now()
      const durationMs = now - lastSampleAt
      const deltaBytes = totalBytes - lastBytes
      const mbps = toMbps(deltaBytes, durationMs)
      smoothedMbps = smoothValue(smoothedMbps, mbps)
      peakMbps = Math.max(peakMbps, mbps)
      series = clampSeries([
        ...series,
        {
          timeSeconds: (now - startedAt) / 1000,
          mbps: smoothedMbps,
        },
      ])
      lastBytes = totalBytes
      lastSampleAt = now
      emit({ status, currentMbps: smoothedMbps })

      if (now >= deadline || signal.aborted) {
        phaseController.abort()
        window.clearInterval(timerId)
        resolve()
      }
    }, speedtestConfig.samplingIntervalMs)

    signal.addEventListener(
      'abort',
      () => {
        phaseController.abort()
        window.clearInterval(timerId)
        resolve()
      },
      { once: true },
    )

    Promise.all(workerPromises)
      .then(() => {
        window.clearInterval(timerId)
        resolve()
      })
      .catch((error: unknown) => {
        window.clearInterval(timerId)
        reject(error)
      })
    })
  } finally {
    phaseController.abort()
    signal.removeEventListener('abort', stopPhase)
    await Promise.allSettled(workerPromises)
  }

  const endedAt = performance.now()
  const averageMbps = toMbps(totalBytes, endedAt - startedAt)

  return {
    averageMbps,
    peakMbps: Math.max(peakMbps, averageMbps),
    series,
  }
}

export async function runSpeedtest(
  stack: StackMode,
  signal: AbortSignal,
  emit: (event: StageEvent) => void,
): Promise<SpeedtestResult> {
  const baseUrl = speedtestConfig.apiTargets[stack]

  emit({ status: 'preparing', message: '检查测速节点可用性' })
  await ensureHealthy(baseUrl, signal)

  emit({ status: 'latency', message: '测试延迟与抖动' })
  const latency = await measureLatency(baseUrl, signal)
  emit({
    status: 'latency',
    message: '延迟与抖动测试完成',
    snapshot: {
      stack,
      latencyMs: latency.latencyMs,
      jitterMs: latency.jitterMs,
      downloadSeries: [],
      uploadSeries: [],
    },
  })

  emit({ status: 'download', currentMbps: 0, message: '正在进行下载测试' })
  const download = await runPhase('download', baseUrl, speedtestConfig.download, signal, emit)
  emit({
    status: 'download',
    currentMbps: download.averageMbps,
    message: '下载测试完成，准备开始上传',
    snapshot: {
      stack,
      latencyMs: latency.latencyMs,
      jitterMs: latency.jitterMs,
      downloadMbps: download.averageMbps,
      downloadPeakMbps: download.peakMbps,
      downloadSeries: download.series,
      uploadSeries: [],
    },
  })

  emit({ status: 'upload', currentMbps: 0, message: '正在进行上传测试' })
  const upload = await runPhase('upload', baseUrl, speedtestConfig.upload, signal, emit)
  emit({
    status: 'upload',
    currentMbps: upload.averageMbps,
    message: '上传测试完成',
    snapshot: {
      stack,
      latencyMs: latency.latencyMs,
      jitterMs: latency.jitterMs,
      downloadMbps: download.averageMbps,
      uploadMbps: upload.averageMbps,
      downloadPeakMbps: download.peakMbps,
      uploadPeakMbps: upload.peakMbps,
      downloadSeries: download.series,
      uploadSeries: upload.series,
    },
  })

  emit({ status: 'completed', message: '测速完成' })

  return {
    stack,
    latencyMs: latency.latencyMs,
    jitterMs: latency.jitterMs,
    downloadMbps: download.averageMbps,
    uploadMbps: upload.averageMbps,
    downloadPeakMbps: download.peakMbps,
    uploadPeakMbps: upload.peakMbps,
    downloadSeries: download.series,
    uploadSeries: upload.series,
    testedAt: new Date().toISOString(),
  }
}
