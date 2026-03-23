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

export const speedtestConfig: SpeedtestConfig = {
  apiTargets: {
    ipv4: 'https://v4-speedtest.taurusxin.com',
    ipv6: 'https://v6-speedtest.taurusxin.com',
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
