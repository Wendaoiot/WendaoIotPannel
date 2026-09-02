<template>
  <el-container class="layout-container">
    <el-aside width="220px" class="layout-aside">
      <div class="logo">
        <span style="font-size: 18px; font-weight: bold;">Wendao IoT</span>
      </div>
      <el-menu
        :default-active="activeMenu"
        router
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409eff"
      >
        <el-menu-item index="/dashboard">
          <el-icon><Odometer /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item v-if="isSuperAdmin" index="/tenants">
          <el-icon><OfficeBuilding /></el-icon>
          <span>租户管理</span>
        </el-menu-item>
        <el-menu-item v-if="isSuperAdmin" index="/users">
          <el-icon><User /></el-icon>
          <span>用户管理</span>
        </el-menu-item>
        <el-menu-item index="/projects">
          <el-icon><FolderOpened /></el-icon>
          <span>项目管理</span>
        </el-menu-item>
        <el-menu-item index="/devices">
          <el-icon><Cpu /></el-icon>
          <span>设备管理</span>
        </el-menu-item>
        <el-menu-item index="/control-logs">
          <el-icon><Document /></el-icon>
          <span>控制日志</span>
        </el-menu-item>
        <el-menu-item index="/firmwares">
          <el-icon><Box /></el-icon>
          <span>固件管理</span>
        </el-menu-item>
        <el-menu-item index="/ota">
          <el-icon><Upload /></el-icon>
          <span>OTA升级</span>
        </el-menu-item>
      </el-menu>
    </el-aside>

    <el-container>
      <el-header class="layout-header">
        <div class="header-left">
          <span style="color: #303133;">{{ pageTitle }}</span>
        </div>
        <div class="header-right">
          <el-dropdown trigger="click">
            <span class="user-dropdown">
              <el-icon style="margin-right: 4px;"><UserFilled /></el-icon>
              {{ authStore.user?.username || '用户' }}
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item>
                  <span>角色: {{ authStore.user?.role === 'super_admin' ? '超级管理员' : '租户管理员' }}</span>
                </el-dropdown-item>
                <el-dropdown-item divided @click="handleLogout">
                  <span style="color: #f56c6c;">退出登录</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>

      <el-main class="layout-main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const isSuperAdmin = computed(() => authStore.role === 'super_admin')

const activeMenu = computed(() => {
  const path = route.path
  if (path.startsWith('/projects')) return '/projects'
  if (path.startsWith('/devices')) return '/devices'
  if (path.startsWith('/firmwares')) return '/firmwares'
  if (path.startsWith('/ota')) return '/ota'
  if (path.startsWith('/users')) return '/users'
  if (path.startsWith('/control-logs')) return '/control-logs'
  return path
})

const pageTitle = computed(() => {
  return (route.meta?.title as string) || 'Wendao IoT Platform'
})

async function handleLogout() {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '确认退出', {
      type: 'warning'
    })
  } catch {
    return
  }
  authStore.clearAuth()
  router.push('/login')
}
</script>

<style scoped>
.layout-container {
  height: 100vh;
}

.layout-aside {
  background-color: #304156;
  overflow-y: auto;
  overflow-x: hidden;
}

.logo {
  height: 60px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.layout-aside .el-menu {
  border-right: none;
}

.layout-header {
  background: #fff;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  height: 60px;
}

.header-left {
  font-size: 16px;
  font-weight: 500;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-dropdown {
  display: flex;
  align-items: center;
  cursor: pointer;
  color: #606266;
  font-size: 14px;
}

.layout-main {
  background: #f0f2f5;
  min-height: calc(100vh - 60px);
}
</style>
