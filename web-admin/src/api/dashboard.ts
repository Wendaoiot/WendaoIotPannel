import http from './index'

export interface DashboardStats {
  total_tenants: number
  total_projects: number
  total_devices: number
  online_devices: number
}

export function getDashboardStats(): Promise<{ code: number; msg: string; data: DashboardStats }> {
  return http.get('/dashboard/stats')
}
