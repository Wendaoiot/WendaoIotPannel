<template>
  <div class="page-container">
    <h2 style="margin-bottom: 20px;">仪表盘</h2>
    <el-row :gutter="20" v-loading="loading">
      <el-col :span="8" v-if="isSuperAdmin">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-icon" style="background: #e6f7ff;"><el-icon :size="28" color="#1890ff"><OfficeBuilding /></el-icon></div>
            <div class="stat-body">
              <div class="stat-label">租户数</div>
              <div class="stat-value">{{ stats.total_tenants }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="isSuperAdmin ? 8 : 12">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-icon" style="background: #f6ffed;"><el-icon :size="28" color="#52c41a"><FolderOpened /></el-icon></div>
            <div class="stat-body">
              <div class="stat-label">项目数</div>
              <div class="stat-value">{{ stats.total_projects }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="isSuperAdmin ? 4 : 6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-icon" style="background: #fff7e6;"><el-icon :size="28" color="#fa8c16"><Cpu /></el-icon></div>
            <div class="stat-body">
              <div class="stat-label">设备总数</div>
              <div class="stat-value">{{ stats.total_devices }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="isSuperAdmin ? 4 : 6">
        <el-card shadow="hover">
          <div class="stat-card">
            <div class="stat-icon" style="background: #e6f7ff;"><el-icon :size="28" color="#1890ff"><Connection /></el-icon></div>
            <div class="stat-body">
              <div class="stat-label">在线设备</div>
              <div class="stat-value" style="color: #52c41a;">{{ stats.online_devices }}</div>
            </div>
          </div>
        </el-card>
      </el-col>
    </el-row>

    <el-row :gutter="20" style="margin-top: 20px;">
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>设备状态分布</template>
          <div v-if="stats.total_devices > 0">
            <div style="margin-bottom: 12px;">
              <span>在线</span>
              <el-progress :percentage="onlinePercent" :color="'#67c23a'" style="margin-top: 4px;" />
            </div>
            <div style="margin-bottom: 12px;">
              <span>离线/未激活</span>
              <el-progress :percentage="offlinePercent" :color="'#e6a23c'" style="margin-top: 4px;" />
            </div>
          </div>
          <div v-else style="color: #909399; text-align: center; padding: 20px 0;">暂无设备数据</div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="hover">
          <template #header>系统信息</template>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="平台版本">v1.0.0</el-descriptions-item>
            <el-descriptions-item label="运行状态"><el-tag type="success">运行中</el-tag></el-descriptions-item>
            <el-descriptions-item label="当前用户">{{ authStore.user?.username || '-' }}</el-descriptions-item>
            <el-descriptions-item label="用户角色">
              <el-tag>{{ authStore.user?.role === 'super_admin' ? '超级管理员' : '租户管理员' }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed, onMounted, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { getDashboardStats, type DashboardStats } from '@/api/dashboard'

const authStore = useAuthStore()
const loading = ref(false)

const stats = reactive<DashboardStats>({
  total_tenants: 0,
  total_projects: 0,
  total_devices: 0,
  online_devices: 0
})

const isSuperAdmin = computed(() => authStore.role === 'super_admin')

const onlinePercent = computed(() => {
  if (stats.total_devices === 0) return 0
  return Math.round(stats.online_devices / stats.total_devices * 100)
})

const offlinePercent = computed(() => {
  if (stats.total_devices === 0) return 0
  return 100 - onlinePercent.value
})

onMounted(async () => {
  loading.value = true
  try {
    const res = await getDashboardStats()
    if (res.data) {
      Object.assign(stats, res.data)
    }
  } catch {
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stat-body {
  flex: 1;
}

.stat-label {
  font-size: 13px;
  color: #909399;
  margin-bottom: 4px;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  color: #303133;
}
</style>
