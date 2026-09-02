import http from './index'

export interface Firmware {
  id: number
  name: string
  version: string
  url: string
  size: number
  md5: string
  description: string
  created_at: string
}

export interface OTATask {
  id: number
  firmware_id: number
  target_type: string
  target_id: string
  status: string
  created_at: string
}

export interface OTALog {
  id: number
  task_id: number
  device_id: string
  status: string
  progress: number
  error_msg: string
  created_at: string
  updated_at: string
}

export interface ControlLog {
  id: number
  device_id: string
  msg_id: string
  tags: Record<string, unknown>
  ack_code: number
  ack_msg: string
  created_at: string
}

export function getFirmwares(): Promise<{ code: number; msg: string; data: Firmware[] }> {
  return http.get('/firmwares')
}

export function createFirmware(data: {
  name: string
  version: string
  url: string
  size?: number
  md5?: string
  description?: string
}): Promise<{ code: number; msg: string; data: Firmware }> {
  return http.post('/firmwares', data)
}

export function deleteFirmware(id: number): Promise<{ code: number; msg: string }> {
  return http.delete('/firmwares', { data: { id } })
}

export function createOTATask(data: {
  firmware_id: number
  target_type: string
  target_id: string
}): Promise<{ code: number; msg: string; data: { task_id: number } }> {
  return http.post('/ota/tasks', data)
}

export function getOTATasks(): Promise<{ code: number; msg: string; data: OTATask[] }> {
  return http.get('/ota/tasks')
}

export function getOTALogs(taskId: number): Promise<{ code: number; msg: string; data: OTALog[] }> {
  return http.get('/ota/logs', { params: { task_id: taskId } })
}

export function getControlLogs(deviceId?: string, limit: number = 50, offset: number = 0): Promise<{ code: number; msg: string; data: { list: ControlLog[]; total: number } }> {
  const params: Record<string, unknown> = { limit, offset }
  if (deviceId) params.device_id = deviceId
  return http.get('/control-logs', { params })
}
