<template>
  <div class="page-container dashboard" v-loading="loading">
    <!-- 资源概览：只放“总量”类指标，状态明细在下方设备面板、消息指标在消息面板，不重复 -->
    <header class="overview">
      <div v-if="isSuperAdmin" class="ov-item">
        <el-icon class="ov-ic ov-blue"><OfficeBuilding /></el-icon>
        <div><div class="ov-num">{{ stats.total_tenants }}</div><div class="ov-label">租户</div></div>
      </div>
      <div v-if="isSuperAdmin" class="ov-divider" />
      <div class="ov-item">
        <el-icon class="ov-ic ov-cyan"><FolderOpened /></el-icon>
        <div><div class="ov-num">{{ stats.total_projects }}</div><div class="ov-label">项目</div></div>
      </div>
      <div class="ov-divider" />
      <div class="ov-item">
        <el-icon class="ov-ic ov-slate"><Cpu /></el-icon>
        <div><div class="ov-num">{{ stats.total_devices }}</div><div class="ov-label">设备总数</div></div>
      </div>
      <div class="ov-divider" />
      <div class="ov-item">
        <el-icon class="ov-ic ov-green"><Connection /></el-icon>
        <div><div class="ov-num ov-text-green">{{ stats.online_devices }}</div><div class="ov-label">在线 · {{ onlinePercent }}%</div></div>
      </div>

      <!-- 设备状态分布：环图 + 四态明细，收进概览条右侧同一容器 -->
      <div class="ov-sep-status"></div>
      <div class="ov-status">
        <svg viewBox="0 0 120 120" class="donut ov-donut">
          <circle cx="60" cy="60" r="46" class="donut-track" />
          <circle
            v-for="seg in donutSegs"
            :key="seg.key"
            cx="60" cy="60" r="46"
            :stroke="seg.color"
            :stroke-dasharray="seg.dash"
            :stroke-dashoffset="seg.offset"
            class="donut-seg"
          />
          <text x="60" y="55" text-anchor="middle" class="donut-total">{{ stats.total_devices }}</text>
          <text x="60" y="73" text-anchor="middle" class="donut-label">设备总数</text>
        </svg>
        <ul class="donut-legend ov-legend">
          <li v-for="seg in donutSegs" :key="seg.key">
            <i :style="{ background: seg.color }"></i>
            <span class="dl-name">{{ seg.name }}</span>
            <b>{{ seg.value }}</b>
          </li>
        </ul>
      </div>
    </header>

    <!-- 消息监控独占整行：时间档（实时/1h/24h/7d）× 指标视图（速率/累计），同一序列前端派生 -->
    <main class="panels">
      <section class="panel panel-message">
        <header class="panel-title">
          <span class="pt-name"><el-icon class="pt-ic"><DataLine /></el-icon>消息监控</span>
          <div class="legend">
            <span class="legend-item"><i class="dot dot-in"></i>流入</span>
            <span class="legend-item"><i class="dot dot-out"></i>流出</span>
          </div>
        </header>

        <div class="msg-toolbar">
          <el-radio-group v-model="timeRange" size="small" @change="onRangeChange">
            <el-radio-button value="live">实时</el-radio-button>
            <el-radio-button value="1h">近1小时</el-radio-button>
            <el-radio-button value="24h">近24小时</el-radio-button>
            <el-radio-button value="7d">近7天</el-radio-button>
          </el-radio-group>
          <el-radio-group v-model="metricView" size="small">
            <el-radio-button value="rate">速率</el-radio-button>
            <el-radio-button value="total">累计</el-radio-button>
          </el-radio-group>
        </div>

        <div class="msg-current">
          <div class="mc-item mc-box-in">
            <span class="mc-label"><i class="dot dot-in"></i>消息流入</span>
            <div><b class="mc-num mc-in">{{ currentIn }}</b><em>{{ valUnit }}</em></div>
          </div>
          <div class="mc-item mc-box-out">
            <span class="mc-label"><i class="dot dot-out"></i>消息流出</span>
            <div><b class="mc-num mc-out">{{ currentOut }}</b><em>{{ valUnit }}</em></div>
          </div>
        </div>

        <div class="chart-wrap" v-loading="histLoading">
          <svg class="rate-svg" viewBox="0 0 800 230" preserveAspectRatio="xMidYMid meet">
            <line v-for="g in 4" :key="'g'+g" :x1="padL" :x2="chartRight" :y1="gridY(g)" :y2="gridY(g)" class="grid" />
            <line :x1="padL" :x2="chartRight" :y1="chartBottom" :y2="chartBottom" class="axis" />
            <text v-for="g in 5" :key="'y'+g" :x="padL-8" :y="gridY(g-1)+4" text-anchor="end" class="svg-label">{{ yLabels[g-1] }}</text>
            <path v-if="inArea" :d="inArea" class="area-in" />
            <path v-if="outArea" :d="outArea" class="area-out" />
            <path v-if="inPath" :d="inPath" class="line-in" fill="none" />
            <path v-if="outPath" :d="outPath" class="line-out" fill="none" />
            <text v-for="(t, i) in xTicks" :key="'x'+i" :x="t.x" :y="chartBottom+20" :text-anchor="t.anchor" class="svg-label svg-x">{{ t.label }}</text>
            <text v-if="seriesEmpty()" :x="400" :y="(topPad+chartBottom)/2" text-anchor="middle" class="svg-empty">{{ emptyText }}</text>
          </svg>
        </div>
        <p class="chart-hint">{{ chartHint }}</p>

        <footer class="msg-sum">
          <span v-if="timeRange === 'live'">近 24h：流入 <b class="ms-in">{{ formatNum(stats.messages_in_24h) }}</b> · 流出 <b class="ms-out">{{ formatNum(stats.messages_out_24h) }}</b></span>
          <span v-else>本区间合计：流入 <b class="ms-in">{{ rangeSumIn }}</b> · 流出 <b class="ms-out">{{ rangeSumOut }}</b></span>
          <span class="ms-total">数据库累计：流入 {{ formatNum(stats.messages_in_total_db) }} · 流出 {{ formatNum(stats.messages_out_total_db) }}</span>
        </footer>
      </section>
    </main>

    <footer class="sys-bar">
      <el-icon><CircleCheck /></el-icon> 服务运行中 · v1.0.0
      <span class="sys-sep">|</span>{{ authStore.user?.username || '-' }}
      （{{ authStore.user?.role === 'super_admin' ? '超级管理员' : '租户管理员' }}）
    </footer>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, onMounted, onUnmounted, ref } from 'vue'
import {
  OfficeBuilding, FolderOpened, Cpu, Connection,
  DataLine, CircleCheck
} from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import {
  getDashboardStats, getDashboardTraffic,
  type DashboardStats, type TrafficRangeKey
} from '@/api/dashboard'
import { useRealtime } from '@/composables/useRealtime'

const authStore = useAuthStore()
const loading = ref(false)
const refreshSec = 2

const stats = reactive<DashboardStats>({
  total_tenants: 0,
  total_projects: 0,
  total_devices: 0,
  online_devices: 0,
  disabled_devices: 0,
  pending_devices: 0,
  messages_in_24h: 0,
  messages_out_24h: 0,
  messages_in_total_db: 0,
  messages_out_total_db: 0,
  in_rate: 0,
  out_rate: 0,
  in_total: 0,
  out_total: 0,
  series_in: [],
  series_out: []
})

const isSuperAdmin = computed(() => authStore.role === 'super_admin')

const onlinePercent = computed(() => {
  if (stats.total_devices === 0) return 0
  return Math.round(stats.online_devices / stats.total_devices * 100)
})

// 离线 = 总数 - 在线 - 禁用 - 待激活（待激活设备本身未上线，单独成类；与 donut 口径一致）
const offlineCount = computed(() =>
  Math.max(0, stats.total_devices - stats.online_devices - stats.disabled_devices - stats.pending_devices)
)

function formatNum(n: number): string {
  return (n || 0).toLocaleString('zh-CN')
}

// 大数字速率取最近 5 秒计数（近似滑动窗口，条/秒），比"当前秒"稳定、不频繁跳 0；
// 瞬时波动由下方 30 秒折线展示
// ===== 消息监控：时间档 × 指标视图 =====
type TimeRangeKey = 'live' | TrafficRangeKey
type MetricView = 'rate' | 'total'
const timeRange = ref<TimeRangeKey>('live')
const metricView = ref<MetricView>('rate')
const histLoading = ref(false)
const hist = ref<{ bucketSec: number; startSec: number; in: number[]; out: number[] } | null>(null)

const RANGE_BUCKET: Record<TimeRangeKey, number> = { live: 1, '1h': 60, '24h': 600, '7d': 3600 }
const HIST_REFRESH_SEC = 30

// 原始每桶“条数”序列：实时=内存环 30 点(1s)；历史=接口分桶
const rawIn = computed<number[]>(() =>
  timeRange.value === 'live' ? stats.series_in : hist.value?.in ?? [])
const rawOut = computed<number[]>(() =>
  timeRange.value === 'live' ? stats.series_out : hist.value?.out ?? [])

// 展示序列：速率=每桶条数/桶秒；累计=前缀和
function perRate(a: number[], bucketSec: number): number[] {
  return a.map(v => v / bucketSec)
}
function cumulative(a: number[]): number[] {
  let acc = 0
  return a.map(v => (acc += v))
}

const viewIn = computed<number[]>(() => {
  const b = RANGE_BUCKET[timeRange.value]
  return metricView.value === 'rate' ? perRate(rawIn.value, b) : cumulative(rawIn.value)
})
const viewOut = computed<number[]>(() => {
  const b = RANGE_BUCKET[timeRange.value]
  return metricView.value === 'rate' ? perRate(rawOut.value, b) : cumulative(rawOut.value)
})

const valUnit = computed(() => (metricView.value === 'rate' ? '条/秒' : '条'))
const currentIn = computed(() => formatChartValue(viewIn.value[viewIn.value.length - 1] ?? 0))
const currentOut = computed(() => formatChartValue(viewOut.value[viewOut.value.length - 1] ?? 0))

const rangeSumIn = computed(() => formatNum(rawIn.value.reduce((a, b) => a + b, 0)))
const rangeSumOut = computed(() => formatNum(rawOut.value.reduce((a, b) => a + b, 0)))

const emptyText = computed(() =>
  timeRange.value === 'live' ? '暂无实时消息' : '该时间段暂无消息')
const chartHint = computed(() => {
  if (timeRange.value === 'live') {
    return `最近 30 秒速率（${refreshSec}s 刷新）· 当前服务节点近似值`
  }
  const b = RANGE_BUCKET[timeRange.value]
  const bucketLabel = b >= 3600 ? `${b / 3600} 小时` : b >= 60 ? `${b / 60} 分钟` : `${b} 秒`
  return `每 ${bucketLabel}一个采样点 · ${HIST_REFRESH_SEC}s 自动刷新 · 数据来自数据库消息记录`
})

// ===== 折线（viewBox 800x230，绘图区 x:[padL,chartRight] y:[20,180]） =====
const padL = 46
const chartRight = 790
const topPad = 20
const chartBottom = 180

function gridY(g: number): number {
  return chartBottom - g * ((chartBottom - topPad) / 4)
}

const yAxisMax = computed(() => {
  const max = Math.max(1, ...viewIn.value, ...viewOut.value)
  return niceStep(max) * 4
})
const yLabels = computed(() => {
  const max = yAxisMax.value / 4
  return [0, 1, 2, 3, 4].map(g => formatAxis(max * g))
})

function niceStep(max: number): number {
  if (max <= 4) return 1
  const raw = max / 4
  const pow = Math.pow(10, Math.floor(Math.log10(raw)))
  const n = raw / pow
  const m = n < 1.5 ? 1 : n < 3 ? 2 : n < 7 ? 5 : 10
  return m * pow
}

function formatAxis(v: number): string {
  if (v >= 10000) return `${Math.round(v / 1000) / 10}w`
  if (Number.isInteger(v)) return String(v)
  return (Math.round(v * 10) / 10).toString()
}
function formatChartValue(v: number): string {
  if (Number.isInteger(v)) return formatNum(v)
  return (Math.round(v * 10) / 10).toLocaleString('zh-CN')
}

function buildPath(series: number[]): { line: string; area: string } {
  if (!series.length || series.every(v => v === 0)) return { line: '', area: '' }
  const maxV = Math.max(1, yAxisMax.value)
  const n = series.length
  const left = padL
  const h = chartBottom - topPad
  const w = chartRight - left
  const xy = series.map((v, i) => {
    const x = left + (n === 1 ? w / 2 : (i / (n - 1)) * w)
    const y = chartBottom - Math.min(v, maxV) / maxV * h
    return [x, y] as const
  })
  const line = xy.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ')
  const area = `${line} L${xy[n - 1][0].toFixed(1)},${chartBottom} L${xy[0][0].toFixed(1)},${chartBottom} Z`
  return { line, area }
}

const inPath = computed(() => buildPath(viewIn.value).line)
const outPath = computed(() => buildPath(viewOut.value).line)
const inArea = computed(() => buildPath(viewIn.value).area)
const outArea = computed(() => buildPath(viewOut.value).area)

function seriesEmpty(): boolean {
  return viewIn.value.every(v => v === 0) && viewOut.value.every(v => v === 0)
}

// X 轴时间刻度（约 6 个）
function pad2(n: number): string { return String(n).padStart(2, '0') }
const xTicks = computed(() => {
  const n = timeRange.value === 'live'
    ? stats.series_in.length
    : (hist.value?.in.length ?? 0)
  if (!n) return []
  const bucketSec = RANGE_BUCKET[timeRange.value]
  const nowSec = Math.floor(Date.now() / 1000)
  const startSec = timeRange.value === 'live'
    ? nowSec - (n - 1)
    : hist.value?.startSec ?? nowSec - (n - 1) * bucketSec
  const left = padL, w = chartRight - left
  const tickCount = Math.min(6, n)
  const ticks: { x: number; label: string; anchor: string }[] = []
  for (let t = 0; t < tickCount; t++) {
    const i = tickCount === 1 ? 0 : Math.round(t * (n - 1) / (tickCount - 1))
    const d = new Date((startSec + i * bucketSec) * 1000)
    let label: string
    if (bucketSec >= 3600) label = `${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
    else if (bucketSec === 1) label = `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
    else label = `${pad2(d.getHours())}:${pad2(d.getMinutes())}`
    const anchor = t === 0 ? 'start' : t === tickCount - 1 ? 'end' : 'middle'
    ticks.push({ x: left + (n === 1 ? w / 2 : (i / (n - 1)) * w), label, anchor })
  }
  return ticks
})

// ===== 设备状态环 =====
const donutSegs = computed(() => {
  const total = Math.max(1, stats.total_devices)
  const C = 2 * Math.PI * 46
  const defs = [
    { key: 'online', name: '在线', value: stats.online_devices, color: '#67c23a' },
    { key: 'offline', name: '离线', value: offlineCount.value, color: '#c0c4cc' },
    { key: 'pending', name: '待激活', value: stats.pending_devices, color: '#e6a23c' },
    { key: 'disabled', name: '已禁用', value: stats.disabled_devices, color: '#f56c6c' }
  ]
  let acc = 0
  return defs.map(d => {
    const frac = d.value / total
    const seg = { ...d, dash: `${frac * C} ${C}`, offset: -acc * C }
    acc += frac
    return seg
  })
})

async function fetchStats(showLoading = false) {
  if (showLoading) loading.value = true
  try {
    const res = await getDashboardStats()
    if (res.data) Object.assign(stats, res.data)
  } catch {
  } finally {
    loading.value = false
  }
}

async function fetchTraffic(range: TrafficRangeKey) {
  histLoading.value = true
  try {
    const res = await getDashboardTraffic(range)
    if (res.data) {
      hist.value = {
        bucketSec: res.data.bucket_sec,
        startSec: res.data.start_sec,
        in: res.data.series_in,
        out: res.data.series_out
      }
    }
  } catch {
  } finally {
    histLoading.value = false
  }
}

function onRangeChange(v: string | number | boolean | undefined) {
  const r = String(v) as TimeRangeKey
  hist.value = null
  if (r !== 'live') fetchTraffic(r as TrafficRangeKey)
}

// 2s 定轮询实时速率与总量；历史档 30s 刷新；设备上下线/激活事件防抖重取
const { onMessage } = useRealtime()
let offRealtime: (() => void) | null = null
let pollTimer: ReturnType<typeof setInterval> | null = null
let histTimer: ReturnType<typeof setInterval> | null = null
let refreshTimer: ReturnType<typeof setTimeout> | null = null

onMounted(() => {
  fetchStats(true)
  pollTimer = setInterval(() => fetchStats(), refreshSec * 1000)
  histTimer = setInterval(() => {
    if (timeRange.value !== 'live') fetchTraffic(timeRange.value as TrafficRangeKey)
  }, HIST_REFRESH_SEC * 1000)
  offRealtime = onMessage((msg) => {
    if (msg.type !== 'device_status' && msg.type !== 'device_activated') return
    if (refreshTimer) clearTimeout(refreshTimer)
    refreshTimer = setTimeout(() => fetchStats(), 1500)
  })
})

onUnmounted(() => {
  offRealtime?.()
  if (pollTimer) clearInterval(pollTimer)
  if (histTimer) clearInterval(histTimer)
  if (refreshTimer) clearTimeout(refreshTimer)
})
</script>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ===== 资源概览条：一行总量，分隔线分段，无卡片盒子 ===== */
.overview {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 18px 22px;
  padding: 16px 22px;
  background: var(--wd-surface);
  border: 1px solid var(--wd-border);
  border-radius: var(--wd-radius-lg);
  box-shadow: var(--wd-shadow-sm);
}
.ov-item {
  display: flex;
  align-items: center;
  gap: 12px;
}
/* 右侧设备状态分布（与总量指标同一容器，右对齐） */
.ov-sep-status {
  width: 1px;
  height: 72px;
  margin-left: auto;
  background: var(--wd-border-lighter);
}
.ov-status {
  display: flex;
  align-items: center;
  gap: 18px;
  flex-shrink: 0;
}
.ov-donut { width: 92px; height: 92px; flex-shrink: 0; }
.ov-legend {
  list-style: none;
  margin: 0;
  padding: 0;
  flex: none;
  min-width: 0;
  display: grid;
  grid-template-columns: auto auto;
  gap: 4px 20px;
}
.ov-legend li {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0;
  border: none;
  font-size: 12px;
  color: var(--wd-text-regular);
  white-space: nowrap;
}
.ov-legend b { font-size: 13px; font-variant-numeric: tabular-nums; color: var(--wd-text-primary); }
.ov-ic {
  font-size: 20px;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.ov-blue { color: #409eff; background: #ecf5ff; }
.ov-cyan { color: #13c2c2; background: #e6fffb; }
.ov-slate { color: #606266; background: #f4f4f5; }
.ov-green { color: var(--wd-success); background: var(--wd-success-bg); }
.ov-num {
  font-size: 22px;
  font-weight: 700;
  line-height: 1.15;
  font-variant-numeric: tabular-nums;
  color: var(--wd-text-primary);
}
.ov-text-green { color: var(--wd-success); }
.ov-label { font-size: 12.5px; color: var(--wd-text-secondary); }
.ov-divider { width: 1px; height: 28px; background: var(--wd-border-lighter); }

/* ===== 消息监控独占整行 ===== */
.panels {
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
  align-items: stretch;
}
.panel {
  background: var(--wd-surface);
  border: 1px solid var(--wd-border);
  border-radius: var(--wd-radius-lg);
  box-shadow: var(--wd-shadow-sm);
  padding: 18px 20px;
  min-width: 0;
}
.panel-title {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14px;
  font-weight: 600;
  color: var(--wd-text-primary);
  margin-bottom: 16px;
}
.panel-message > .panel-title { justify-content: space-between; }
.panel-title .el-icon { color: var(--wd-primary); font-size: 16px; }
.pt-name { display: inline-flex; align-items: center; gap: 7px; min-width: 0; }
.pt-ic { color: var(--wd-success); font-size: 16px; flex-shrink: 0; }

.legend { display: flex; gap: 14px; font-size: 12px; color: var(--wd-text-regular); }
.legend-item { display: inline-flex; align-items: center; gap: 5px; }
.dot { width: 9px; height: 9px; border-radius: 50%; display: inline-block; flex-shrink: 0; }
.dot-in { background: var(--wd-success); }
.dot-out { background: var(--wd-warning); }

/* ===== 设备状态环（内嵌概览条） ===== */
.donut-track { fill: none; stroke: var(--wd-border-lighter); stroke-width: 13; }
.donut-seg {
  fill: none;
  stroke-width: 13;
  transform: rotate(-90deg);
  transform-origin: 60px 60px;
  transition: stroke-dasharray .4s ease, stroke-dashoffset .4s ease;
}
.donut-total { fill: var(--wd-text-primary); font-size: 23px; font-weight: 700; }
.donut-label { fill: var(--wd-text-secondary); font-size: 10.5px; }
.ov-legend i { width: 9px; height: 9px; border-radius: 2px; flex-shrink: 0; }
.ov-legend .dl-name { flex: none; }

/* ===== 消息监控 ===== */
.msg-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}
.msg-toolbar :deep(.el-radio-button__inner) { padding: 6px 12px; }

.msg-current {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-bottom: 14px;
}
.mc-item {
  padding: 10px 14px;
  border-radius: var(--wd-radius-md);
  min-width: 0;
}
.mc-box-in { background: var(--wd-success-bg); }
.mc-box-out { background: var(--wd-warning-bg); }
.mc-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--wd-text-secondary);
  margin-bottom: 2px;
}
.mc-item div { display: flex; align-items: baseline; gap: 5px; }
.mc-num { font-size: 26px; font-weight: 700; line-height: 1.1; font-variant-numeric: tabular-nums; }
.mc-in { color: var(--wd-success); }
.mc-out { color: var(--wd-warning); }
.mc-item em { font-style: normal; font-size: 12px; color: var(--wd-text-secondary); }

/* 等比缩放（meet）：保证图中文字/网格不被拉伸变形 */
.chart-wrap { position: relative; width: 100%; }
.rate-svg { width: 100%; height: auto; display: block; }
.grid { stroke: var(--wd-border-lighter); stroke-width: 1; }
.axis { stroke: var(--wd-border); stroke-width: 1; }
.svg-label { fill: var(--wd-text-placeholder); font-size: 12px; }
.svg-x { font-size: 11px; }
.svg-empty { fill: var(--wd-text-secondary); font-size: 16px; }
.line-in { stroke: var(--wd-success); stroke-width: 2; }
.line-out { stroke: var(--wd-warning); stroke-width: 2; }
.area-in { fill: var(--wd-success); opacity: 0.1; }
.area-out { fill: var(--wd-warning); opacity: 0.08; }
.chart-hint { margin: 8px 0 0; font-size: 11.5px; color: var(--wd-text-placeholder); }

.msg-sum {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--wd-border-lighter);
  font-size: 12.5px;
  color: var(--wd-text-secondary);
}
.msg-sum b { font-variant-numeric: tabular-nums; }
.ms-in { color: var(--wd-success); }
.ms-out { color: var(--wd-warning); }
.ms-total { color: var(--wd-text-placeholder); }

.sys-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--wd-text-placeholder);
  padding: 2px 4px;
}
.sys-bar .el-icon { color: var(--wd-success); }
.sys-sep { margin: 0 4px; }

@media (max-width: 900px) {
  .panels { grid-template-columns: 1fr; }
}
</style>
