<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="$router.back()">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h2>设备标签配置 - {{ deviceId }}</h2>
      </div>
      <div class="toolbar-right">
        <el-button type="success" @click="goToChart">
          <el-icon><PieChart /></el-icon>
          数据图表
        </el-button>
        <el-button type="primary" @click="showAddDialog">
          <el-icon><Plus /></el-icon>
          添加标签配置
        </el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="tags" v-loading="loading" stripe border>
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="tag_key" label="标签键" width="160" />
        <el-table-column prop="interface" label="接口" width="120" />
        <el-table-column prop="formula" label="公式/配置" />
        <el-table-column label="操作" width="120" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="handleRemove(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="添加标签配置" width="500px" @closed="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item label="标签键" prop="tag_key">
          <el-input v-model="form.tag_key" placeholder="例如: temperature" />
        </el-form-item>
        <el-form-item label="接口" prop="interface">
          <el-input v-model="form.interface" placeholder="例如: modbus/rtu, sensor/analog" />
        </el-form-item>
        <el-form-item label="公式" prop="formula">
          <el-input v-model="form.formula" placeholder="例如: x*0.1, value/100" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleAdd">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { ArrowLeft, Plus, PieChart } from '@element-plus/icons-vue'
import { getDeviceTags, addDeviceTag, removeDeviceTag, type DeviceTag } from '@/api/device'

const route = useRoute()
const router = useRouter()
const deviceId = String(route.params.deviceId)

const tags = ref<DeviceTag[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  tag_key: '',
  interface: '',
  formula: ''
})

const rules: FormRules = {
  tag_key: [{ required: true, message: '请输入标签键', trigger: 'blur' }],
  interface: [{ required: true, message: '请输入接口', trigger: 'blur' }]
}

async function fetchTags() {
  loading.value = true
  try {
    const res = await getDeviceTags(deviceId)
    tags.value = res.data || []
  } catch {
  } finally {
    loading.value = false
  }
}

function goToChart() {
  router.push(`/devices/${deviceId}/chart`)
}

function showAddDialog() {
  dialogVisible.value = true
}

function resetForm() {
  form.tag_key = ''
  form.interface = ''
  form.formula = ''
  formRef.value?.resetFields()
}

async function handleAdd() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    await addDeviceTag(deviceId, {
      tag_key: form.tag_key,
      interface: form.interface,
      formula: form.formula
    })
    ElMessage.success('标签配置添加成功')
    dialogVisible.value = false
    fetchTags()
  } catch {
  } finally {
    submitting.value = false
  }
}

async function handleRemove(row: DeviceTag) {
  try {
    await ElMessageBox.confirm(`确定要删除标签配置 "${row.tag_key}" 吗？`, '确认', {
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    await removeDeviceTag(deviceId, row.id)
    ElMessage.success('删除成功')
    fetchTags()
  } catch {
  }
}

onMounted(() => {
  fetchTags()
})
</script>
