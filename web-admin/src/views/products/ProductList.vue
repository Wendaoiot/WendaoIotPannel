<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-select
          v-if="!isTenantAdmin"
          v-model="filterTenantId"
          placeholder="按租户筛选"
          clearable
          style="width: 200px"
          @change="fetchProducts"
        >
          <el-option
            v-for="t in tenants"
            :key="t.id"
            :label="t.name"
            :value="t.id"
          />
        </el-select>
      </div>
      <div class="toolbar-right">
        <el-button type="primary" @click="showCreateDialog">
          <el-icon><Plus /></el-icon>
          新建产品
        </el-button>
      </div>
    </div>

    <el-card class="table-card">
      <el-table :data="products" v-loading="loading" stripe border>
        <el-table-column prop="id" label="ID" width="70" align="center" />
        <el-table-column prop="name" label="产品名称" min-width="140" />
        <el-table-column label="ProductKey" min-width="200">
          <template #default="{ row }">
            <div class="key-cell">
              <span class="mono">{{ row.product_key }}</span>
              <el-button text size="small" title="复制" @click="copyText(row.product_key, 'ProductKey')">
                <el-icon><CopyDocument /></el-icon>
              </el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="动态注册" width="110" align="center">
          <template #default="{ row }">
            <el-switch
              :model-value="row.dyn_reg_enabled"
              :loading="switchingKey === row.product_key"
              @change="(v: string | number | boolean) => toggleDynReg(row, Boolean(v))"
            />
          </template>
        </el-table-column>
        <el-table-column v-if="!isTenantAdmin" label="所属租户" width="150" align="center">
          <template #default="{ row }">
            {{ getTenantName(row.tenant_id) }}
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="180" align="center">
          <template #default="{ row }">{{ formatTs(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="210" align="center" fixed="right">
          <template #default="{ row }">
            <el-button size="small" type="warning" @click="handleResetSecret(row)">重置密钥</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createDialogVisible" title="新建产品" width="460px" @closed="resetCreateForm">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules" label-width="90px">
        <el-form-item v-if="!isTenantAdmin" label="所属租户" prop="tenant_id">
          <el-select v-model="createForm.tenant_id" placeholder="请选择租户" style="width: 100%">
            <el-option
              v-for="t in tenants"
              :key="t.id"
              :label="t.name"
              :value="t.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="产品名称" prop="name">
          <el-input v-model="createForm.name" placeholder="如：温感型号A（同型号设备共用凭证）" />
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
import { ref, reactive, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import { Plus, CopyDocument } from '@element-plus/icons-vue'
import {
  getProducts,
  createProduct,
  updateProduct,
  resetProductSecret,
  deleteProduct,
  type Product
} from '@/api/product'
import { getTenants, type Tenant } from '@/api/tenant'
import { useAuthStore } from '@/stores/auth'
import { formatTs } from '@/utils/datetime'

const authStore = useAuthStore()
const isTenantAdmin = computed(() => authStore.role === 'tenant_admin')
const tenantId = computed(() => authStore.tenantId)
const products = ref<Product[]>([])
const tenants = ref<Tenant[]>([])
const loading = ref(false)
const submitting = ref(false)
const switchingKey = ref('')
const filterTenantId = ref<number | undefined>(undefined)

const createDialogVisible = ref(false)
const createFormRef = ref<FormInstance>()
const createForm = reactive({
  tenant_id: undefined as number | undefined,
  name: ''
})
const createRules: FormRules = {
  tenant_id: isTenantAdmin.value ? [] : [{ required: true, message: '请选择租户', trigger: 'change' }],
  name: [{ required: true, message: '请输入产品名称', trigger: 'blur' }]
}

function getTenantName(tid: number): string {
  return tenants.value.find(t => t.id === tid)?.name || `租户${tid}`
}

async function fetchProducts() {
  loading.value = true
  try {
    const res = await getProducts(isTenantAdmin.value ? tenantId.value : filterTenantId.value)
    products.value = res.data || []
  } catch {
  } finally {
    loading.value = false
  }
}

async function fetchTenants() {
  if (isTenantAdmin.value) return
  try {
    const res = await getTenants()
    tenants.value = res.data || []
  } catch {
  }
}

function showCreateDialog() {
  if (isTenantAdmin.value) createForm.tenant_id = tenantId.value
  createDialogVisible.value = true
}

function resetCreateForm() {
  createForm.name = ''
  if (!isTenantAdmin.value) createForm.tenant_id = undefined
  createFormRef.value?.resetFields()
}

async function handleCreate() {
  const valid = await createFormRef.value?.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    const res = await createProduct({
      name: createForm.name,
      tenant_id: isTenantAdmin.value ? undefined : createForm.tenant_id
    })
    createDialogVisible.value = false
    ElMessage.success('产品创建成功')
    await showCredential(
      res.data.product.product_key,
      res.data.product_secret,
      '产品凭证（仅显示一次）'
    )
    fetchProducts()
  } catch {
  } finally {
    submitting.value = false
  }
}

async function toggleDynReg(row: Product, enabled: boolean) {
  switchingKey.value = row.product_key
  try {
    await updateProduct(row.product_key, { dyn_reg_enabled: enabled })
    row.dyn_reg_enabled = enabled
    ElMessage.success(enabled ? '动态注册已开启' : '动态注册已关闭，待激活设备的引导连接将被拒绝')
  } catch {
    // 全局拦截器已提示；switch 值随行数据不变，自动回弹
  } finally {
    switchingKey.value = ''
  }
}

async function handleResetSecret(row: Product) {
  try {
    await ElMessageBox.confirm(
      `重置产品 "${row.name}" 的密钥后，旧密钥立即失效；尚未激活的设备必须使用新密钥引导注册。确定继续？`,
      '重置产品密钥',
      { type: 'warning', confirmButtonText: '重置', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  try {
    const res = await resetProductSecret(row.product_key)
    const key = res.data.product_key || row.product_key
    await showCredential(key, res.data.product_secret, '新产品密钥（仅显示一次）')
  } catch {
  }
}

async function handleDelete(row: Product) {
  try {
    await ElMessageBox.confirm(
      `确定删除产品 "${row.name}" 吗？产品下仍有设备（含待激活）时将被平台拒绝。`,
      '确认删除',
      { type: 'warning', confirmButtonText: '删除', confirmButtonClass: 'el-button--danger' }
    )
  } catch {
    return
  }
  try {
    await deleteProduct(row.product_key)
    ElMessage.success('产品已删除')
    fetchProducts()
  } catch {
    // 引用保护等业务错误由响应拦截器统一提示
  }
}

/** 凭证一次性展示框：ProductKey + ProductSecret + 固件烧录/引导连接说明 */
async function showCredential(productKey: string, productSecret: string, title: string) {
  // ElMessageBox 内容在调用后才挂到 DOM，用 document 事件委托处理两条“复制”链接
  const handler = (e: MouseEvent) => {
    const el = (e.target as HTMLElement)?.closest('a[data-copy]') as HTMLElement | null
    if (!el) return
    const which = el.dataset.copy
    copyText(which === 'key' ? productKey : productSecret, which === 'key' ? 'ProductKey' : 'ProductSecret')
  }
  document.addEventListener('click', handler)
  try {
    await ElMessageBox.alert(
    `<div style="line-height:1.9;font-size:13px">`
    + `ProductKey：<b class="mono">${productKey}</b> `
    + `<a href="javascript:void(0)" data-copy="key" style="margin-left:6px">复制</a><br/>`
    + `ProductSecret：<b class="mono">${productSecret}</b> `
    + `<a href="javascript:void(0)" data-copy="sec" style="margin-left:6px">复制</a>`
    + `</div>`
    + `<div style="margin-top:10px;padding:10px;background:var(--el-fill-color-light);border-radius:6px;line-height:1.8;font-size:12.5px">`
    + `烧录到同型号设备的凭证：<br/>`
    + `· 首次上电引导连接：<b>mqtts://主机:8883</b>（需导入平台 CA）<br/>`
    + `· 用户名：<b class="mono">{SN}&amp;${productKey}</b>　密码：<b class="mono">${productSecret}</b><br/>`
    + `· 注册成功后设备获得一机一密，改用用户名 <b>{SN}</b> 正常连接`
    + `</div>`
    + `<div style="margin-top:8px;color:var(--el-color-danger);font-size:12.5px">产品密钥仅本次显示，请立即妥善保存。</div>`,
    title,
    {
      dangerouslyUseHTMLString: true,
      confirmButtonText: '我已保存',
      type: 'success',
      customClass: 'product-credential-box',
      callback: () => undefined
    }
  )
  } catch {
  } finally {
    document.removeEventListener('click', handler)
  }
}

async function copyText(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`${label} 已复制`)
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    try {
      document.execCommand('copy')
      ElMessage.success(`${label} 已复制`)
    } catch {
      ElMessage.warning('复制失败，请手动选择文本复制')
    }
    document.body.removeChild(ta)
  }
}

onMounted(() => {
  fetchTenants()
  fetchProducts()
})
</script>

<style scoped>
.mono {
  font-family: var(--wd-font-mono, ui-monospace, Consolas, monospace);
}

.key-cell {
  display: flex;
  align-items: center;
  gap: 4px;
}

.key-cell .mono {
  font-size: 12.5px;
}
</style>
