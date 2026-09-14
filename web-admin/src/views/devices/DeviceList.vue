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
        <!-- 一型一密「录入设备」入口：后端已上线，前端暂隐；联调收尾后置 true 放开（对话框与逻辑仍保留） -->
        <el-button v-if="preregisterEnabled" type="success" plain @click="showPreregisterDialog">
          <el-icon><Connection /></el-icon>
          录入设备
        </el-button>
        <el-button type="primary" @click="showCreateDialog">
          <el-icon><Plus /></el-icon>
          新建设备
        </el-button>
      </div>
    </div>

    <div v-loading="loading" class="card-grid-wrap">
      <el-row v-if="devices.length" :gutter="16">
        <el-col v-for="dev in devices" :key="dev.id" :xs="24" :sm="12" :md="8" :lg="6">
          <article
            class="device-card"
            :class="`is-${deviceState(dev)}`"
            role="button"
            tabindex="0"
            @click="goDetail(dev)"
            @keydown.enter="goDetail(dev)"
          >
            <span class="card-strip" :title="statusText(dev)" />
            <div class="card-inner">
              <div class="card-head">
                <span class="device-avatar"><el-icon><Cpu /></el-icon></span>
                <!-- 待激活：状态胶囊本身即橙色提示，不重复 tag；已激活产品设备在胶囊左侧放产品名 -->
                <span v-if="!isPending(dev) && isDynreg(dev)" class="mini-tag tag-product" :title="productName(dev.product_id!)">
                  {{ productName(dev.product_id!) }}
                </span>
                <span class="status-pill">{{ statusText(dev) }}</span>
              </div>
              <div class="card-body">
                <h3 class="card-name" :title="dev.name">{{ dev.name }}</h3>
                <p class="card-id mono" :title="dev.id">{{ dev.id }}</p>
              </div>
              <div class="card-meta">
                <span class="meta-item" :title="getProjectName(dev.project_id)">
                  <el-icon><Folder /></el-icon>{{ getProjectName(dev.project_id) }}
                </span>
                <span class="meta-item">
                  <el-icon><Clock /></el-icon>{{ isPending(dev) ? '等待首次上线' : relativeActive(dev.last_active) }}
                </span>
                <span v-if="modeLabel(dev)" class="meta-item meta-mode">{{ modeLabel(dev) }}</span>
              </div>
            </div>
          </article>
        </el-col>
      </el-row>
      <el-empty v-else-if="!loading" description="暂无设备，点击右上角新建" />
    </div>

    <el-dialog v-if="preregisterEnabled" v-model="preregDialogVisible" title="录入设备（一型一密）" width="460px" @closed="resetPreregForm">
      <el-alert
        type="info"
        :closable="false"
        style="margin-bottom: 16px"
        title="录入后设备为待激活状态，无需人工分发密钥：设备首次以产品凭证经 mqtts://8883 引导连接时，自动换取一机一密。"
      />
      <el-form ref="preregFormRef" :model="preregForm" :rules="preregRules" label-width="80px">
        <el-form-item label="所属项目" prop="project_id">
          <el-select v-model="preregForm.project_id" placeholder="请选择项目" style="width: 100%">
            <el-option v-for="p in projects" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="所属产品" prop="product_key">
          <el-select
            v-model="preregForm.product_key"
            placeholder="请选择产品（同租户）"
            style="width: 100%"
          >
            <el-option
              v-for="p in filteredProducts"
              :key="p.product_key"
              :label="`${p.name}（${p.product_key}）`"
              :value="p.product_key"
            />
          </el-select>
          <div v-if="preregForm.project_id && filteredProducts.length === 0" class="form-hint">
            该项目所属租户下还没有产品，
            <router-link to="/products">前往「产品管理」创建</router-link>
          </div>
        </el-form-item>
        <el-form-item label="SN 码" prop="sn">
          <el-input v-model="preregForm.sn" placeholder="设备出厂序列号（字母/数字/_/-，≤64，区分大小写）" />
        </el-form-item>
        <el-form-item label="设备名称" prop="name">
          <el-input v-model="preregForm.name" placeholder="选填，留空则与 SN 相同" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="preregDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="preregSubmitting" @click="handlePreregister">录入</el-button>
      </template>
    </el-dialog>

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
import { Plus, Folder, Clock, Connection } from '@element-plus/icons-vue'
import { getDevices, createDevice, preregisterDevice, type Device } from '@/api/device'
import { getProjects, type Project } from '@/api/project'
import { getProducts, type Product } from '@/api/product'
import { useAuthStore } from '@/stores/auth'
import { resolveOnlineMode, formatTimeoutDuration } from '@/utils/onlineMode'
import { useRealtime } from '@/composables/useRealtime'

const router = useRouter()
const authStore = useAuthStore()
const isTenantAdmin = computed(() => authStore.role === 'tenant_admin')
const tenantId = computed(() => authStore.tenantId)
const devices = ref<Device[]>([])
const projects = ref<Project[]>([])
const products = ref<Product[]>([])
const loading = ref(false)
const submitting = ref(false)
const filterProjectId = ref<number | undefined>(undefined)

// 一型一密前端入口开关：后端（产品管理/动态注册）已上线，「录入设备」入口暂隐，置 true 即恢复
const preregisterEnabled = false

// 一型一密：产品设备识别（product_id>0；activated_at 为空=待激活）
function isDynreg(dev: Device): boolean {
  return (dev.product_id ?? 0) > 0
}
function isPending(dev: Device): boolean {
  return isDynreg(dev) && !dev.activated_at
}
function productName(pid: number): string {
  return products.value.find(p => p.id === pid)?.name || `产品${pid}`
}

// 卡片统一状态：disabled > pending > online > offline（驱动色条/头像配色）
function deviceState(dev: Device): 'disabled' | 'pending' | 'online' | 'offline' {
  if (dev.enabled === false || dev.status === 2) return 'disabled'
  if (isPending(dev)) return 'pending'
  return dev.status === 1 ? 'online' : 'offline'
}

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

// ---- 一型一密：单设备录入 ----
const preregDialogVisible = ref(false)
const preregSubmitting = ref(false)
const preregFormRef = ref<FormInstance>()
const preregForm = reactive({
  project_id: undefined as number | undefined,
  product_key: '',
  sn: '',
  name: ''
})
const snPattern = /^[A-Za-z0-9_-]{1,64}$/
const preregRules: FormRules = {
  project_id: [{ required: true, message: '请选择项目', trigger: 'change' }],
  product_key: [{ required: true, message: '请选择产品', trigger: 'change' }],
  sn: [
    { required: true, message: '请输入 SN 码', trigger: 'blur' },
    { pattern: snPattern, message: '1-64 位字母/数字/下划线/短横线，区分大小写', trigger: 'blur' }
  ]
}

// 产品下拉只列所选项目同租户的产品（产品与项目须同租户）
const filteredProducts = computed<Product[]>(() => {
  if (preregForm.project_id === undefined) return []
  const proj = projects.value.find(p => p.id === preregForm.project_id)
  if (!proj) return []
  return products.value.filter(p => p.tenant_id === proj.tenant_id)
})

function getProjectName(pid: number): string {
  const p = projects.value.find(item => item.id === pid)
  return p?.name || `项目${pid}`
}

function statusText(dev: Device): string {
  if (dev.enabled === false || dev.status === 2) return '已禁用'
  if (isPending(dev)) return '待激活'
  return dev.status === 1 ? '在线' : '离线'
}

function modeLabel(dev: Device): string {
  // 生效模式 = 设备显式覆盖 -> 项目默认 -> 系统(connection)；仅按连接不在卡片重复展示
  const proj = projects.value.find(p => p.id === dev.project_id)
  const mode = resolveOnlineMode(dev.online_mode, proj?.online_mode)
  if (mode === 'connection') return ''
  const devTimeout = dev.offline_timeout_sec ?? 0
  const projTimeout = proj?.offline_timeout_sec ?? 0
  const t: number = devTimeout > 0 ? devTimeout : projTimeout
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

async function fetchProducts() {
  try {
    const res = await getProducts(isTenantAdmin.value ? tenantId.value : undefined)
    products.value = res.data || []
  } catch {
  }
}

function showPreregisterDialog() {
  preregForm.project_id = filterProjectId.value
  preregDialogVisible.value = true
}

function resetPreregForm() {
  preregForm.project_id = undefined
  preregForm.product_key = ''
  preregForm.sn = ''
  preregForm.name = ''
  preregFormRef.value?.resetFields()
}

async function handlePreregister() {
  const valid = await preregFormRef.value?.validate().catch(() => false)
  if (!valid) return
  preregSubmitting.value = true
  try {
    const res = await preregisterDevice({
      product_key: preregForm.product_key,
      project_id: preregForm.project_id!,
      sn: preregForm.sn.trim(),
      name: preregForm.name.trim()
    })
    const row = res.data.results[0]
    if (!row?.ok) {
      // 逐行业务失败（SN 已存在等）：保留对话框便于修改
      ElMessage.error(row?.msg || '录入失败')
      return
    }
    preregDialogVisible.value = false
    await ElMessageBox.alert(
      `设备 <b>${row.id}</b> 已预录（待激活）。<br/>设备首次使用产品凭证连接 mqtts://8883 时将自动激活，无需人工分发密钥。`,
      '录入成功',
      { dangerouslyUseHTMLString: true, confirmButtonText: '知道了', type: 'success' }
    ).catch(() => undefined)
    fetchDevices()
  } catch {
  } finally {
    preregSubmitting.value = false
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
  fetchProducts()
  fetchDevices()
  offRealtime = onMessage((msg) => {
    const id = String(msg.data?.device_id ?? '')
    const idx = devices.value.findIndex(d => d.id === id)
    if (idx >= 0 && msg.type === 'device_activated') {
      // 一型一密动态注册完成：待激活 → 已激活
      const cur = devices.value[idx]
      devices.value[idx] = {
        ...cur,
        product_id: Number(msg.data?.product_id ?? cur.product_id ?? 0),
        activated_at: msg.data?.activated_at
          ? new Date(Number(msg.data.activated_at)).toISOString()
          : cur.activated_at
      }
      return
    }
    if (msg.type !== 'device_status') return
    const online = Boolean(msg.data?.online)
    if (idx >= 0) {
      const cur = devices.value[idx]
      if (isPending(cur)) return // 待激活设备不可能有真实连接事件，忽略
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

/* ===== 设备卡片：左侧状态色条 + 图标头像 + 状态胶囊，信息紧凑无空白 ===== */
.device-card {
  position: relative;
  height: 100%;
  min-height: 150px;
  background: var(--wd-surface);
  border: 1px solid var(--wd-border);
  border-radius: var(--wd-radius-lg);
  box-shadow: var(--wd-shadow-sm);
  cursor: pointer;
  overflow: hidden;
  transition: box-shadow var(--wd-dur) var(--wd-ease), transform var(--wd-dur) var(--wd-ease), border-color var(--wd-dur) var(--wd-ease);
  outline: none;
}

.device-card:hover,
.device-card:focus-visible {
  transform: translateY(-2px);
  border-color: var(--wd-primary-light);
  box-shadow: var(--wd-shadow-md);
}

.card-strip {
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 4px;
  background: var(--wd-text-placeholder);
}

.is-online .card-strip { background: var(--wd-success); }
.is-disabled .card-strip { background: var(--wd-danger); }
.is-pending .card-strip { background: var(--wd-warning); }

.card-inner {
  padding: 14px 14px 12px 18px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
}

.card-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.device-avatar {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
  color: var(--wd-text-secondary);
  background: var(--wd-border-lighter);
}

.is-online .device-avatar { color: var(--wd-success); background: var(--wd-success-bg); }
.is-disabled .device-avatar { color: var(--wd-danger); background: var(--wd-danger-bg); }
.is-pending .device-avatar { color: var(--wd-warning); background: var(--wd-warning-bg); }

.status-pill {
  margin-left: auto;
  flex-shrink: 0;
  font-size: 12px;
  line-height: 1;
  padding: 4px 9px;
  border-radius: 999px;
  color: var(--wd-text-secondary);
  background: var(--wd-border-lighter);
  white-space: nowrap;
}

.is-online .status-pill { color: var(--wd-success); background: var(--wd-success-bg); }
.is-disabled .status-pill { color: var(--wd-danger); background: var(--wd-danger-bg); }
.is-pending .status-pill { color: var(--wd-warning); background: var(--wd-warning-bg); }

.mini-tag {
  flex-shrink: 1;
  min-width: 0;
  font-size: 12px;
  line-height: 1;
  padding: 4px 8px;
  border-radius: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tag-pending {
  color: var(--wd-warning);
  background: var(--wd-warning-bg);
  font-weight: 600;
}

.tag-product {
  color: var(--wd-success);
  background: var(--wd-success-bg);
}

.card-body {
  min-width: 0;
}

.card-name {
  margin: 0 0 3px;
  font-size: 15px;
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
  margin-top: auto;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  padding-top: 10px;
  border-top: 1px solid var(--wd-border-lighter);
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  max-width: 100%;
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

.meta-mode {
  color: var(--wd-primary);
}

.form-hint {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--wd-text-secondary);
}

.form-hint a {
  color: var(--wd-primary);
}
</style>
