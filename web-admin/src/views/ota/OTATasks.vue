<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left"><h2>OTA 升级</h2></div>
      <div class="toolbar-right">
        <el-button type="primary" @click="showTaskDialog">
          <el-icon><Upload /></el-icon>创建升级任务
        </el-button>
      </div>
    </div>

    <el-card class="table-card" style="margin-bottom: 20px;">
      <el-table :data="tasks" v-loading="loading" stripe border @row-click="selectTask">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="firmware_id" label="固件ID" width="80" />
        <el-table-column label="目标类型" width="100">
          <template #default="{ row }">{{ row.target_type === 'device' ? '单设备' : '项目级' }}</template>
        </el-table-column>
        <el-table-column prop="target_id" label="目标" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="tagType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="170" />
        <el-table-column label="操作" width="80" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="info" @click.stop="selectTask(row)">详情</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-card v-if="selectedTaskId" class="table-card">
      <template #header>升级日志 (任务ID: {{ selectedTaskId }})</template>
      <el-table :data="logs" v-loading="logsLoading" stripe border size="small">
        <el-table-column prop="device_id" label="设备ID" width="160" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="logTagType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="进度" width="120">
          <template #default="{ row }">
            <el-progress :percentage="row.progress" :status="row.status === 'failed' ? 'exception' : undefined" />
          </template>
        </el-table-column>
        <el-table-column prop="error_msg" label="错误信息" show-overflow-tooltip />
        <el-table-column prop="updated_at" label="更新时间" width="170" />
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="创建OTA升级任务" width="500px" @closed="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item label="固件" prop="firmware_id">
          <el-select v-model="form.firmware_id" placeholder="选择固件" style="width: 100%">
            <el-option v-for="fw in firmwares" :key="fw.id" :label="`${fw.name} v${fw.version}`" :value="fw.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标类型" prop="target_type">
          <el-radio-group v-model="form.target_type">
            <el-radio value="device">单设备</el-radio>
            <el-radio value="project">项目(全部设备)</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item :label="form.target_type === 'device' ? '设备ID' : '项目ID'" prop="target_id">
          <el-input v-model="form.target_id" :placeholder="form.target_type === 'device' ? '如: ESP32-001' : '项目ID数字'" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleCreate">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { getFirmwares, getOTATasks, getOTALogs, createOTATask } from '@/api/firmware'
import type { Firmware, OTATask, OTALog } from '@/api/firmware'

const tasks = ref<OTATask[]>([])
const firmwares = ref<Firmware[]>([])
const logs = ref<OTALog[]>([])
const loading = ref(false)
const logsLoading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const selectedTaskId = ref(0)

let logsTimer: ReturnType<typeof setInterval> | null = null

const form = reactive({ firmware_id: undefined as number | undefined, target_type: 'device', target_id: '' })

const rules: FormRules = {
  firmware_id: [{ required: true, message: '请选择固件', trigger: 'change' }],
  target_type: [{ required: true }],
  target_id: [{ required: true, message: '请输入目标ID', trigger: 'blur' }]
}

function tagType(status: string) {
  if (status === 'running') return 'warning'
  if (status === 'done') return 'success'
  return 'info'
}

function logTagType(status: string) {
  if (status === 'success') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'downloading' || status === 'installing') return 'warning'
  return 'info'
}

async function fetchTasks() { loading.value = true; try { tasks.value = (await getOTATasks()).data || [] } catch { } finally { loading.value = false } }
async function fetchFirmwares() { try { firmwares.value = (await getFirmwares()).data || [] } catch { } }

async function selectTask(row: OTATask) {
  selectedTaskId.value = row.id
  await fetchLogs()
  if (logsTimer) clearInterval(logsTimer)
  logsTimer = setInterval(fetchLogs, 3000)
}

async function fetchLogs() {
  if (!selectedTaskId.value) return
  logsLoading.value = true
  try { logs.value = (await getOTALogs(selectedTaskId.value)).data || [] } catch { } finally { logsLoading.value = false }
}

function showTaskDialog() { dialogVisible.value = true }
function resetForm() { form.firmware_id = undefined; form.target_id = ''; form.target_type = 'device'; formRef.value?.resetFields() }

async function handleCreate() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    await createOTATask({ firmware_id: form.firmware_id!, target_type: form.target_type, target_id: form.target_id })
    ElMessage.success('OTA升级任务已下发')
    dialogVisible.value = false
    fetchTasks()
  } catch { } finally { submitting.value = false }
}

onMounted(() => { fetchFirmwares(); fetchTasks() })
</script>
