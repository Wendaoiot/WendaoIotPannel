<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-select
          v-model="filterProjectId"
          placeholder="按项目筛选"
          clearable
          style="width: 200px"
          @change="fetchDevices"
        >
          <el-option
            v-for="p in projects"
            :key="p.id"
            :label="p.name"
            :value="p.id"
          />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="showCreateDialog">
          <el-icon><Plus /></el-icon>
          新建设备
        </el-button>
      </div>
    </div>

    <div v-loading="loading" class="card-grid-wrap">
      <el-row v-if="devices.length" :gutter="16">
        <el-col v-for="dev in devices" :key="dev.id" :xs="24" :sm="12" :md="8" :lg="6">
          <article class="device-card" role="button" tabindex="0" @click="goDetail(dev)" @keydown.enter="goDetail(dev)">
            <div class="card-top">
              <span class="status-dot" :class="dotClass(dev)" :title="statusText(dev)" />
              <el-tag v-if="modeLabel(dev)" size="small" type="info" effect="plain">{{ modeLabel(dev) }}</el-tag>
            </div>
            <div class="card-body">
              <h3 class="card-name" :title="dev.name">{{ dev.name }}</h3>
              <p class="card-id mono" :title="dev.id">{{ dev.id }}</p>
            </div>
            <div class="card-meta">
              <span class="meta-item" :title="getProjectName(dev.project_id)">
                <el-icon><Folder /></el-icon>
                {{ getProjectName(dev.project_id) }}
              </span>
              <span class="meta-item">
                <el-icon><Clock /></el-icon>
                {{ relativeActive(dev.last_active) }}
              </span>
            </div>
          </article>
        </el-col>
      </el-row>
      <el-empty v-else-if="!loading" description="暂无设备，点击右上角新建" />
    </div>

    <el-dialog v-model="createDialogVisible" title="新建设备" width="450px" @closed="resetCreateForm">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="80px">
        <el-form-item label="所属项目" prop="project_id">
          <el-select v-model="createForm.project_id" placeholder="请选择项目" style="width: 100%">
            <el-option
              v-for="p in projects"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="设备ID" prop="id">
          <el-input v-model="createForm.id" placeholder="请输入设备唯一ID" />
        </el-form-item>
        <el-form-item label="设备名称" prop="name">
          <el-input v-model="createForm.name" placeholder="请输入设备名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreate">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, Folder, Clock } from '@element-plus/icons-vue'
import { getDevices, createDevice, type Device } from '@/api/device'
import { getProjects, type Project } from '@/api/project'
import { useAuthStore } from '@/stores/auth'
import { normalizeOnlineMode, formatTimeoutDuration } from '@/utils/onlineMode'
import { useRealtime } from '@/composables/useRealtime'

const router = useRouter()
const authStore = useAuthStore()
const isTenantAdmin = computed(() => authStore.role === 'tenant_admin')
const tenantId = computed(() => authStore.tenantId)
const devices = ref<Device[]>([])
const projects = ref<Project[]>([])
const loading = ref(false)
const submitting = ref(false)
const filterProjectId = ref<number | undefined>(undefined)

const createDialogVisible = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  id: '',
  project_id: undefined as number | undefined,
  name: ''
})
const createRules: FormRules = {
  project_id: [{ required: true, message: '请选择项目', trigger: 'change' }],
  id: [{ required: true, message: '请输入设备ID', trigger: 'blur' }],
  name: [{ required: true, message: '请输入设备名称', trigger: 'blur' }]
}

function getProjectName(pid: number): string {
  const p = projects.value.find(item => item.id === pid)
  return p?.name || `项目${pid}`
}

function statusText(dev: Device): string {
  if (dev.enabled === false || dev.status === 2) return '已禁用'
  return dev.status === 1 ? '在线' : '离线'
}

function dotClass(dev: Device): string {
  if (dev.enabled === false || dev.status === 2) return 'dot-disabled'
  return dev.status === 1 ? 'dot-online' : 'dot-offline'
}

function modeLabel(dev: Device): string {
  // 默认「仅按连接」不在卡片上重复展示，仅标注非默认判定方式
  const mode = normalizeOnlineMode(dev.online_mode)
  if (mode === 'connection') return ''
  const t = dev.offline_timeout_sec || 0
  const label = mode === 'report' ? '按上报时间' : '按应答信号'
  return t > 0 ? `${label} · 时限${formatTimeoutDuration(t)}` : label
}

function relativeActive(lastActive: string | null): string {
  if (!lastActive) return '从未活跃'
  const ms = new Date(lastActive).getTime()
  if (Number.isNaN(ms)) return '—'
  const diff = Date.now() - ms
  if (diff < 60_000) return '刚刚活跃'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  return `${Math.floor(diff / 86_400_000)} 天前`
}

async function fetchDevices() {
  loading.value = true
  try {
    const res = await getDevices(filterProjectId.value)
    devices.value = res.data || []
  } catch {
  } finally {
    loading.value = false
  }
}

async function fetchProjects() {
  try {
    const res = await getProjects(isTenantAdmin.value ? tenantId.value : undefined)
    projects.value = res.data || []
  } catch {
  }
}

function goDetail(dev: Device) {
  router.push(`/devices/${dev.id}`)
}

function showCreateDialog() {
  createDialogVisible.value = true
}

function resetCreateForm() {
  createForm.id = ''
  createForm.project_id = undefined
  createForm.name = ''
  createFormRef.value?.resetFields()
}

async function handleCreate() {
  const valid = await createFormRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const res = await createDevice({
      id: createForm.id,
      project_id: createForm.project_id!,
      name: createForm.name
    })
    createDialogVisible.value = false
    const secret = res.data?.device_secret
    if (secret) {
      ElMessageBox.alert(
        `<div style="line-height:1.9;font-size:13px">`
        + `接入用户名（设备ID）：<b>${createForm.id}</b><br/>`
        + `接入密码（密钥）：<b>${secret}</b><br/>`
        + `Broker：tcp://127.0.0.1:1883`
        + `</div><br/>用户名严格区分大小写；该密钥<b>仅此一次显示</b>，请妥善保存并配置到设备。`,
        '设备接入凭证', { dangerouslyUseHTMLString: true, confirmButtonText: '我已保存', type: 'success' }
      )
    } else {
      ElMessage.success('设备创建成功')
    }
    fetchDevices()
  } catch {
  } finally {
    submitting.value = false
  }
}

// WS 实时上下线：本地即时更新卡片状态，无需手动刷新
const { onMessage } = useRealtime()
let offRealtime: (() => void) | null = null

onMounted(() => {
  fetchProjects()
  fetchDevices()
  offRealtime = onMessage((msg) => {
    if (msg.type !== 'device_status') return
    const id = String(msg.data?.device_id ?? '')
    const online = Boolean(msg.data?.online)
    const idx = devices.value.findIndex(d => d.id === id)
    if (idx >= 0) {
      const cur = devices.value[idx]
      if (msg.data?.enabled === false) {
        devices.value[idx] = { ...cur, enabled: false, status: 2 }
      } else if (online) {
        devices.value[idx] = { ...cur, enabled: true, status: 1 }
      } else {
        devices.value[idx] = { ...cur, status: 0 }
      }
    }
  })
})

onUnmounted(() => {
  offRealtime?.()
  offRealtime = null
})
</script>

<style scoped>
.card-grid-wrap {
  min-height: 200px;
}

.card-grid-wrap :deep(.el-row) {
  row-gap: 16px;
}

.device-card {
  height: 100%;
  padding: 16px;
  background: var(--wd-surface);
  border: 1px solid var(--wd-border);
  border-radius: var(--wd-radius-lg);
  box-shadow: var(--wd-shadow-sm);
  cursor: pointer;
  transition: box-shadow var(--wd-dur) var(--wd-ease), transform var(--wd-dur) var(--wd-ease), border-color var(--wd-dur) var(--wd-ease);
  outline: none;
}

.device-card:hover,
.device-card:focus-visible {
  transform: translateY(-2px);
  border-color: var(--wd-primary-light);
  box-shadow: var(--wd-shadow-md);
}

.card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-online {
  background: var(--wd-success);
  box-shadow: 0 0 0 3px var(--wd-success-bg);
}

.dot-offline {
  background: var(--wd-text-placeholder);
  box-shadow: 0 0 0 3px var(--wd-border-lighter);
}

.dot-disabled {
  background: var(--wd-danger);
  box-shadow: 0 0 0 3px var(--wd-danger-bg);
}

.card-body {
  margin-bottom: 14px;
}

.card-name {
  margin: 0 0 6px;
  font-size: 16px;
  font-weight: 600;
  color: var(--wd-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-id {
  margin: 0;
  font-size: 12px;
  color: var(--wd-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono {
  font-family: 'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace;
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--wd-border-lighter);
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  font-size: 12px;
  color: var(--wd-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.meta-item .el-icon {
  flex-shrink: 0;
}
</style>
