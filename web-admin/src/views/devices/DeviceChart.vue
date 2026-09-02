<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="$router.back()">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h2>设备数据图表 - {{ deviceId }}</h2>
      </div>
      <div class="toolbar-right">
        <el-select
          v-model="selectedTag"
          placeholder="选择数据项"
          style="width: 150px; margin-right: 12px;"
        >
          <el-option
            v-for="tag in availableTags"
            :key="tag"
            :label="getTagName(tag)"
            :value="tag"
          />
        </el-select>
        <el-select
          v-model="timeRange"
          placeholder="时间范围"
          style="width: 120px; margin-right: 12px;"
        >
          <el-option label="最近1小时" value="1h" />
          <el-option label="最近6小时" value="6h" />
          <el-option label="最近24小时" value="24h" />
        </el-select>
        <el-switch
          v-model="autoRefresh"
          active-text="自动刷新"
          inactive-text="手动刷新"
          style="margin-right: 12px;"
        />
        <el-button :icon="Refresh" @click="fetchData" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-row :gutter="20">
      <el-col :span="16">
        <el-card shadow="hover" style="border-radius: 12px;">
          <template #header>
            <span style="font-weight: 600; color: #1f2329;">{{ getTagName(selectedTag) }} 趋势图</span>
          </template>
          <div class="chart-container">
            <svg ref="chartSvg" class="line-chart" viewBox="0 0 800 300" preserveAspectRatio="xMidYMid meet">
              <!-- Y轴网格线 -->
              <line v-for="i in 5" :key="'grid-' + i"
                :x1="60" :y1="i * 60"
                :x2="800" :y2="i * 60"
                stroke="#eee" stroke-width="1" stroke-dasharray="4,4"
              />
              <!-- Y轴标签 -->
              <text v-for="(label, i) in yAxisLabels" :key="'ylabel-' + i"
                :x="50" :y="i * 60 + 4"
                text-anchor="end" class="axis-label"
              >{{ label }}</text>
              <!-- X轴标签 -->
              <text v-for="(label, i) in xAxisLabels" :key="'xlabel-' + i"
                :x="60 + i * (740 / (xAxisLabels.length - 1))" :y="290"
                text-anchor="middle" class="axis-label"
              >{{ label }}</text>
              <!-- 数据线 -->
              <path
                v-if="chartPath"
                :d="chartPath"
                fill="none"
                stroke="#1890ff"
                stroke-width="2"
              />
              <!-- 数据区域填充 -->
              <path
                v-if="areaPath"
                :d="areaPath"
                fill="url(#gradient)"
                opacity="0.3"
              />
              <!-- 渐变定义 -->
              <defs>
                <linearGradient id="gradient" x1="0%" y1="0%" x2="0%" y2="100%">
                  <stop offset="0%" stop-color="#1890ff" />
                  <stop offset="100%" stop-color="#fff" />
                </linearGradient>
              </defs>
              <!-- 数据点 -->
              <circle
                v-for="(point, index) in chartPoints"
                :key="'point-' + index"
                :cx="point.x"
                :cy="point.y"
                r="4"
                fill="#1890ff"
                class="data-point"
                @mouseenter="showTooltip(point, $event)"
                @mouseleave="hideTooltip"
              />
            </svg>
            <div v-if="tooltipVisible" class="tooltip" :style="tooltipStyle">
              <div class="tooltip-time">{{ tooltipData.time }}</div>
              <div class="tooltip-value">{{ tooltipData.value }} {{ getUnit(selectedTag) }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="realtime-card-container" style="border-radius: 12px;">
          <template #header>
            <span style="font-weight: 600; color: #1f2329;">实时数值</span>
          </template>
          <div class="realtime-cards">
            <div v-for="tag in availableTags" :key="tag" class="realtime-card">
              <div class="realtime-content">
                <span class="realtime-label">{{ getTagName(tag) }}</span>
                <span class="realtime-value">{{ getRealtimeValue(tag) }}</span>
                <span class="realtime-unit">{{ getUnit(tag) }}</span>
                <div class="realtime-trend" :class="getTrendClass(tag)">
                  <el-icon v-if="getTrend(tag) > 0"><ArrowUp /></el-icon>
                  <el-icon v-else-if="getTrend(tag) < 0"><ArrowDown /></el-icon>
                  <span>{{ getTrendText(tag) }}</span>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px;">
      <el-col :span="16">
        <el-card shadow="hover" class="table-card">
          <template #header>
            <span>历史数据</span>
            <el-button
              :icon="Refresh"
              size="small"
              @click="fetchHistoryData"
              :loading="historyLoading"
              style="float: right;"
            >刷新</el-button>
          </template>
          <el-table :data="historyData" v-loading="historyLoading" stripe border max-height="400px">
            <el-table-column prop="id" label="ID" width="80" align="center" />
            <el-table-column prop="msg_id" label="消息ID" width="120" show-overflow-tooltip />
            <el-table-column prop="ts" label="上报时间" width="180" align="center">
              <template #default="{ row }">
                {{ formatDate(row.ts) }}
              </template>
            </el-table-column>
            <el-table-column v-for="tag in availableTags" :key="tag" :label="getTagName(tag)" width="120" align="center">
              <template #default="{ row }">
                <span v-if="row.data && row.data[tag] !== undefined">{{ row.data[tag] }} {{ getUnit(tag) }}</span>
                <span v-else style="color: #909399;">-</span>
              </template>
            </el-table-column>
          </el-table>
          <div v-if="historyData.length === 0 && !historyLoading" style="text-align: center; padding: 40px; color: #909399;">
            暂无数据
          </div>
          <div v-if="historyTotal > historyPageSize" style="display: flex; justify-content: center; margin-top: 16px;">
            <el-pagination
              v-model:current-page="historyCurrentPage"
              :page-size="historyPageSize"
              :total="historyTotal"
              layout="prev, pager, next"
              @current-change="handleHistoryPageChange"
            />
          </div>
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover" class="stat-card" style="margin-bottom: 20px; border-radius: 12px;">
          <template #header>
            <span style="font-weight: 600; color: #1f2329;">统计信息</span>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="最大值">
              <span class="stat-highlight">{{ stats.max?.toFixed(2) || '-' }}</span>
              <span style="color: #8c8c8c; margin-left: 8px;">{{ getUnit(selectedTag) }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="最小值">
              <span class="stat-highlight" style="color: #52c41a;">{{ stats.min?.toFixed(2) || '-' }}</span>
              <span style="color: #8c8c8c; margin-left: 8px;">{{ getUnit(selectedTag) }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="平均值">
              <span class="stat-highlight" style="color: #722ed1;">{{ stats.avg?.toFixed(2) || '-' }}</span>
              <span style="color: #8c8c8c; margin-left: 8px;">{{ getUnit(selectedTag) }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="数据点数">
              <span style="font-size: 16px; font-weight: 600; color: #1f2329;">{{ stats.count || 0 }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
        <el-card shadow="hover" class="device-info-card" style="border-radius: 12px;">
          <template #header>
            <span style="font-weight: 600; color: #1f2329;">设备信息</span>
          </template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="设备状态">
              <el-tag :type="deviceStatusType">{{ deviceStatus }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="设备版本">{{ deviceVersion }}</el-descriptions-item>
            <el-descriptions-item label="初次上线">{{ firstOnlineTime }}</el-descriptions-item>
            <el-descriptions-item label="最后活跃">{{ lastActiveTime }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>

  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowLeft, Refresh, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { getDeviceData, getDeviceTags, getDevice, type DeviceDataPoint, type DeviceTag, type Device } from '@/api/device'
import { getProjectTags, type ProjectTag } from '@/api/project'

const route = useRoute()
const deviceId = String(route.params.deviceId)

const dataPoints = ref<DeviceDataPoint[]>([])
const deviceTags = ref<DeviceTag[]>([])
const projectTags = ref<ProjectTag[]>([])
const deviceInfo = ref<Device | null>(null)
const loading = ref(false)
const autoRefresh = ref(true)
const timeRange = ref('1h')
const selectedTag = ref('')

// 根据设备标签生成可用的标签列表，如果没有配置则使用项目标签，再没有则使用默认标签
const availableTags = computed(() => {
  if (deviceTags.value.length > 0) {
    // 优先使用设备标签配置
    return deviceTags.value.map(dt => dt.tag_key)
  } else if (projectTags.value.length > 0) {
    // 其次使用项目标签
    return projectTags.value.map(pt => pt.tag_key)
  } else {
    // 最后使用默认标签
    return ['temperature', 'humidity', 'voltage', 'current']
  }
})

// 计算设备状态
const deviceStatus = computed(() => {
  if (!deviceInfo.value) return '未知'
  switch (deviceInfo.value.status) {
    case 1: return '在线'
    case 0: return '离线'
    case 2: return '未激活'
    default: return '未知'
  }
})

// 计算设备状态类型
const deviceStatusType = computed(() => {
  if (!deviceInfo.value) return 'info'
  switch (deviceInfo.value.status) {
    case 1: return 'success'
    case 0: return 'danger'
    case 2: return 'warning'
    default: return 'info'
  }
})

// 计算最后活跃时间（最新数据的时间）
const lastActiveTime = computed(() => {
  if (dataPoints.value.length === 0) return '-'
  const latestData = dataPoints.value[0]
  return formatDate(latestData.ts)
})

// 计算设备版本（从最新数据中获取）
const deviceVersion = computed(() => {
  if (dataPoints.value.length === 0) return '-'
  // 后端按 ts desc 排序，索引 0 是最新数据
  const latestData = dataPoints.value[0]
  return latestData.version || '-'
})

// 计算初次上线时间（优先使用设备端上报的首次开机时间，否则从历史数据推断）
const firstOnlineTime = computed(() => {
  // 优先使用设备端上报的首次开机时间
  if (deviceInfo.value && deviceInfo.value.first_ts > 0) {
    return formatDate(deviceInfo.value.first_ts)
  }
  // 否则从历史数据推断
  if (dataPoints.value.length === 0) return '-'
  // 后端按 ts desc 排序，最后一个元素是最旧数据
  const oldestData = dataPoints.value[dataPoints.value.length - 1]
  return formatDate(oldestData.ts)
})

const tooltipVisible = ref(false)
const tooltipData = ref({ time: '', value: '' })
const tooltipStyle = ref({})

const stats = ref({
  max: null as number | null,
  min: null as number | null,
  avg: null as number | null,
  count: 0
})

let timer: ReturnType<typeof setInterval> | null = null

function getUnit(tag: string): string {
  // 优先从项目标签获取单位
  const pt = projectTags.value.find(p => p.tag_key === tag)
  if (pt && pt.unit) {
    return pt.unit
  }
  // 使用默认单位映射
  const units: Record<string, string> = {
    temperature: '°C',
    humidity: '%',
    voltage: 'V',
    current: 'A'
  }
  return units[tag] || ''
}

function getTagName(tag: string): string {
  // 优先从项目标签获取名称
  const pt = projectTags.value.find(p => p.tag_key === tag)
  if (pt && pt.tag_name) {
    return pt.tag_name
  }
  // 使用默认名称映射
  const names: Record<string, string> = {
    temperature: '温度',
    humidity: '湿度',
    voltage: '电压',
    current: '电流'
  }
  return names[tag] || tag
}

function formatTs(ts: number): string {
  const d = new Date(ts * 1000)
  return d.toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

function formatDate(ts: number): string {
  const d = new Date(ts * 1000)
  return d.toLocaleString('zh-CN')
}

async function fetchTags() {
  try {
    // 获取设备标签
    const deviceRes = await getDeviceTags(deviceId)
    deviceTags.value = deviceRes.data || []

    // 获取设备信息以获取项目ID
    const deviceInfoRes = await getDevice(deviceId)
    deviceInfo.value = deviceInfoRes.data
    if (deviceInfo.value && deviceInfo.value.project_id) {
      // 获取项目标签
      const projectId = deviceInfo.value.project_id
      const projectRes = await getProjectTags(projectId)
      projectTags.value = projectRes.data || []
    }

    // 设置默认选中的标签
    if (availableTags.value.length > 0 && !selectedTag.value) {
      selectedTag.value = availableTags.value[0]
    }
  } catch {
    // 如果获取失败，使用默认标签
    if (!selectedTag.value) {
      selectedTag.value = 'temperature'
    }
  }
}

async function fetchData() {
  loading.value = true
  try {
    const res = await getDeviceData(deviceId, 100, 0)
    dataPoints.value = res.data?.list || []
    calculateStats()
  } catch {
  } finally {
    loading.value = false
  }
}

function calculateStats() {
  const values = getSelectedTagValues()
  if (values.length === 0) {
    stats.value = { max: null, min: null, avg: null, count: 0 }
    return
  }
  stats.value = {
    max: Math.max(...values),
    min: Math.min(...values),
    avg: values.reduce((a, b) => a + b, 0) / values.length,
    count: values.length
  }
}

function getSelectedTagValues(): number[] {
  return dataPoints.value
    .map(p => {
      const data = typeof p.data === 'string' ? JSON.parse(p.data) : p.data
      return data[selectedTag.value]
    })
    .filter(v => typeof v === 'number')
}

const chartPoints = computed(() => {
  const values = getSelectedTagValues()
  if (values.length === 0) return []
  
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  
  return dataPoints.value
    .map((p, index) => {
      const data = typeof p.data === 'string' ? JSON.parse(p.data) : p.data
      const value = data[selectedTag.value]
      if (typeof value !== 'number') return null
      
      return {
        x: 60 + (index / (dataPoints.value.length - 1)) * 740,
        y: 240 - ((value - min) / range) * 200,
        value,
        ts: p.ts
      }
    })
    .filter(p => p !== null)
})

const chartPath = computed(() => {
  if (chartPoints.value.length < 2) return ''
  return chartPoints.value
    .map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`)
    .join(' ')
})

const areaPath = computed(() => {
  if (chartPoints.value.length < 2) return ''
  const linePath = chartPoints.value
    .map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`)
    .join(' ')
  const first = chartPoints.value[0]
  const last = chartPoints.value[chartPoints.value.length - 1]
  return `${linePath} L ${last.x} 240 L ${first.x} 240 Z`
})

const yAxisLabels = computed(() => {
  const values = getSelectedTagValues()
  if (values.length === 0) return ['0', '0', '0', '0', '0']
  
  const min = Math.min(...values)
  const max = Math.max(...values)
  const range = max - min || 1
  
  return [0, 1, 2, 3, 4].map(i => {
    const value = max - (i / 4) * range
    return value.toFixed(1)
  })
})

const xAxisLabels = computed(() => {
  if (dataPoints.value.length === 0) return []
  
  const step = Math.max(1, Math.floor(dataPoints.value.length / 5))
  return dataPoints.value
    .filter((_, i) => i % step === 0 || i === dataPoints.value.length - 1)
    .map(p => formatTs(p.ts))
})

const realtimeValues = ref<Record<string, number>>({})
const prevValues = ref<Record<string, number>>({})

function getRealtimeValue(tag: string): string {
  const values = dataPoints.value
    .map(p => {
      const data = typeof p.data === 'string' ? JSON.parse(p.data) : p.data
      return data[tag]
    })
    .filter(v => typeof v === 'number')
  
  if (values.length === 0) return '-'
  
  const latest = values[values.length - 1]
  realtimeValues.value[tag] = latest
  
  if (prevValues.value[tag] === undefined) {
    prevValues.value[tag] = latest
  }
  
  return latest.toFixed(2)
}

function getTrend(tag: string): number {
  const current = realtimeValues.value[tag]
  const prev = prevValues.value[tag]
  if (current === undefined || prev === undefined) return 0
  return current - prev
}

function getTrendClass(tag: string): string {
  const trend = getTrend(tag)
  if (trend > 0) return 'trend-up'
  if (trend < 0) return 'trend-down'
  return 'trend-neutral'
}

function getTrendText(tag: string): string {
  const trend = getTrend(tag)
  if (trend > 0) return `+${trend.toFixed(2)}`
  if (trend < 0) return trend.toFixed(2)
  return '0'
}

function showTooltip(point: { value: number; ts: number }, event: MouseEvent) {
  tooltipData.value = {
    time: formatDate(point.ts),
    value: point.value.toFixed(2)
  }
  tooltipStyle.value = {
    left: `${event.clientX + 10}px`,
    top: `${event.clientY + 10}px`
  }
  tooltipVisible.value = true
}

function hideTooltip() {
  tooltipVisible.value = false
}

function startPolling() {
  stopPolling()
  timer = setInterval(() => {
    fetchData()
    // 只在第一页时刷新历史数据，避免频繁翻页时重复请求
    if (historyCurrentPage.value === 1) {
      fetchHistoryData()
    }
  }, 5000)
}

// 历史数据相关
const historyData = ref<DeviceDataPoint[]>([])
const historyTotal = ref(0)
const historyCurrentPage = ref(1)
const historyPageSize = 20
const historyLoading = ref(false)

function formatHistoryTs(ts: number): string {
  const d = new Date(ts * 1000)
  return d.toLocaleString('zh-CN')
}

async function fetchHistoryData() {
  historyLoading.value = true
  try {
    const offset = (historyCurrentPage.value - 1) * historyPageSize
    const res = await getDeviceData(deviceId, historyPageSize, offset)
    historyData.value = res.data?.list || []
    historyTotal.value = res.data?.total || 0
  } catch {
  } finally {
    historyLoading.value = false
  }
}

function handleHistoryPageChange(page: number) {
  historyCurrentPage.value = page
  fetchHistoryData()
}

function stopPolling() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

watch(autoRefresh, (val) => {
  if (val) {
    startPolling()
  } else {
    stopPolling()
  }
})

watch(selectedTag, () => {
  calculateStats()
})

onMounted(async () => {
  await fetchTags()
  fetchData()
  fetchHistoryData()
  if (autoRefresh.value) {
    startPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.page-container {
  padding: 20px;
  min-height: 100vh;
  background: linear-gradient(135deg, #f5f7fa 0%, #e4e8ec 100%);
}

.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);
  margin-bottom: 20px;
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.toolbar-left h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #1f2329;
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.chart-container {
  position: relative;
  width: 100%;
  height: 320px;
}

.line-chart {
  width: 100%;
  height: 100%;
}

.axis-label {
  font-size: 11px;
  fill: #8c8c8c;
}

.data-point {
  cursor: pointer;
  transition: all 0.2s ease;
}

.data-point:hover {
  r: 6;
  filter: drop-shadow(0 0 6px rgba(24, 144, 255, 0.5));
}

.tooltip {
  position: fixed;
  background: linear-gradient(135deg, #1f2329 0%, #303744 100%);
  color: #fff;
  padding: 12px 16px;
  border-radius: 10px;
  font-size: 13px;
  z-index: 1000;
  pointer-events: none;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.tooltip-time {
  margin-bottom: 8px;
  opacity: 0.7;
  font-size: 12px;
}

.tooltip-value {
  font-size: 18px;
  font-weight: 600;
}

.realtime-card-container {
  height: 420px;
}

.realtime-card-container :deep(.el-card__body) {
  padding: 16px;
  height: calc(100% - 57px);
  overflow: hidden;
}

.realtime-cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  height: 100%;
  overflow-y: auto;
  padding-right: 8px;
  align-content: start;
}

.realtime-cards::-webkit-scrollbar {
  width: 4px;
}

.realtime-cards::-webkit-scrollbar-track {
  background: transparent;
}

.realtime-cards::-webkit-scrollbar-thumb {
  background: #d9d9d9;
  border-radius: 2px;
}

.realtime-cards::-webkit-scrollbar-thumb:hover {
  background: #bfbfbf;
}

.realtime-card {
  background: linear-gradient(145deg, #ffffff 0%, #f8f9fa 100%);
  height: 64px;
  padding: 10px 12px;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
  border: 1px solid #f0f0f0;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
  box-sizing: border-box;
}

.realtime-card::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  background: linear-gradient(180deg, #1890ff 0%, #69c0ff 100%);
}

.realtime-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.1);
}

.realtime-content {
  display: grid;
  grid-template-columns: minmax(36px, 52px) minmax(0, 1fr) auto;
  grid-template-areas:
    "label value unit"
    "label trend trend";
  align-items: center;
  column-gap: 6px;
  row-gap: 4px;
  height: 100%;
}

.realtime-label {
  grid-area: label;
  font-size: 12px;
  color: #646a73;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.realtime-value {
  grid-area: value;
  font-size: 18px;
  font-weight: 700;
  color: #1f2329;
  font-family: 'SF Mono', 'Monaco', 'Inconsolata', monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.realtime-unit {
  grid-area: unit;
  font-size: 12px;
  color: #8c8c8c;
  font-weight: 500;
  white-space: nowrap;
}

.realtime-trend {
  grid-area: trend;
  font-size: 11px;
  display: flex;
  align-items: center;
  gap: 2px;
  font-weight: 500;
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.05);
  width: fit-content;
  max-width: 100%;
  overflow: hidden;
  white-space: nowrap;
}

.realtime-badge {
  font-size: 10px;
  color: #fff;
  background: linear-gradient(135deg, #52c41a 0%, #389e0d 100%);
  padding: 2px 6px;
  border-radius: 10px;
  font-weight: 500;
  margin-left: auto;
}

.trend-up {
  color: #52c41a;
}

.trend-down {
  color: #f56c6c;
}

.trend-neutral {
  color: #8c8c8c;
}

.stat-highlight {
  font-size: 20px;
  font-weight: 700;
  color: #1890ff;
}

.table-card {
  border-radius: 12px;
  overflow: hidden;
  background: #fff;
}

.table-card :deep(.el-card__header) {
  background: #fff;
  border-bottom: 1px solid #e8e8e8;
}

.table-card :deep(.el-card__body) {
  background: #fff;
}

.table-card :deep(.el-table) {
  border-radius: 0;
  background: #fff;
}

.table-card :deep(.el-table th) {
  background: #fff;
  font-weight: 600;
  color: #434e59;
  border-bottom: 1px solid #e8e8e8;
}

.table-card :deep(.el-table td) {
  background: #fff;
  border-bottom: 1px solid #f5f5f5;
}

.stat-card :deep(.el-card__body) {
  padding: 16px;
}

.stat-card :deep(.el-descriptions-item__label) {
  color: #646a73;
  font-weight: 500;
}

.stat-card :deep(.el-descriptions-item__content) {
  color: #1f2329;
}

.device-info-card :deep(.el-card__body) {
  padding: 16px;
}

.device-info-card :deep(.el-tag) {
  font-weight: 500;
}
</style>