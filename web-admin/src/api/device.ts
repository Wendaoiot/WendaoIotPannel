import http from './index'
import { downloadGet } from '@/utils/download'

export interface Device {
  id: string
  project_id: number
  name: string
  status: number
  enabled?: boolean
  /** 设备级在线判定：connection=仅按连接(系统默认) / report=按上报时间 / ping=按应答信号；历史空串等同 connection */
  online_mode?: string
  /** 判定离线时限（秒），0=沿用全局时限；connection 下不使用，上限 604800 */
  offline_timeout_sec?: number
  first_ts: number
  last_active: string | null
  created_at: string
}

export interface DeviceTag {
  id: number
  device_id: string
  tag_key: string
  name?: string
  unit?: string
  interface: string
  formula: string
}

export interface DeviceDataPoint {
  id: number
  device_id: string
  msg_id: string
  ts: number
  device_ts?: number
  data: Record<string, unknown>
  version?: string
  created_at: string
}

export interface ControlSendResult {
  msg_id: string
  status?: string
  hint?: string
}

export interface ControlResult {
  msg_id: string
}

export interface ControlStatus {
  msg_id: string
  status: 'pending' | 'delivered' | 'success' | 'failed' | 'timeout' | string
  ack_code: number | null
  ack_msg: string
}

// 项目自定义控制命令
export interface ControlCommand {
  id: number
  project_id: number
  device_id?: string // 空字符串 = 项目通用
  name: string
  tag_key: string
  value: number
  icon?: string
  sort?: number
  danger?: boolean
}

export interface DeviceDataQuery {
  start?: number
  end?: number
  limit?: number
  offset?: number
}

// 数据删除条件（三选一）：按行 ID / 时间范围（毫秒，可只给一端）/ 全部
export interface DeviceDataDeleteQuery {
  ids?: number[]
  start?: number
  end?: number
  all?: boolean
}

export function getDevices(projectId?: number): Promise<{ code: number; msg: string; data: Device[] }> {
  const params: Record<string, number> = {}
  if (projectId !== undefined && projectId !== null) {
    params.project_id = projectId
  }
  return http.get('/devices', { params })
}

export function getDevice(deviceId: string): Promise<{ code: number; msg: string; data: Device }> {
  return http.get(`/devices/${deviceId}`)
}

export function createDevice(data: { id: string; project_id: number; name: string }): Promise<{ code: number; msg: string; data: { device: Device; device_secret: string; secret_note: string } }> {
  return http.post('/devices', data)
}

export function updateDevice(
  deviceId: string,
  data: { name?: string; project_id?: number; status?: number; online_mode?: string; offline_timeout_sec?: number }
): Promise<{ code: number; msg: string; data: Device }> {
  return http.put(`/devices/${deviceId}`, data)
}

export function deleteDevice(deviceId: string): Promise<{ code: number; msg: string }> {
  return http.delete(`/devices/${deviceId}`)
}

// 启用/禁用设备
export function setDeviceEnabled(deviceId: string, enabled: boolean): Promise<{ code: number; msg: string }> {
  return http.put(`/devices/${deviceId}/enabled`, { enabled })
}

// 重置设备密钥（明文仅本次返回）
export function resetDeviceSecret(deviceId: string): Promise<{ code: number; msg: string; data: { device_id: string; device_secret: string; secret_note: string } }> {
  return http.post(`/devices/${deviceId}/secret/reset`)
}

export function addDeviceTag(deviceId: string, data: { tag_key: string; name?: string; unit?: string; interface: string; formula: string }): Promise<{ code: number; msg: string; data: DeviceTag }> {
  return http.post(`/devices/${deviceId}/tags`, data)
}

export function getDeviceTags(deviceId: string): Promise<{ code: number; msg: string; data: DeviceTag[] }> {
  return http.get(`/devices/${deviceId}/tags`)
}

export function removeDeviceTag(deviceId: string, id: number): Promise<{ code: number; msg: string }> {
  return http.delete(`/devices/${deviceId}/tags`, { data: { id } })
}

export function getDeviceData(
  deviceId: string,
  query: DeviceDataQuery = {}
): Promise<{ code: number; msg: string; data: { list: DeviceDataPoint[]; total: number; limit: number; offset: number } }> {
  const params: Record<string, unknown> = {
    limit: query.limit ?? 100,
    offset: query.offset ?? 0
  }
  if (query.start !== undefined) params.start = query.start
  if (query.end !== undefined) params.end = query.end
  return http.get(`/devices/${deviceId}/data`, { params })
}

export function sendDeviceControl(deviceId: string, tags: Record<string, number>): Promise<{ code: number; msg: string; data: ControlSendResult }> {
  return http.post(`/devices/${deviceId}/control`, { tags })
}

// 删除设备数据（仅超管；后端按 ids / start+end / all 三选一处理）
export function deleteDeviceData(deviceId: string, data: DeviceDataDeleteQuery): Promise<{ code: number; msg: string; data: { deleted: number } }> {
  return http.delete(`/devices/${deviceId}/data`, { data })
}

// 查询单条控制指令状态（pending/delivered/success/failed/timeout）
export function getControlStatus(deviceId: string, msgId: string): Promise<{ code: number; msg: string; data: ControlStatus }> {
  return http.get(`/devices/${deviceId}/control/${msgId}`)
}

// ===== 项目自定义控制命令 =====

export function listCommands(projectId: number, deviceId?: string): Promise<{ code: number; msg: string; data: ControlCommand[] }> {
  const params: Record<string, unknown> = {}
  if (deviceId) params.device_id = deviceId
  return http.get(`/projects/${projectId}/commands`, { params })
}

export function createCommand(data: { project_id: number; device_id?: string; name: string; tag_key: string; value: number; icon?: string; sort?: number; danger?: boolean }): Promise<{ code: number; msg: string; data: ControlCommand }> {
  return http.post(`/projects/${data.project_id}/commands`, data)
}

export function updateCommand(id: number, data: Partial<Omit<ControlCommand, 'id'>>): Promise<{ code: number; msg: string; data: ControlCommand }> {
  return http.put(`/commands/${id}`, data)
}

export function deleteCommand(id: number): Promise<{ code: number; msg: string }> {
  return http.delete(`/commands/${id}`)
}

// ===== CSV 导出（携带 JWT，Blob 下载）=====

export function exportDeviceDataCsv(deviceId: string, query: DeviceDataQuery = {}): Promise<void> {
  const params: Record<string, unknown> = { export: 'csv' }
  if (query.start !== undefined) params.start = query.start
  if (query.end !== undefined) params.end = query.end
  if (query.limit !== undefined) params.limit = query.limit
  if (query.offset !== undefined) params.offset = query.offset
  return downloadGet(`/devices/${deviceId}/data`, params, `device_data_${deviceId}.csv`)
}
