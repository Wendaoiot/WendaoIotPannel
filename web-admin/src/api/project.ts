import http from './index'

export interface Project {
  id: number
  tenant_id: number
  name: string
  created_at: string
}

export interface ProjectTag {
  id: number
  project_id: number
  tag_key: string
  tag_name: string
  unit: string
  data_type: string
  writable: boolean
  created_at: string
}

export function getProjects(tenantId?: number): Promise<{ code: number; msg: string; data: Project[] }> {
  const params = tenantId ? { tenant_id: tenantId } : {}
  return http.get('/projects', { params })
}

export function createProject(data: { tenant_id: number; name: string }): Promise<{ code: number; msg: string; data: Project }> {
  return http.post('/projects', data)
}

export function updateProject(id: number, data: { name?: string; tenant_id?: number }): Promise<{ code: number; msg: string; data: Project }> {
  return http.put(`/projects/${id}`, data)
}

export function deleteProject(id: number): Promise<{ code: number; msg: string }> {
  return http.delete(`/projects/${id}`)
}

export function addProjectTag(projectId: number, data: { tag_key: string; tag_name: string; unit: string; data_type: string; writable: boolean }): Promise<{ code: number; msg: string; data: ProjectTag }> {
  return http.post(`/projects/${projectId}/tags`, data)
}

export function getProjectTags(projectId: number): Promise<{ code: number; msg: string; data: ProjectTag[] }> {
  return http.get(`/projects/${projectId}/tags`)
}

export function removeProjectTag(projectId: number, id: number): Promise<{ code: number; msg: string }> {
  return http.delete(`/projects/${projectId}/tags`, { data: { id } })
}
