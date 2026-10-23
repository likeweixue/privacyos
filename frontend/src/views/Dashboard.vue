<template>
  <div class="pos-page">
    <PageHeader :title="t('dashboard.title')" :subtitle="`${t('dashboard.welcome')}，${auth.displayName}`">
      <el-tag effect="plain" type="success">{{ auth.editionName }}</el-tag>
      <el-button :icon="Refresh" :loading="loading" @click="load">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <div class="cards">
      <StatCard
        :label="t('dashboard.cpu')"
        :value="snapshot.cpu ?? 0"
        suffix="%"
        icon="Cpu"
        color="#2dd4bf"
        :percent="snapshot.cpu ?? 0"
        :hint="`${snapshot.cores || 0} ${t('dashboard.cores')} · ${t('dashboard.load')} ${loadText}`"
      />
      <StatCard
        :label="t('dashboard.memory')"
        :value="snapshot.mem?.percent ?? 0"
        suffix="%"
        icon="Coin"
        color="#60a5fa"
        :percent="snapshot.mem?.percent ?? 0"
        :hint="`${formatBytes(snapshot.mem?.used || 0)} / ${formatBytes(snapshot.mem?.total || 0)}`"
      />
      <StatCard
        :label="t('dashboard.disk')"
        :value="diskPercent"
        suffix="%"
        icon="Coin"
        color="#c084fc"
        :percent="diskPercent"
        :hint="diskHint"
      />
      <StatCard
        :label="t('dashboard.network')"
        :value="formatSpeed(snapshot.net?.rx_rate || 0)"
        icon="Connection"
        color="#fbbf24"
        :hint="`${t('dashboard.up')} ${formatSpeed(snapshot.net?.tx_rate || 0)}`"
      />
    </div>

    <div class="grid-main">
      <section class="pos-panel chart-panel">
        <div class="panel-head">
          <div>
            <div class="pos-card-title">{{ t('dashboard.network') }}</div>
            <div class="pos-hint">{{ t('monitor.refreshHint') }}</div>
          </div>
          <div class="legend">
            <span><i class="dot down" />{{ t('dashboard.down') }} {{ formatSpeed(snapshot.net?.rx_rate || 0) }}</span>
            <span><i class="dot up" />{{ t('dashboard.up') }} {{ formatSpeed(snapshot.net?.tx_rate || 0) }}</span>
          </div>
        </div>
        <EChart :option="chartOption" :height="244" />
      </section>

      <section class="pos-panel quick-panel">
        <div class="panel-head">
          <div class="pos-card-title">{{ t('dashboard.quickAccess') }}</div>
        </div>
        <div class="quick-grid">
          <button class="quick" type="button" @click="$router.push({ name: 'files' })">
            <el-icon :size="22"><FolderOpened /></el-icon><span>{{ t('nav.files') }}</span>
          </button>
          <button class="quick" type="button" @click="$router.push({ name: 'photos' })">
            <el-icon :size="22"><Picture /></el-icon><span>{{ t('nav.photos') }}</span>
          </button>
          <button class="quick" type="button" @click="$router.push({ name: 'movies' })">
            <el-icon :size="22"><VideoCamera /></el-icon><span>{{ t('nav.movies') }}</span>
          </button>
          <button class="quick" type="button" @click="$router.push({ name: 'docker' })">
            <el-icon :size="22"><Box /></el-icon><span>{{ t('nav.docker') }}</span>
          </button>
        </div>
        <div class="divider" />
        <UsageBar
          :label="t('dashboard.storageUsage')"
          :used="summary.usage?.used_bytes || 0"
          :quota="summary.usage?.quota_bytes || 0"
        />
        <div class="mini-stats">
          <div><span>{{ t('dashboard.photos') }}</span><strong>{{ summary.counts?.photos ?? 0 }}</strong></div>
          <div><span>{{ t('dashboard.movies') }}</span><strong>{{ summary.counts?.movies ?? 0 }}</strong></div>
          <div><span>{{ t('dashboard.shares') }}</span><strong>{{ summary.counts?.shares ?? 0 }}</strong></div>
          <div><span>{{ t('dashboard.users') }}</span><strong>{{ summary.counts?.users ?? 0 }}</strong></div>
        </div>
      </section>
    </div>

    <div class="grid-bottom">
      <section class="pos-panel">
        <div class="panel-head">
          <div class="pos-card-title">{{ t('dashboard.alerts') }}</div>
          <el-tag v-if="alertCount" type="warning" effect="plain">{{ alertCount }}</el-tag>
          <el-tag v-else type="success" effect="plain">{{ t('dashboard.systemHealthy') }}</el-tag>
        </div>
        <div v-if="!alerts.length" class="health">
          <el-icon :size="28" color="#34d399"><CircleCheck /></el-icon>
          <div>
            <strong>{{ t('dashboard.noAlerts') }}</strong>
            <p>{{ t('dashboard.systemHealthy') }}</p>
          </div>
        </div>
        <div v-for="alert in alerts" :key="alert.key" class="alert-row">
          <el-icon :class="alert.level" :size="17"><WarningFilled /></el-icon>
          <div class="alert-text">
            <strong>{{ alert.title }}</strong>
            <span>{{ alert.message }}</span>
          </div>
          <span class="alert-time">{{ formatRelative(alert.created_at) }}</span>
        </div>
      </section>

      <section class="pos-panel">
        <div class="panel-head">
          <div class="pos-card-title">{{ t('dashboard.recentActivity') }}</div>
          <router-link v-if="auth.isAdmin" :to="{ name: 'logs' }" class="link">{{ t('dashboard.viewAll') }}</router-link>
        </div>
        <div v-if="!activity.length" class="pos-hint" style="padding: 18px 0">{{ t('dashboard.activityEmpty') }}</div>
        <div v-for="item in activity" :key="item.id" class="activity-row">
          <span class="cat" :class="item.category">{{ categoryIcon(item.category) }}</span>
          <div class="activity-text">
            <strong>{{ item.detail || item.action }}</strong>
            <span>{{ item.target }} · {{ item.username }}</span>
          </div>
          <span class="activity-time">{{ formatRelative(item.created_at) }}</span>
        </div>
      </section>
    </div>

    <div class="foot-stats">
      <span><el-icon><Timer /></el-icon>{{ t('dashboard.uptime') }} {{ formatUptime(snapshot.uptime || 0) }}</span>
      <span><el-icon><Cpu /></el-icon>{{ snapshot.cpu_model || '—' }}</span>
      <span><el-icon><Monitor /></el-icon>{{ t('dashboard.processes') }} {{ snapshot.processes || 0 }}</span>
      <span class="spacer" />
      <span>v{{ auth.version }} · {{ auth.editionName }}</span>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import {
  Refresh, CircleCheck, WarningFilled, FolderOpened, Picture, VideoCamera, Box, Timer, Cpu, Monitor
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import StatCard from '../components/StatCard.vue'
import EChart from '../components/EChart.vue'
import UsageBar from '../components/UsageBar.vue'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { formatBytes, formatSpeed, formatRelative, formatUptime } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const ui = useUiStore()

const loading = ref(false)
const summary = ref({ counts: {}, usage: {}, snapshot: {} })
const snapshot = ref({})
const activity = ref([])
const history = ref([])
let pollTimer = null

const alerts = computed(() => {
  const raised = (summary.value.alerts || []).map((a) => ({ ...a, key: a.key || `alert-${a.id}` }))
  const disk = (summary.value.disk_alerts || []).map((d) => ({
    key: `disk-${d.device}`,
    level: d.level,
    title: t('dashboard.diskAlerts'),
    message: `${d.device} · ${d.message}`,
    created_at: new Date().toISOString()
  }))
  return [...raised, ...disk]
})
const alertCount = computed(() => alerts.value.length)
const loadText = computed(() => (snapshot.value.load || []).map((n) => Number(n).toFixed(2)).join(' / ') || '—')
const diskPercent = computed(() => {
  const disks = snapshot.value.disks || []
  const data = disks.find((d) => d.mountpoint === '/data') || disks[0]
  return data ? Number(data.percent || 0) : 0
})
const diskHint = computed(() => {
  const disks = snapshot.value.disks || []
  const data = disks.find((d) => d.mountpoint === '/data') || disks[0]
  if (!data) return '—'
  return `${data.mountpoint} · ${formatBytes(data.used || 0)} / ${formatBytes(data.total || 0)}`
})

const chartOption = computed(() => {
  const text = ui.theme === 'dark' ? '#8ba1b4' : '#5c7186'
  const grid = ui.theme === 'dark' ? 'rgba(139,161,180,.12)' : 'rgba(92,113,134,.15)'
  const labels = history.value.map((h) => new Date(h.time).toLocaleTimeString())
  return {
    backgroundColor: 'transparent',
    grid: { left: 42, right: 14, top: 20, bottom: 24 },
    tooltip: { trigger: 'axis', valueFormatter: (v) => formatSpeed(v) },
    xAxis: {
      type: 'category',
      data: labels,
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
        name: t('dashboard.down'),
        type: 'line',
        smooth: true,
        symbol: 'none',
        data: history.value.map((h) => h.net?.rx_rate || 0),
        lineStyle: { width: 2, color: '#2dd4bf' },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(45,212,191,.32)' },
              { offset: 1, color: 'rgba(45,212,191,0)' }
            ]
          }
        }
      },
      {
        name: t('dashboard.up'),
        type: 'line',
        smooth: true,
        symbol: 'none',
        data: history.value.map((h) => h.net?.tx_rate || 0),
        lineStyle: { width: 2, color: '#60a5fa' },
        areaStyle: {
          color: {
            type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(96,165,250,.25)' },
              { offset: 1, color: 'rgba(96,165,250,0)' }
            ]
          }
        }
      }
    ]
  }
})

async function load(showError = false) {
  loading.value = true
  try {
    const [payload, entries] = await Promise.all([
      api.dashboard.summary(),
      api.dashboard.activity({ limit: 8 }).catch(() => [])
    ])
    summary.value = payload
    snapshot.value = payload.snapshot || {}
    activity.value = entries || []
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

function categoryIcon(category) {
  const map = {
    auth: 'Lock', file: 'FolderOpened', user: 'User', photo: 'Picture',
    movie: 'VideoCamera', docker: 'Box', storage: 'Coin', system: 'Setting',
    destroy: 'Delete', network: 'Connection', app: 'Grid'
  }
  return map[category] || 'Document'
}
</script>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}
.grid-main {
  display: grid;
  grid-template-columns: minmax(0, 1.55fr) minmax(0, 1fr);
  gap: 16px;
}
.grid-bottom {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
  gap: 16px;
}
section.pos-panel {
  padding: 18px 20px;
}
.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.legend {
  display: flex;
  gap: 14px;
  color: var(--pos-muted);
  font-size: 11.5px;
}
.legend .dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  margin-right: 5px;
}
.dot.down { background: #2dd4bf; }
.dot.up { background: #60a5fa; }
.quick-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}
.quick {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 14px;
  border-radius: 12px;
  border: 1px solid var(--pos-border);
  background: var(--pos-panel-2);
  color: var(--pos-text);
  font-size: 12.5px;
  cursor: pointer;
  transition: all 0.16s ease;
}
.quick:hover {
  border-color: rgba(45, 212, 191, 0.45);
  color: var(--pos-accent);
  transform: translateY(-1px);
}
.quick .el-icon { color: var(--pos-accent); }
.divider {
  height: 1px;
  background: var(--pos-border);
  margin: 16px 0;
}
.mini-stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin-top: 16px;
}
.mini-stats div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px;
  border-radius: 10px;
  background: var(--pos-panel-2);
}
.mini-stats span { font-size: 10.5px; color: var(--pos-muted); }
.mini-stats strong { font-size: 18px; font-variant-numeric: tabular-nums; }
.health {
  display: flex;
  gap: 14px;
  align-items: center;
  padding: 16px;
  border-radius: 12px;
  background: rgba(52, 211, 153, 0.06);
  border: 1px solid rgba(52, 211, 153, 0.16);
}
.health strong { font-size: 13px; }
.health p { margin: 4px 0 0; color: var(--pos-muted); font-size: 12px; }
.alert-row {
  display: flex;
  gap: 10px;
  align-items: flex-start;
  padding: 11px 0;
  border-bottom: 1px dashed var(--pos-border);
}
.alert-row:last-child { border-bottom: none; }
.alert-row .el-icon.warning { color: var(--pos-warning); }
.alert-row .el-icon.critical { color: var(--pos-danger); }
.alert-text { flex: 1; display: flex; flex-direction: column; gap: 3px; }
.alert-text strong { font-size: 12.5px; }
.alert-text span { font-size: 11.5px; color: var(--pos-muted); line-height: 1.55; }
.alert-time { font-size: 10.5px; color: var(--pos-muted); white-space: nowrap; }
.activity-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 9px 0;
  border-bottom: 1px dashed var(--pos-border);
}
.activity-row:last-child { border-bottom: none; }
.cat {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  background: var(--pos-panel-2);
  color: var(--pos-muted);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  text-transform: uppercase;
}
.activity-text { flex: 1; display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.activity-text strong { font-size: 12.5px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.activity-text span { font-size: 11px; color: var(--pos-muted); }
.activity-time { font-size: 10.5px; color: var(--pos-muted); white-space: nowrap; }
.link { font-size: 12px; }
.foot-stats {
  display: flex;
  align-items: center;
  gap: 18px;
  color: var(--pos-muted);
  font-size: 11.5px;
  padding: 2px 2px 8px;
  flex-wrap: wrap;
}
.foot-stats span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.foot-stats .spacer { flex: 1; }

@media (max-width: 1200px) {
  .cards { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .grid-main, .grid-bottom { grid-template-columns: 1fr; }
}
@media (max-width: 640px) {
  .cards { grid-template-columns: 1fr; }
  .mini-stats { grid-template-columns: repeat(2, 1fr); }
}
</style>
