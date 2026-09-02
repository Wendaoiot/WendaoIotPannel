<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>设备管理</h2>
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

    <el-card class="table-card">
      <el-table :data="devices" v-loading="loading" stripe border>
        <el-table-column prop="id" label="设备ID" width="160" />
        <el-table-column prop="name" label="设备名称" />
        <el-table-column label="项目" width="160" align="center">
          <template #default="{ row }">
            {{ getProjectName(row.project_id) }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="statusType(row.status)" size="small">
              {{ statusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" align="center" />
        <el-table-column label="操作" width="420" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="showEditDialog(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
            <el-button size="small" type="warning" @click="goToTags(row)">标签</el-button>
            <el-button size="small" type="info" @click="goToData(row)">数据</el-button>
            <el-button size="small" type="success" @click="goToControl(row)">控制</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

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

    <el-dialog v-model="editDialogVisible" title="编辑设备" width="450px" @closed="resetEditForm">
      <el-form ref="editFormRef" :model="editForm" :rules="editRules" label-width="80px">
        <el-form-item label="设备名称" prop="name">
          <el-input v-model="editForm.name" placeholder="请输入设备名称" />
        </el-form-item>
        <el-form-item label="所属项目" prop="project_id">
          <el-select v-model="editForm.project_id" placeholder="请选择项目" style="width: 100%">
            <el-option
              v-for="p in projects"
              :key="p.id"
              :label="p.name"
              :value="p.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="设备状态" prop="status">
          <el-select v-model="editForm.status" placeholder="请选择状态" style="width: 100%">
            <el-option label="在线" :value="1" />
            <el-option label="离线" :value="0" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleEdit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { getDevices, createDevice, updateDevice, deleteDevice, type Device } from '@/api/device'
import { getProjects, type Project } from '@/api/project'
import { useAuthStore } from '@/stores/auth'

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

const editDialogVisible = ref(false)
const editFormRef = ref<FormInstance>()
const editForm = reactive({
  id: '',
  name: '',
  project_id: undefined as number | undefined,
  status: 0
})
const editRules: FormRules = {
  name: [{ required: true, message: '请输入设备名称', trigger: 'blur' }],
  project_id: [{ required: true, message: '请选择项目', trigger: 'change' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }]
}

function getProjectName(pid: number): string {
  const p = projects.value.find(item => item.id === pid)
  return p?.name || `项目${pid}`
}

function statusType(status: number): 'success' | 'warning' | 'info' {
  if (status === 1) return 'success'
  if (status === 0) return 'warning'
  return 'info'
}

function statusText(status: number): string {
  if (status === 1) return '在线'
  if (status === 0) return '离线'
  return '未激活'
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
    await createDevice({
      id: createForm.id,
      project_id: createForm.project_id!,
      name: createForm.name
    })
    ElMessage.success('设备创建成功')
    createDialogVisible.value = false
    fetchDevices()
  } catch {
  } finally {
    submitting.value = false
  }
}

function showEditDialog(row: Device) {
  editForm.id = row.id
  editForm.name = row.name
  editForm.project_id = row.project_id
  editForm.status = row.status
  editDialogVisible.value = true
}

function resetEditForm() {
  editForm.id = ''
  editForm.name = ''
  editForm.project_id = undefined
  editForm.status = 0
  editFormRef.value?.resetFields()
}

async function handleEdit() {
  const valid = await editFormRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    await updateDevice(editForm.id, {
      name: editForm.name,
      project_id: editForm.project_id,
      status: editForm.status
    })
    ElMessage.success('设备更新成功')
    editDialogVisible.value = false
    fetchDevices()
  } catch {
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: Device) {
  try {
    await ElMessageBox.confirm(`确定要删除设备 "${row.name}" (${row.id}) 吗？此操作将同时删除该设备的所有标签配置和数据，不可恢复。`, '确认删除', {
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    await deleteDevice(row.id)
    ElMessage.success('设备删除成功')
    fetchDevices()
  } catch {
  }
}

function goToTags(row: Device) {
  router.push(`/devices/${row.id}/tags`)
}

function goToData(row: Device) {
  router.push(`/devices/${row.id}/data`)
}

function goToControl(row: Device) {
  router.push(`/devices/${row.id}/control`)
}

onMounted(() => {
  fetchProjects()
  fetchDevices()
})
</script>
