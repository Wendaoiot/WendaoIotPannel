import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory('/iot/'), // 新增
  routes: [
    {
      path: '/login',
      name: 'Login',
      component: () => import('@/views/Login.vue'),
      meta: { requiresAuth: false }
    },
    {
      path: '/',
      component: () => import('@/components/Layout.vue'),
      meta: { requiresAuth: true },
      redirect: '/dashboard',
      children: [
        {
          path: 'dashboard',
          name: 'Dashboard',
          component: () => import('@/views/Dashboard.vue'),
          meta: { title: '仪表盘' }
        },
        {
          path: 'tenants',
          name: 'Tenants',
          component: () => import('@/views/tenants/TenantList.vue'),
          meta: { title: '租户管理', roles: ['super_admin'] }
        },
        {
          path: 'projects',
          name: 'Projects',
          component: () => import('@/views/projects/ProjectList.vue'),
          meta: { title: '项目管理' }
        },
        {
          path: 'projects/:projectId/tags',
          name: 'ProjectTags',
          component: () => import('@/views/projects/ProjectTags.vue'),
          meta: { title: '项目标签' }
        },
        {
          path: 'devices',
          name: 'Devices',
          component: () => import('@/views/devices/DeviceList.vue'),
          meta: { title: '设备管理' }
        },
        {
          path: 'devices/:deviceId/tags',
          name: 'DeviceTags',
          component: () => import('@/views/devices/DeviceTags.vue'),
          meta: { title: '设备标签配置' }
        },
        {
          path: 'devices/:deviceId/data',
          name: 'DeviceData',
          component: () => import('@/views/devices/DeviceData.vue'),
          meta: { title: '设备数据' }
        },
        {
          path: 'devices/:deviceId/chart',
          name: 'DeviceChart',
          component: () => import('@/views/devices/DeviceChart.vue'),
          meta: { title: '数据图表' }
        },
        {
          path: 'devices/:deviceId/control',
          name: 'DeviceControl',
          component: () => import('@/views/devices/DeviceControl.vue'),
          meta: { title: '设备控制' }
        },
        {
          path: 'firmwares',
          name: 'Firmwares',
          component: () => import('@/views/firmwares/FirmwareList.vue'),
          meta: { title: '固件管理' }
        },
        {
          path: 'ota',
          name: 'OTA',
          component: () => import('@/views/ota/OTATasks.vue'),
          meta: { title: 'OTA升级' }
        },
        {
          path: 'users',
          name: 'Users',
          component: () => import('@/views/users/UserList.vue'),
          meta: { title: '用户管理', roles: ['super_admin'] }
        },
        {
          path: 'control-logs',
          name: 'ControlLogs',
          component: () => import('@/views/logs/ControlLogs.vue'),
          meta: { title: '控制日志' }
        }
      ]
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'NotFound',
      component: () => import('@/views/NotFound.vue'),
      meta: { requiresAuth: false }
    }
  ]
})

router.beforeEach((to, _from, next) => {
  const authStore = useAuthStore()
  const requiresAuth = to.matched.some(record => record.meta.requiresAuth !== false)

  if (requiresAuth && !authStore.token) {
    next('/login')
    return
  }

  if (to.path === '/login' && authStore.token) {
    next('/dashboard')
    return
  }

  const role = authStore.user?.role
  if (to.meta.roles && role && !(to.meta.roles as string[]).includes(role)) {
    next('/dashboard')
    return
  }

  next()
})

export default router
