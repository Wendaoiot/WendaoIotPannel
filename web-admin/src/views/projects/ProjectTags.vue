<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="$router.back()">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h2>项目标签 - {{ projectId }}</h2>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="showAddDialog">
          <el-icon><Plus /></el-icon>
          绑定标签
        </el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="tags" v-loading="loading" stripe border>
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="tag_key" label="标签键" width="140" />
        <el-table-column prop="tag_name" label="标签名称" width="140" />
        <el-table-column prop="unit" label="单位" width="100" align="center" />
        <el-table-column prop="data_type" label="数据类型" width="120" align="center">
          <template #default="{ row }">
            <el-tag size="small">{{ row.data_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="可写" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.writable ? 'success' : 'info'" size="small">{{ row.writable ? '是' : '否' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="120" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="handleRemove(row)">解绑</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" title="绑定标签" width="500px" @closed="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
        <el-form-item label="标签键" prop="tag_key">
          <el-input v-model="form.tag_key" placeholder="请输入标签键名称" />
        </el-form-item>
        <el-form-item label="标签名称" prop="tag_name">
          <el-input v-model="form.tag_name" placeholder="请输入标签显示名称" />
        </el-form-item>
        <el-form-item label="单位" prop="unit">
          <el-input v-model="form.unit" placeholder="如: ℃, %, V" />
        </el-form-item>
        <el-form-item label="数据类型" prop="data_type">
          <el-select v-model="form.data_type" placeholder="请选择数据类型" style="width: 100%">
            <el-option label="数值 (number)" value="number" />
            <el-option label="布尔 (boolean)" value="boolean" />
            <el-option label="字符串 (string)" value="string" />
          </el-select>
        </el-form-item>
        <el-form-item label="可写" prop="writable">
          <el-switch v-model="form.writable" />
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
import { ref, reactive, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { getProjectTags, addProjectTag, removeProjectTag, type ProjectTag } from '@/api/project'

const route = useRoute()
const router = useRouter()
const projectId = computed(() => Number(route.params.projectId))
const isValidProjectId = computed(() => !isNaN(projectId.value) && projectId.value > 0)

const tags = ref<ProjectTag[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  tag_key: '',
  tag_name: '',
  unit: '',
  data_type: 'number',
  writable: false
})

const rules: FormRules = {
  tag_key: [{ required: true, message: '请输入标签键', trigger: 'blur' }],
  tag_name: [{ required: true, message: '请输入标签名称', trigger: 'blur' }],
  data_type: [{ required: true, message: '请选择数据类型', trigger: 'change' }]
}

async function fetchTags() {
  console.log('[ProjectTags] fetchTags - route params:', route.params, 'projectId:', projectId.value, 'isValid:', isValidProjectId.value)
  if (!isValidProjectId.value) {
    ElMessage.error('无效的项目ID')
    return
  }
  loading.value = true
  try {
    const res = await getProjectTags(projectId.value)
    tags.value = res.data || []
  } catch {
  } finally {
    loading.value = false
  }
}

function showAddDialog() {
  dialogVisible.value = true
}

function resetForm() {
  form.tag_key = ''
  form.tag_name = ''
  form.unit = ''
  form.data_type = 'number'
  form.writable = false
  formRef.value?.resetFields()
}

async function handleAdd() {
  if (!isValidProjectId.value) {
    ElMessage.error('无效的项目ID')
    return
  }

  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    await addProjectTag(projectId.value, {
      tag_key: form.tag_key,
      tag_name: form.tag_name,
      unit: form.unit,
      data_type: form.data_type,
      writable: form.writable
    })
    ElMessage.success('标签绑定成功')
    dialogVisible.value = false
    fetchTags()
  } catch {
  } finally {
    submitting.value = false
  }
}

async function handleRemove(row: ProjectTag) {
  if (!isValidProjectId.value) {
    ElMessage.error('无效的项目ID')
    return
  }

  try {
    await ElMessageBox.confirm(`确定要解绑标签 "${row.tag_key}" 吗？此操作不可恢复。`, '确认', {
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    await removeProjectTag(projectId.value, row.id)
    ElMessage.success('标签解绑成功')
    fetchTags()
  } catch {
  }
}

onMounted(() => {
  fetchTags()
})
</script>
