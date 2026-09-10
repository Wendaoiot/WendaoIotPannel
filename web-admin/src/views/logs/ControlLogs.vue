<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-input
          v-model="filterDeviceId"
          placeholder="按设备ID筛选"
          clearable
          style="width: 220px;"
          @clear="onFilterChange"
          @keyup.enter="onFilterChange"
        />
        <el-select v-model="filterStatus" placeholder="状态" clearable style="width: 140px; margin-left: 8px;" @change="onFilterChange">
          <el-option label="已送达" value="delivered" />
          <el-option label="执行成功" value="success" />
          <el-option label="执行失败" value="failed" />
          <el-option label="超时" value="timeout" />
          <el-option label="下发中" value="pending" />
        </el-select>
        <el-date-picker
          v-model="timeRange"
          type="datetimerange"
          range-separator="至"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          style="margin-left: 8px;"
          @change="onFilterChange"
        />
        <el-button type="primary" style="margin-left: 8px;" @click="onFilterChange">查询</el-button>
      </div>
      <div class="toolbar-right">
        <el-switch v-model="autoRefresh" active-text="自动刷新" inactive-text="手动刷新" style="margin-right: 12px;" />
        <el-button @click="fetchLogs" :loading="loading">刷新</el-button>
        <el-button type="success" :loading="exporting" @click="handleExport">
          <el-icon><Download /></el-icon>&nbsp;导出CSV
        </el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="filtered" v-loading="loading" stripe border max-height="calc(100vh - 300px)">
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="device_id" label="设备ID" width="160" />
        <el-table-column prop="msg_id" label="消息ID" width="150" show-overflow-tooltip />
        <el-table-column label="控制标签" min-width="180">
          <template #default="{ row }">
            <pre class="tags-pre">{{ formatTags(row.tags) }}</pre>
          </template>
        </el-table-column>
        <el-table-column label="执行状态" width="120" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="响应码" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.ack_code != null" :type="row.ack_code === 0 ? 'success' : 'danger'" size="small">
              {{ row.ack_code }}
            </el-tag>
            <span v-else>—</span>
          </template>
        </el-table-column>
        <el-table-column prop="ack_msg" label="响应消息" width="160" show-overflow-tooltip />
        <el-table-column label="创建时间" width="180" align="center">
          <template #default="{ row }">{{ formatTs(row.created_at) }}</template>
        </el-table-column>
      </el-table>
      <div v-if="filtered.length === 0 && !loading" class="empty-tip">
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
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { Download } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getControlLogs, exportControlLogsCsv, type ControlLog, type ControlLogQuery } from '@/api/firmware'
import { formatTs } from '@/utils/datetime'

// 支持从设备控制页跳转时预填设备过滤（/iot/logs/control?device_id=xxx）
const route = useRoute()

const logs = ref<ControlLog[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = 50
const loading = ref(false)
const exporting = ref(false)
const autoRefresh = ref(false)
const filterDeviceId = ref('')
const filterStatus = ref('')
const timeRange = ref<[Date, Date] | null>(null)

let timer: ReturnType<typeof setInterval> | null = null

function formatTags(tags: Record<string, unknown>): string {
  return typeof tags === 'string' ? tags : JSON.stringify(tags, null, 2)
}

function statusType(status?: string) {
  return ({ success: 'success', failed: 'danger', timeout: 'info', delivered: 'warning', pending: 'warning' } as Record<string, 'success' | 'danger' | 'info' | 'warning'>)[status || 'pending']
}
function statusText(status?: string) {
  return ({ success: '执行成功', failed: '执行失败', timeout: '超时', delivered: '已送达', pending: '下发中' } as Record<string, string>)[status || 'pending'] || status || '下发中'
}

function buildQuery(): ControlLogQuery {
  const offset = (currentPage.value - 1) * pageSize
  const q: ControlLogQuery = { device_id: filterDeviceId.value || undefined, limit: pageSize, offset }
  if (timeRange.value && timeRange.value.length === 2) {
    q.start = timeRange.value[0].getTime()
    q.end = timeRange.value[1].getTime()
  }
  return q
}

async function fetchLogs() {
  loading.value = true
  try {
    const res = await getControlLogs(buildQuery())
    logs.value = res.data?.list || []
    total.value = res.data?.total || 0
  } catch {
  } finally {
    loading.value = false
  }
}

// 状态过滤在前端完成（控制日志量可控；后端已按设备/时间/租户过滤）
const filtered = computed(() => {
  if (!filterStatus.value) return logs.value
  return logs.value.filter(l => l.status === filterStatus.value)
})

function onFilterChange() {
  currentPage.value = 1
  fetchLogs()
}

function handlePageChange(page: number) {
  currentPage.value = page
  fetchLogs()
}

async function handleExport() {
  exporting.value = true
  try {
    const q = buildQuery()
    q.limit = 100000
    q.offset = 0
    await exportControlLogsCsv(q)
    ElMessage.success('导出成功')
  } catch {
  } finally {
    exporting.value = false
  }
}

function startPolling() {
  stopPolling()
  timer = setInterval(fetchLogs, 5000)
}
function stopPolling() {
  if (timer) { clearInterval(timer); timer = null }
}

watch(autoRefresh, (val) => { if (val) startPolling(); else stopPolling() })
onMounted(() => {
  const qDevice = route.query.device_id
  if (typeof qDevice === 'string' && qDevice) {
    filterDeviceId.value = qDevice
    currentPage.value = 1
  }
  fetchLogs()
})
onUnmounted(stopPolling)
</script>

<style scoped>
.tags-pre {
  margin: 0;
  font-family: 'JetBrains Mono', Consolas, monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
