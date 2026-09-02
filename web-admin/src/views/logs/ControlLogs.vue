<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>控制日志</h2>
        <el-input
          v-model="filterDeviceId"
          placeholder="按设备ID筛选"
          clearable
          style="width: 240px; margin-left: 12px;"
          @clear="fetchLogs"
          @keyup.enter="fetchLogs"
        />
        <el-button type="primary" style="margin-left: 8px;" @click="fetchLogs">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-switch
          v-model="autoRefresh"
          active-text="自动刷新"
          inactive-text="手动刷新"
          style="margin-right: 12px;"
        />
        <el-button @click="fetchLogs" :loading="loading">刷新</el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="logs" v-loading="loading" stripe border max-height="calc(100vh - 260px)">
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="device_id" label="设备ID" width="160" />
        <el-table-column prop="msg_id" label="消息ID" width="140" show-overflow-tooltip />
        <el-table-column label="控制标签" min-width="200">
          <template #default="{ row }">
            <pre class="tags-pre">{{ formatTags(row.tags) }}</pre>
          </template>
        </el-table-column>
        <el-table-column label="响应码" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.ack_code === 0 ? 'success' : 'danger'" size="small">
              {{ row.ack_code }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ack_msg" label="响应消息" width="160" show-overflow-tooltip />
        <el-table-column prop="created_at" label="创建时间" width="180" align="center" />
      </el-table>
      <div v-if="logs.length === 0 && !loading" style="text-align: center; padding: 40px; color: #909399;">
        暂无控制日志
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
import { getControlLogs, type ControlLog } from '@/api/firmware'

const logs = ref<ControlLog[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = 50
const loading = ref(false)
const autoRefresh = ref(false)
const filterDeviceId = ref('')

let timer: ReturnType<typeof setInterval> | null = null

function formatTags(tags: Record<string, unknown>): string {
  return JSON.stringify(tags, null, 2)
}

async function fetchLogs() {
  loading.value = true
  try {
    const offset = (currentPage.value - 1) * pageSize
    const res = await getControlLogs(filterDeviceId.value || undefined, pageSize, offset)
    logs.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch {
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

function startPolling() {
  stopPolling()
  timer = setInterval(fetchLogs, 5000)
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
  fetchLogs()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
.tags-pre {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 120px;
  overflow-y: auto;
}
</style>
