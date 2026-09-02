import http from './index'

export interface Device {
  id: string
  project_id: number
  name: string
  status: number
  first_ts: number
  last_active: string | null
  created_at: string
}

export interface DeviceTag {
  id: number
  device_id: string
  tag_key: string
  interface: string
  formula: string
}

export interface DeviceDataPoint {
  id: number
  device_id: string
  msg_id: string
  ts: number
  data: Record<string, unknown>
  version?: string
  created_at: string
}

export interface ControlResult {
  msg_id: string
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

export function createDevice(data: { id: string; project_id: number; name: string }): Promise<{ code: number; msg: string; data: Device }> {
  return http.post('/devices', data)
}

export function updateDevice(deviceId: string, data: { name?: string; project_id?: number; status?: number }): Promise<{ code: number; msg: string; data: Device }> {
  return http.put(`/devices/${deviceId}`, data)
}

export function deleteDevice(deviceId: string): Promise<{ code: number; msg: string }> {
  return http.delete(`/devices/${deviceId}`)
}

export function addDeviceTag(deviceId: string, data: { tag_key: string; interface: string; formula: string }): Promise<{ code: number; msg: string; data: DeviceTag }> {
  return http.post(`/devices/${deviceId}/tags`, data)
}

export function getDeviceTags(deviceId: string): Promise<{ code: number; msg: string; data: DeviceTag[] }> {
  return http.get(`/devices/${deviceId}/tags`)
}

export function removeDeviceTag(deviceId: string, id: number): Promise<{ code: number; msg: string }> {
  return http.delete(`/devices/${deviceId}/tags`, { data: { id } })
}

export function getDeviceData(deviceId: string, limit: number = 100, offset: number = 0): Promise<{ code: number; msg: string; data: { list: DeviceDataPoint[]; total: number; limit: number; offset: number } }> {
  return http.get(`/devices/${deviceId}/data`, { params: { limit, offset } })
}

export function sendDeviceControl(deviceId: string, tags: Record<string, number>): Promise<{ code: number; msg: string; data: ControlResult }> {
  return http.post(`/devices/${deviceId}/control`, { tags })
}
