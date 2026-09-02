<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <h2>用户管理</h2>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="users" v-loading="loading" stripe border>
        <el-table-column prop="id" label="ID" width="80" align="center" />
        <el-table-column prop="username" label="用户名" width="160" />
        <el-table-column label="角色" width="140" align="center">
          <template #default="{ row }">
            <el-tag :type="row.role === 'super_admin' ? 'danger' : 'warning'" size="small">
              {{ row.role === 'super_admin' ? '超级管理员' : '租户管理员' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="租户ID" width="120" align="center">
          <template #default="{ row }">
            {{ row.tenant_id || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" align="center" />
        <el-table-column label="操作" width="200" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="warning" @click="showResetDialog(row)">重置密码</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="resetDialogVisible" title="重置密码" width="450px" @closed="resetResetForm">
      <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules" label-width="80px">
        <el-form-item label="用户名">
          <el-input :model-value="resetForm.username" disabled />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="resetForm.new_password" type="password" placeholder="请输入新密码" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleReset">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { listUsers, adminResetPassword, deleteUser, type UserListItem } from '@/api/auth'

const users = ref<UserListItem[]>([])
const loading = ref(false)
const submitting = ref(false)

const resetDialogVisible = ref(false)
const resetFormRef = ref<FormInstance>()
const resetForm = reactive({
  id: 0,
  username: '',
  new_password: ''
})
const resetRules: FormRules = {
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 4, message: '密码长度不能少于4位', trigger: 'blur' }
  ]
}

async function fetchUsers() {
  loading.value = true
  try {
    const res = await listUsers()
    users.value = res.data || []
  } catch {
  } finally {
    loading.value = false
  }
}

function showResetDialog(row: UserListItem) {
  resetForm.id = row.id
  resetForm.username = row.username
  resetForm.new_password = ''
  resetDialogVisible.value = true
}

function resetResetForm() {
  resetForm.id = 0
  resetForm.username = ''
  resetForm.new_password = ''
  resetFormRef.value?.resetFields()
}

async function handleReset() {
  const valid = await resetFormRef.value?.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    await adminResetPassword(resetForm.id, resetForm.new_password)
    ElMessage.success('密码重置成功')
    resetDialogVisible.value = false
  } catch {
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: UserListItem) {
  try {
    await ElMessageBox.confirm(`确定要删除用户 "${row.username}" 吗？此操作不可恢复。`, '确认删除', {
      type: 'warning'
    })
  } catch {
    return
  }

  try {
    await deleteUser(row.id)
    ElMessage.success('用户删除成功')
    fetchUsers()
  } catch {
  }
}

onMounted(() => {
  fetchUsers()
})
</script>
