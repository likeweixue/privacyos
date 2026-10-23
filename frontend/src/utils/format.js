import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/zh-cn'

dayjs.extend(relativeTime)
dayjs.locale('zh-cn')

export function formatBytes(value, digits = 1) {
  const n = Number(value || 0)
  if (!n) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1)
  const size = n / Math.pow(1024, i)
  const fixed = i === 0 ? 0 : size >= 100 ? 0 : digits
  return `${size.toFixed(fixed)} ${units[i]}`
}

export function formatPercent(value, digits = 1) {
  const n = Number(value || 0)
  return `${n.toFixed(digits)}%`
}

export function formatDate(value, pattern = 'YYYY-MM-DD HH:mm') {
  if (!value) return '—'
  const d = dayjs(value)
  return d.isValid() ? d.format(pattern) : '—'
}

export function formatRelative(value) {
  if (!value) return '—'
  const d = dayjs(value)
  return d.isValid() ? d.fromNow() : '—'
}

export function formatDuration(seconds) {
  const s = Number(seconds || 0)
  if (s <= 0) return '—'
  const days = Math.floor(s / 86400)
  const hours = Math.floor((s % 86400) / 3600)
  const minutes = Math.floor((s % 3600) / 60)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${minutes}m`
  return `${minutes}m`
}

export function formatUptime(seconds) {
  const s = Number(seconds || 0)
  const days = Math.floor(s / 86400)
  const hours = Math.floor((s % 86400) / 3600)
  const minutes = Math.floor((s % 3600) / 60)
  const parts = []
  if (days) parts.push(`${days} 天`)
  if (hours) parts.push(`${hours} 小时`)
  if (!days && minutes) parts.push(`${minutes} 分钟`)
  return parts.join(' ') || '刚刚'
}

export function formatSpeed(bytesPerSecond) {
  return `${formatBytes(bytesPerSecond, 1)}/s`
}

export function shortID(id, length = 12) {
  return (id || '').replace('sha256:', '').slice(0, length)
}

export function fileExt(name = '') {
  const idx = name.lastIndexOf('.')
  return idx > 0 ? name.slice(idx + 1).toLowerCase() : ''
}

export function dateFromNow(days) {
  return dayjs().add(days, 'day').format('YYYY-MM-DD')
}
