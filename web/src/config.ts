export type StackMode = 'ipv4' | 'ipv6'

export interface SpeedPhaseConfig {
  concurrency: number
  durationMs: number
  chunkBytes: number
}

export interface DualStackTargetMap {
  ipv4: string
  ipv6: string
}

export interface SpeedtestConfig {
  apiTargets: DualStackTargetMap
  latency: {
    sampleCount: number
    sampleGapMs: number
  }
  download: SpeedPhaseConfig
  upload: SpeedPhaseConfig
  samplingIntervalMs: number
  chartPointsLimit: number
  displaySmoothingFactor: number
}

export const defaultSpeedtestConfig: SpeedtestConfig = {
  apiTargets: {
    ipv4: 'speedtest-v4only.taurusxin.com',
    ipv6: 'speedtest-v6only.taurusxin.com',
  },
  latency: {
    sampleCount: 10,
    sampleGapMs: 160,
  },
  download: {
    concurrency: 6,
    durationMs: 9000,
    chunkBytes: 6 * 1024 * 1024,
  },
  upload: {
    concurrency: 4,
    durationMs: 7000,
    chunkBytes: 1024 * 1024,
  },
  samplingIntervalMs: 250,
  chartPointsLimit: 120,
  displaySmoothingFactor: 0.35,
}

let speedtestConfigState: SpeedtestConfig = defaultSpeedtestConfig

export function resolveTargetBaseUrl(target: string) {
  if (/^https?:\/\//i.test(target)) {
    return target
  }

  return `${window.location.protocol}//${target}`
}

export function getSpeedtestConfig() {
  return speedtestConfigState
}

export async function loadRuntimeConfig() {
  try {
    const response = await fetch('/api/v1/runtime-config', {
      cache: 'no-store',
    })

    if (!response.ok) {
      return speedtestConfigState
    }

    const runtimeConfig = (await response.json()) as Partial<SpeedtestConfig>
    speedtestConfigState = {
      ...defaultSpeedtestConfig,
      ...runtimeConfig,
      apiTargets: {
        ...defaultSpeedtestConfig.apiTargets,
        ...runtimeConfig.apiTargets,
      },
      latency: {
        ...defaultSpeedtestConfig.latency,
        ...runtimeConfig.latency,
      },
      download: {
        ...defaultSpeedtestConfig.download,
        ...runtimeConfig.download,
      },
      upload: {
        ...defaultSpeedtestConfig.upload,
        ...runtimeConfig.upload,
      },
    }
  } catch {
    speedtestConfigState = defaultSpeedtestConfig
  }

  return speedtestConfigState
}
