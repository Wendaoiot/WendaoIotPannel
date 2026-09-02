<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="$router.back()">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h2>设备数据 - {{ deviceId }}</h2>
      </div>
      <div class="toolbar-right">
        <el-switch
          v-model="autoRefresh"
          active-text="自动刷新"
          inactive-text="手动刷新"
          style="margin-right: 12px;"
        />
        <el-button :icon="Refresh" @click="fetchData" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="dataPoints" v-loading="loading" stripe border max-height="calc(100vh - 240px)">
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="msg_id" label="消息ID" width="120" show-overflow-tooltip />
        <el-table-column prop="ts" label="时间戳" width="180" align="center">
          <template #default="{ row }">
            {{ formatTs(row.ts) }}
          </template>
        </el-table-column>
        <el-table-column label="数据" min-width="300">
          <template #default="{ row }">
            <pre class="data-pre">{{ formatData(row.data) }}</pre>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="接收时间" width="180" align="center" />
      </el-table>
      <div v-if="dataPoints.length === 0 && !loading" style="text-align: center; padding: 40px; color: #909399;">
        暂无数据
      </div>
      <div v-if="total > pageSize" style="display: flex; justify-content: center; margin-top: 16px;">
        <el-pagination
          v-model:current-page="currentPage"
          :page-size="pageSize"
          :total="total"
          layout="prev, pager, next"
          @current-change="handlePageChange"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import { getDeviceData, type DeviceDataPoint } from '@/api/device'

const route = useRoute()
const deviceId = String(route.params.deviceId)

const dataPoints = ref<DeviceDataPoint[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = 20
const loading = ref(false)
const autoRefresh = ref(true)

let timer: ReturnType<typeof setInterval> | null = null

function formatData(data: Record<string, unknown>): string {
  return JSON.stringify(data, null, 2)
}

function formatTs(ts: number): string {
  const d = new Date(ts * 1000)
  return d.toLocaleString('zh-CN')
}

async function fetchData() {
  loading.value = true
  try {
    const offset = (currentPage.value - 1) * pageSize
    const res = await getDeviceData(deviceId, pageSize, offset)
    dataPoints.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch {
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchData()
}

function startPolling() {
  stopPolling()
  timer = setInterval(fetchData, 3000)
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

onMounted(() => {
  fetchData()
  if (autoRefresh.value) {
    startPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.data-pre {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
}
</style>
