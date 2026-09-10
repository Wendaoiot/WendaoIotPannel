/**
 * 设备级在线判定方式（系统默认「仅按连接」）。
 * 历史数据可能为空串，空串/未知值一律按 connection 展示与提交。
 */
export const ONLINE_MODES = [
  { value: 'connection', label: '仅按连接' },
  { value: 'report', label: '按上报时间' },
  { value: 'ping', label: '按应答信号' }
] as const

export type OnlineModeValue = (typeof ONLINE_MODES)[number]['value']

/** 空串（跟随全局的历史值）与未知值统一归一为 connection。 */
export function normalizeOnlineMode(mode?: string | null): OnlineModeValue {
  return mode === 'report' || mode === 'ping' ? mode : 'connection'
}

export function onlineModeLabel(mode?: string | null): string {
  return ONLINE_MODES.find(item => item.value === normalizeOnlineMode(mode))?.label ?? '仅按连接'
}

/** 将秒数格式化为中文时长：如 1天2小时30分 / 45秒。 */
export function formatTimeoutDuration(sec: number): string {
  const total = Math.max(0, Math.floor(sec))
  const d = Math.floor(total / 86400)
  const h = Math.floor((total % 86400) / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const parts: string[] = []
  if (d) parts.push(`${d}天`)
  if (h) parts.push(`${h}小时`)
  if (m) parts.push(`${m}分`)
  if (s || parts.length === 0) parts.push(`${s}秒`)
  return parts.join('')
}

/** 完整展示文案：按上报时间 · 时限5分（仅 report/ping 且设定了时限时追加）。 */
export function onlineModeText(mode?: string | null, timeoutSec = 0): string {
  const normalized = normalizeOnlineMode(mode)
  const base = onlineModeLabel(normalized)
  if ((normalized === 'report' || normalized === 'ping') && timeoutSec > 0) {
    return `${base} · 时限${formatTimeoutDuration(timeoutSec)}`
  }
  return base
}
