<template>
  <div class="AdminPage DashboardPage">
    <header class="AdminHeader">
      <div>
        <h2>{{ t('dashboard.title') }}</h2>
        <p>{{ t('dashboard.welcome') }}，{{ auth.displayName }} · {{ t('dashboard.welcomeBack') }}</p>
      </div>
      <div class="AdminHeader-actions">
        <el-tag effect="plain" type="success">{{ auth.editionName }}</el-tag>
        <el-button :icon="Refresh" :loading="loading" @click="load(true)">{{ t('common.refresh') }}</el-button>
      </div>
    </header>

    <section class="Widget DashboardWidget StatusWidget">
      <div class="Widget-header">
        <div>
          <h3>{{ t('dashboard.systemStatus') }}</h3>
          <span class="Widget-subtitle">{{ snapshot.hostname || 'privacyos' }} · {{ t('dashboard.subtitle') }}</span>
        </div>
        <el-dropdown trigger="click" @command="onTool">
          <button type="button" class="Button Button--small">
            {{ t('dashboard.tools') }}<el-icon :size="12"><ArrowDown /></el-icon>
          </button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="cache">{{ t('dashboard.clearCache') }}</el-dropdown-item>
              <el-dropdown-item command="info">{{ t('dashboard.systemInfo') }}</el-dropdown-item>
              <el-dropdown-item command="advanced" divided>
                {{ advanced ? t('dashboard.hideAdvanced') : t('dashboard.showAdvanced') }}
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
      <div class="StatusWidget-body">
        <div v-for="row in statusRows" :key="row.key" class="Status-row">
          <span class="Status-label">{{ row.label }}</span>
          <span class="Status-value">{{ row.value }}</span>
          <span class="Status-indicator" :class="`is-${row.level}`" aria-hidden="true" />
          <span class="Status-state">{{ row.state }}</span>
        </div>
        <template v-if="advanced">
          <div v-for="row in advancedRows" :key="row.key" class="Status-row Status-row--advanced">
            <span class="Status-label">{{ row.label }}</span>
            <span class="Status-value">{{ row.value }}</span>
            <span />
            <span class="Status-state">{{ row.state }}</span>
          </div>
        </template>
      </div>
    </section>

    <div class="Dashboard-grid">
      <div class="Dashboard-main">
        <section class="Widget DashboardWidget StatisticsWidget">
          <div class="Widget-header">
            <h3>{{ t('dashboard.statistics') }}</h3>
            <span class="Widget-subtitle">{{ t('dashboard.live') }}</span>
          </div>
          <div class="Stats-grid">
            <div v-for="stat in stats" :key="stat.key" class="Stat">
              <span>{{ stat.label }}</span>
              <strong>{{ stat.value }}</strong>
            </div>
          </div>
        </section>

        <section class="Widget DashboardWidget NetworkWidget">
          <div class="Widget-header">
            <h3>{{ t('dashboard.network') }}</h3>
            <div class="Widget-legend">
              <span><i class="dot down" />{{ t('dashboard.down') }} {{ formatSpeed(snapshot.net?.rx_rate || 0) }}</span>
              <span><i class="dot up" />{{ t('dashboard.up') }} {{ formatSpeed(snapshot.net?.tx_rate || 0) }}</span>
            </div>
          </div>
          <div class="Widget-body">
            <EChart :option="chartOption" :height="218" />
          </div>
        </section>

        <section class="Widget DashboardWidget ActivityWidget">
          <div class="Widget-header">
            <h3>{{ t('dashboard.recentActivity') }}</h3>
            <router-link v-if="auth.isAdmin" :to="{ name: 'logs' }" class="Widget-link">{{ t('dashboard.viewLogs') }}</router-link>
          </div>
          <div class="Widget-body Widget-body--flush">
            <div v-if="!activity.length" class="Widget-empty">{{ t('dashboard.activityEmpty') }}</div>
            <div v-for="item in activity" :key="item.id" class="Activity-row">
              <span class="Activity-icon"><el-icon :size="13"><component :is="categoryIcon(item.category)" /></el-icon></span>
              <div class="Activity-text">
                <strong>{{ item.detail || item.action }}</strong>
                <span>{{ item.target }} · {{ item.username }}</span>
              </div>
              <time>{{ formatRelative(item.created_at) }}</time>
            </div>
          </div>
        </section>
      </div>

      <div class="Dashboard-side">
        <section class="Widget DashboardWidget AnnouncementsWidget">
          <div class="Widget-header">
            <h3>{{ t('dashboard.announcements') }}</h3>
            <span v-if="unreadCount" class="Widget-badge">{{ unreadCount }}</span>
          </div>
          <div class="Widget-body Widget-body--flush">
            <div v-if="!notifications.length" class="Widget-empty">{{ t('dashboard.noAnnouncements') }}</div>
            <article v-for="notice in notifications" :key="notice.id" class="Announcement">
              <span class="Status-indicator" :class="`is-${notice.level === 'critical' ? 'danger' : notice.level === 'warning' ? 'warning' : 'ok'}`" aria-hidden="true" />
              <div>
                <strong>{{ notice.title }}</strong>
                <p>{{ notice.message }}</p>
                <time>{{ formatRelative(notice.created_at) }}</time>
              </div>
            </article>
            <div class="SecurityTip">
              <strong>{{ t('dashboard.securityTipTitle') }}</strong>
              <p>{{ t('dashboard.securityTip') }}</p>
            </div>
          </div>
        </section>

        <section class="Widget DashboardWidget ExtensionsWidget">
          <div class="Widget-header">
            <h3>{{ t('dashboard.extensions') }}</h3>
            <span class="Widget-subtitle">{{ extensionApps.length }} {{ t('dashboard.enabledModules') }}</span>
          </div>
          <div class="Widget-body">
            <div class="Widget-section-title">
              <span>{{ t('dashboard.attention') }}</span>
              <span v-if="alerts.length" class="Widget-badge">{{ alerts.length }}</span>
            </div>
            <div v-if="!alerts.length" class="Attention-ok">
              <el-icon :size="15"><CircleCheck /></el-icon>
              <span>{{ t('dashboard.noAlerts') }}</span>
            </div>
            <div v-for="alert in alerts.slice(0, 3)" :key="alert.key" class="Attention-row">
              <el-icon :size="14"><WarningFilled /></el-icon>
              <div>
                <strong>{{ alert.title }}</strong>
                <span>{{ alert.message }}</span>
              </div>
            </div>

            <div class="Widget-section-title Widget-section-title--spaced">
              <span>{{ t('dashboard.allExtensions') }}</span>
            </div>
            <div class="Extension-grid">
              <button
                v-for="app in extensionApps"
                :key="app.key"
                type="button"
                class="Extension"
                @click="$router.push({ name: app.key })"
              >
                <span class="ExtensionIcon" :style="{ color: app.color }">
                  <el-icon :size="15"><component :is="app.icon" /></el-icon>
                </span>
                <span class="ExtensionInfo">
                  <strong>{{ t(app.titleKey) }}</strong>
                  <small>{{ t(groupLabel(app.group)) }}</small>
                </span>
                <span
                  class="Status-indicator"
                  :class="isEnabled(app) ? 'is-ok' : 'is-disabled'"
                  :title="isEnabled(app) ? t('common.enabled') : t('common.disabled')"
                  aria-hidden="true"
                />
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>

    <footer class="AdminFooter">
      <span><el-icon><Timer /></el-icon>{{ t('dashboard.uptime') }} {{ formatUptime(snapshot.uptime || 0) }}</span>
      <span><el-icon><Cpu /></el-icon>{{ snapshot.cpu_model || '—' }}</span>
      <span><el-icon><Monitor /></el-icon>{{ t('dashboard.processes') }} {{ snapshot.processes || 0 }}</span>
      <span class="spacer" />
      <span>PrivacyOS v{{ auth.version }} · Debian 13.7</span>
    </footer>

    <el-dialog v-model="infoOpen" :title="t('dashboard.systemInfo')" width="520px">
      <el-descriptions :column="1" border size="small">
        <el-descriptions-item :label="t('dashboard.version')">PrivacyOS v{{ auth.version }} · {{ auth.editionName }}</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.os')">Debian GNU/Linux 13.7 (Trixie)</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.database')">SQLite · WAL</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.cpuModel')">{{ snapshot.cpu_model || '—' }}</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.load')">{{ loadText }}</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.memory')">{{ formatBytes(snapshot.mem?.used || 0) }} / {{ formatBytes(snapshot.mem?.total || 0) }}</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.processes')">{{ snapshot.processes || 0 }}</el-descriptions-item>
        <el-descriptions-item :label="t('dashboard.uptime')">{{ formatUptime(snapshot.uptime || 0) }}</el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button type="primary" @click="infoOpen = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import {
  ArrowDown, CircleCheck, Cpu, Monitor, Refresh, Timer, WarningFilled
} from '@element-plus/icons-vue'
import EChart from '../components/EChart.vue'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { visibleApps } from '../apps'
import { formatBytes, formatSpeed, formatRelative, formatUptime } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const ui = useUiStore()

const loading = ref(false)
const advanced = ref(false)
const infoOpen = ref(false)
const summary = ref({ counts: {}, usage: {}, snapshot: {}, alerts: [], disk_alerts: [], notifications: [], recent: [] })
const snapshot = ref({})
const history = ref([])
let pollTimer = null

const counts = computed(() => summary.value.counts || {})
const usage = computed(() => summary.value.usage || {})
const notifications = computed(() => {
  const seen = new Set()
  return (summary.value.notifications || []).filter((notice) => {
    const key = `${notice.title}|${notice.message}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  }).slice(0, 4)
})
const unreadCount = computed(() => summary.value.unread || 0)
const activity = computed(() => (summary.value.recent || []).slice(0, 8))
const extensionApps = computed(() => visibleApps(auth).filter((app) => app.key !== 'dashboard'))

const alerts = computed(() => {
  const raised = (summary.value.alerts || []).map((a) => ({ ...a, key: a.key || `alert-${a.id}` }))
  const disk = (summary.value.disk_alerts || []).map((d) => ({
    key: `disk-${d.device}`,
    level: d.level,
    title: t('dashboard.diskAlerts'),
    message: `${d.device} · ${d.message}`
  }))
  return [...raised, ...disk]
})

const loadText = computed(() => (snapshot.value.load || []).map((n) => Number(n).toFixed(2)).join(' / ') || '—')
const disk = computed(() => {
  const disks = snapshot.value.disks || []
  return disks.find((d) => d.mountpoint === '/data') || disks[0] || null
})
const diskPercent = computed(() => Number(disk.value?.percent || 0))
const diskHint = computed(() => {
  if (!disk.value) return '—'
  return `${formatBytes(disk.value.used || 0)} / ${formatBytes(disk.value.total || 0)}`
})
const diskLevel = computed(() => (diskPercent.value >= 90 ? 'danger' : diskPercent.value >= 75 ? 'warning' : 'ok'))

const statusRows = computed(() => [
  {
    key: 'app', label: t('dashboard.version'),
    value: `v${auth.version} · ${auth.editionName}`,
    level: 'ok', state: t('dashboard.statusNormal')
  },
  {
    key: 'os', label: t('dashboard.os'),
    value: 'Debian 13.7 Trixie', level: 'ok', state: t('dashboard.statusNormal')
  },
  {
    key: 'db', label: t('dashboard.database'),
    value: 'SQLite · WAL', level: 'ok', state: t('dashboard.statusNormal')
  },
  {
    key: 'docker', label: 'Docker',
    value: auth.features.docker === false ? t('common.disabled') : t('common.enabled'),
    level: auth.features.docker === false ? 'disabled' : 'ok',
    state: auth.features.docker === false ? t('dashboard.statusDisabled') : t('dashboard.statusNormal')
  },
  {
    key: 'storage', label: t('dashboard.storage'),
    value: `${diskPercent.value}% · ${diskHint.value}`,
    level: diskLevel.value,
    state: diskLevel.value === 'ok' ? t('dashboard.statusNormal') : t('dashboard.statusWarning')
  },
  {
    key: 'uptime', label: t('dashboard.uptime'),
    value: formatUptime(snapshot.value.uptime || 0),
    level: 'ok', state: t('dashboard.statusRunning')
  }
])

const advancedRows = computed(() => [
  { key: 'cpu', label: t('dashboard.cpuModel'), value: snapshot.value.cpu_model || '—', state: t('dashboard.statusNormal') },
  { key: 'load', label: t('dashboard.load'), value: loadText.value, state: t('dashboard.statusNormal') },
  { key: 'mem', label: t('dashboard.memory'), value: `${formatBytes(snapshot.value.mem?.used || 0)} / ${formatBytes(snapshot.value.mem?.total || 0)}`, state: t('dashboard.statusNormal') },
  { key: 'proc', label: t('dashboard.processes'), value: snapshot.value.processes || 0, state: t('dashboard.statusNormal') }
])

const stats = computed(() => [
  { key: 'users', label: t('dashboard.users'), value: counts.value.users ?? 0 },
  { key: 'files', label: t('dashboard.files'), value: usage.value.file_count ?? 0 },
  { key: 'photos', label: t('dashboard.photos'), value: counts.value.photos ?? 0 },
  { key: 'movies', label: t('dashboard.movies'), value: counts.value.movies ?? 0 },
  { key: 'shares', label: t('dashboard.shares'), value: counts.value.shares ?? 0 }
])

const chartOption = computed(() => {
  const dark = ui.theme === 'dark'
  const text = dark ? '#8b9bab' : '#6b7d8d'
  const grid = dark ? 'rgba(139,155,171,.16)' : 'rgba(107,125,141,.18)'
  const labels = history.value.map((h) => new Date(h.time).toLocaleTimeString())
  return {
    backgroundColor: 'transparent',
    grid: { left: 44, right: 14, top: 16, bottom: 22 },
    tooltip: { trigger: 'axis', valueFormatter: (v) => formatSpeed(v) },
    xAxis: {
      type: 'category', data: labels,
      axisLine: { lineStyle: { color: grid } },
      axisLabel: { color: text, fontSize: 10, interval: Math.max(0, Math.floor(labels.length / 6)) },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: text, fontSize: 10, formatter: (v) => formatBytes(v, 1) },
      splitLine: { lineStyle: { color: grid } }
    },
    series: [
      {
        name: t('dashboard.down'), type: 'line', smooth: true, symbol: 'none',
        data: history.value.map((h) => h.net?.rx_rate || 0),
        lineStyle: { width: 2, color: '#0d9488' },
        areaStyle: { color: 'rgba(13,148,136,.1)' }
      },
      {
        name: t('dashboard.up'), type: 'line', smooth: true, symbol: 'none',
        data: history.value.map((h) => h.net?.tx_rate || 0),
        lineStyle: { width: 2, color: '#2563eb' },
        areaStyle: { color: 'rgba(37,99,235,.07)' }
      }
    ]
  }
})

async function load(showError = false) {
  loading.value = true
  try {
    const payload = await api.dashboard.summary()
    summary.value = payload
    snapshot.value = payload.snapshot || {}
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  try {
    const data = await api.monitor.history(30)
    history.value = data || []
    if (history.value.length) {
      const last = history.value[history.value.length - 1]
      snapshot.value = { ...snapshot.value, ...last }
    }
  } catch (error) {
    // Chart history is optional.
  }
}

function onTool(command) {
  if (command === 'cache') {
    load(true)
    ElMessage.success(t('dashboard.cacheCleared'))
  } else if (command === 'info') {
    infoOpen.value = true
  } else if (command === 'advanced') {
    advanced.value = !advanced.value
  }
}

function isEnabled(app) {
  return !(app.feature && auth.features[app.feature] === false)
}

function groupLabel(group) {
  return { core: 'nav.groups.core', features: 'nav.groups.features', system: 'nav.groups.system' }[group] || 'nav.admin'
}

function categoryIcon(category) {
  const map = {
    auth: 'Lock', file: 'FolderOpened', user: 'User', photo: 'Picture',
    movie: 'VideoCamera', docker: 'Box', storage: 'Coin', system: 'Setting',
    destroy: 'Delete', network: 'Connection', app: 'Grid'
  }
  return map[category] || 'Document'
}

onMounted(() => {
  load(true)
  loadHistory()
  pollTimer = setInterval(() => {
    loadHistory()
    load(false)
  }, 5000)
})
onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
watch(() => ui.theme, () => {})
</script>

<style scoped>
.AdminPage { display: flex; flex-direction: column; gap: 16px; }
.AdminHeader {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.AdminHeader h2 { margin: 0; font-size: 20px; line-height: 1.3; font-weight: 650; }
.AdminHeader p { margin: 4px 0 0; color: var(--pos-muted); font-size: 12.5px; }
.AdminHeader-actions { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }

.Widget {
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  border-radius: 3px;
}
.Widget-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 16px;
  border-bottom: 1px solid var(--pos-border);
}
.Widget-header h3 { margin: 0; font-size: 13px; font-weight: 650; }
.Widget-subtitle { display: block; margin-top: 2px; font-size: 10.5px; color: var(--pos-muted); }
.Widget-legend { display: flex; gap: 14px; color: var(--pos-muted); font-size: 10.5px; }
.Widget-legend .dot { display: inline-block; width: 6px; height: 6px; border-radius: 50%; margin-right: 4px; }
.dot.down { background: #0d9488; }
.dot.up { background: #2563eb; }
.Widget-body { padding: 14px 16px; }
.Widget-body--flush { padding: 0 16px; }
.Widget-link { font-size: 11.5px; }
.Widget-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 2px;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  font-size: 10.5px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.Widget-empty { padding: 22px 0; text-align: center; color: var(--pos-muted); font-size: 12px; }
.Widget-section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.5px;
  text-transform: uppercase;
  color: var(--pos-muted);
  margin-bottom: 8px;
}
.Widget-section-title--spaced { margin-top: 18px; }

.Button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  height: 28px;
  padding: 0 9px;
  border: 1px solid var(--pos-border);
  border-radius: 3px;
  background: var(--pos-panel);
  color: var(--pos-text);
  font-size: 11.5px;
  cursor: pointer;
}
.Button:hover { background: var(--pos-panel-2); }
.Button--small { height: 27px; }

.StatusWidget-body { padding: 0; }
.Status-row {
  display: grid;
  grid-template-columns: 128px minmax(0, 1fr) 8px 76px;
  gap: 12px;
  align-items: center;
  padding: 10px 16px;
  border-bottom: 1px solid var(--pos-border);
  font-size: 12px;
}
.Status-row:last-child { border-bottom: none; }
.Status-row--advanced { background: var(--pos-panel-2); }
.Status-label { color: var(--pos-muted); }
.Status-value { color: var(--pos-text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.Status-state { color: var(--pos-muted); font-size: 11px; text-align: right; }
.Status-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--pos-success);
  display: inline-block;
}
.Status-indicator.is-ok { background: var(--pos-success); }
.Status-indicator.is-warning { background: var(--pos-warning); }
.Status-indicator.is-danger { background: var(--pos-danger); }
.Status-indicator.is-disabled { background: var(--pos-muted); opacity: 0.6; }

.Dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.Dashboard-main, .Dashboard-side { display: flex; flex-direction: column; gap: 16px; min-width: 0; }

.Stats-grid { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); }
.Stat { padding: 14px 16px; border-right: 1px solid var(--pos-border); display: flex; flex-direction: column; gap: 3px; }
.Stat:last-child { border-right: none; }
.Stat span { font-size: 10.5px; color: var(--pos-muted); }
.Stat strong { font-size: 21px; font-weight: 650; font-variant-numeric: tabular-nums; letter-spacing: -0.3px; }

.Announcement {
  display: flex;
  gap: 9px;
  padding: 11px 0;
  border-bottom: 1px solid var(--pos-border);
}
.Announcement:last-of-type { border-bottom: none; }
.Announcement .Status-indicator { margin-top: 5px; flex-shrink: 0; }
.Announcement strong { display: block; font-size: 12px; font-weight: 600; margin-bottom: 2px; }
.Announcement p { margin: 0; font-size: 11.5px; line-height: 1.55; color: var(--pos-muted); }
.Announcement time { display: block; margin-top: 4px; font-size: 10px; color: var(--pos-muted); opacity: 0.8; }
.SecurityTip {
  margin: 8px 0 14px;
  padding: 10px 11px;
  background: var(--pos-panel-2);
  border-left: 2px solid var(--pos-accent);
}
.SecurityTip strong { font-size: 11.5px; }
.SecurityTip p { margin: 3px 0 0; font-size: 11px; line-height: 1.55; color: var(--pos-muted); }

.Attention-ok {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 6px 0 8px;
  color: var(--pos-success);
  font-size: 11.5px;
}
.Attention-row { display: flex; gap: 7px; padding: 5px 0; color: var(--pos-warning); }
.Attention-row .el-icon { margin-top: 2px; flex-shrink: 0; }
.Attention-row strong { display: block; font-size: 11.5px; color: var(--pos-text); font-weight: 550; }
.Attention-row span { display: block; font-size: 11px; color: var(--pos-muted); line-height: 1.5; }

.Extension-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}
.Extension {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  padding: 8px 9px;
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  background: var(--pos-panel);
  color: var(--pos-text);
  cursor: pointer;
  text-align: left;
}
.Extension:hover { background: var(--pos-panel-2); }
.ExtensionIcon {
  width: 27px;
  height: 27px;
  flex-shrink: 0;
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-panel-2);
}
.ExtensionInfo { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.ExtensionInfo strong { font-size: 11.5px; font-weight: 550; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ExtensionInfo small { font-size: 9.5px; color: var(--pos-muted); }

.Activity-row {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 0;
  border-bottom: 1px solid var(--pos-border);
}
.Activity-row:last-child { border-bottom: none; }
.Activity-icon {
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  background: var(--pos-panel-2);
  color: var(--pos-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.Activity-text { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
.Activity-text strong { font-size: 11.5px; font-weight: 550; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.Activity-text span { font-size: 10.5px; color: var(--pos-muted); }
.Activity-row time { font-size: 10px; color: var(--pos-muted); white-space: nowrap; }

.AdminFooter {
  display: flex;
  align-items: center;
  gap: 18px;
  flex-wrap: wrap;
  color: var(--pos-muted);
  font-size: 10.5px;
  padding: 2px 1px 6px;
}
.AdminFooter span { display: inline-flex; align-items: center; gap: 5px; }
.AdminFooter .spacer { flex: 1; }

@media (max-width: 1180px) {
  .Dashboard-grid { grid-template-columns: 1fr; }
  .Dashboard-side { display: grid; grid-template-columns: 1fr 1fr; align-items: start; }
}
@media (max-width: 820px) {
  .Stats-grid { grid-template-columns: repeat(3, 1fr); }
  .Stat:nth-child(3) { border-right: none; }
  .Stat:nth-child(n + 4) { border-top: 1px solid var(--pos-border); }
  .Dashboard-side { grid-template-columns: 1fr; }
}
@media (max-width: 620px) {
  .Status-row { grid-template-columns: 96px minmax(0, 1fr) 8px; }
  .Status-state { display: none; }
  .Stats-grid { grid-template-columns: repeat(2, 1fr); }
  .Stat:nth-child(2n) { border-right: none; }
  .Stat:nth-child(n + 3) { border-top: 1px solid var(--pos-border); }
  .Stat:last-child:nth-child(odd) { grid-column: 1 / -1; border-right: none; }
  .Extension-grid { grid-template-columns: 1fr; }
}
</style>
