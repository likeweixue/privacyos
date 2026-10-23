<template>
  <div class="pos-page monitor-page">
    <PageHeader :title="t('monitor.title')" :subtitle="t('monitor.subtitle')">
      <span class="live-badge" :class="{ offline: !live }">
        <span class="live-dot" />
        {{ t('monitor.realtime') }}
      </span>
      <span class="pos-hint refresh-note">{{ t('monitor.refreshHint') }}</span>
      <el-button :icon="Refresh" :loading="loading" @click="refreshAll">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <div v-if="auth.dev" class="dev-note">
      <el-icon :size="15"><InfoFilled /></el-icon>
      <span>{{ t('monitor.devNote') }}</span>
    </div>

    <div class="cards">
      <StatCard
        :label="t('dashboard.cpu')"
        :value="snapshot.cpu ?? 0"
        suffix="%"
        icon="Cpu"
        color="#2dd4bf"
        :percent="Number(snapshot.cpu || 0)"
        :hint="`${snapshot.cores || 0} ${t('dashboard.cores')} · ${t('dashboard.load')} ${loadText}`"
      />
      <StatCard
        :label="t('dashboard.memory')"
        :value="memPercent"
        suffix="%"
        icon="Coin"
        color="#60a5fa"
        :percent="memPercent"
        :hint="`${formatBytes(mem.used || 0)} / ${formatBytes(mem.total || 0)} · ${t('dashboard.free')} ${formatBytes(mem.available || 0)}`"
      />
      <StatCard
        :label="t('monitor.network')"
        :value="formatSpeed(netDown)"
        icon="Connection"
        color="#fbbf24"
        :hint="`${t('dashboard.down')} ${formatSpeed(netDown)} · ${t('dashboard.up')} ${formatSpeed(netUp)}`"
      />
      <StatCard
        :label="t('monitor.diskIO')"
        :value="formatSpeed(diskRead)"
        icon="Files"
        color="#c084fc"
        :hint="`${t('monitor.read')} ${formatSpeed(diskRead)} · ${t('monitor.write')} ${formatSpeed(diskWrite)}`"
      />
    </div>

    <div class="vitals">
      <StatCard
        :label="t('dashboard.uptime')"
        :value="formatDuration(snapshot.uptime || 0)"
        icon="Timer"
        color="#34d399"
        :hint="snapshot.hostname || '—'"
      />
      <StatCard
        :label="t('monitor.load')"
        :value="loadAvg"
        icon="Odometer"
        color="#f472b6"
        :hint="loadText"
      />
      <StatCard
        :label="t('dashboard.processes')"
        :value="processCount"
        icon="Monitor"
        color="#60a5fa"
        :hint="t('monitor.realtime')"
      />
    </div>

    <div class="charts">
      <section class="pos-panel chart-panel">
        <div class="panel-head">
          <div class="panel-title">
            <div class="pos-card-title">{{ t('monitor.history') }}</div>
            <div class="pos-hint">{{ t('monitor.refreshHint') }}</div>
          </div>
          <div class="legend">
            <span><i class="dot cpu" />{{ t('monitor.cpu') }} {{ Number(snapshot.cpu || 0).toFixed(1) }}%</span>
            <span><i class="dot mem" />{{ t('monitor.memory') }} {{ memPercent.toFixed(1) }}%</span>
          </div>
        </div>
        <div class="chart-wrap">
          <EChart :option="cpuOption" :height="230" />
          <div v-if="!samples.length" class="chart-empty pos-hint">
            {{ t(ready ? 'common.empty' : 'common.loading') }}
          </div>
        </div>
      </section>

      <section class="pos-panel chart-panel">
        <div class="panel-head">
          <div class="panel-title">
            <div class="pos-card-title">{{ t('monitor.network') }}</div>
            <div class="pos-hint">{{ t('monitor.refreshHint') }}</div>
          </div>
          <div class="legend">
            <span><i class="dot down" />{{ t('dashboard.down') }} {{ formatSpeed(netDown) }}</span>
            <span><i class="dot up" />{{ t('dashboard.up') }} {{ formatSpeed(netUp) }}</span>
          </div>
        </div>
        <div class="chart-wrap">
          <EChart :option="netOption" :height="230" />
          <div v-if="!samples.length" class="chart-empty pos-hint">
            {{ t(ready ? 'common.empty' : 'common.loading') }}
          </div>
        </div>
      </section>
    </div>

    <div class="bottom-grid">
      <section class="pos-panel temp-panel">
        <div class="panel-head">
          <div class="pos-card-title">{{ t('monitor.temperature') }}</div>
          <el-tag v-if="temps.length" size="small" effect="plain">{{ temps.length }}</el-tag>
        </div>
        <div v-if="!temps.length" class="temp-empty">
          <el-icon :size="22"><Sunny /></el-icon>
          <span class="pos-hint">{{ t('monitor.noTemps') }}</span>
        </div>
        <div v-else class="temp-list">
          <div v-for="(temp, index) in temps" :key="`${temp.name}-${index}`" class="temp-row">
            <div class="temp-meta">
              <span class="temp-name" :title="temp.name">{{ temp.name }}</span>
              <el-tag size="small" effect="plain" :type="tempTag(temp.celsius)">
                {{ Number(temp.celsius || 0).toFixed(1) }} °C
              </el-tag>
            </div>
            <el-progress
              :percentage="tempPercent(temp.celsius)"
              :color="tempColor(temp.celsius)"
              :stroke-width="6"
              :show-text="false"
            />
          </div>
        </div>
      </section>

      <section class="pos-panel proc-panel">
        <div class="panel-head">
          <div class="proc-title">
            <span class="pos-card-title">{{ t('monitor.processes') }}</span>
            <el-tag size="small" effect="plain" type="info">{{ processes.length }}</el-tag>
          </div>
          <div class="proc-tools">
            <el-input
              v-model="processSearch"
              class="proc-search"
              size="small"
              clearable
              :prefix-icon="Search"
              :placeholder="t('common.searchPlaceholder')"
            />
            <el-button size="small" :icon="Refresh" :loading="processesLoading" @click="loadProcesses(true)">
              {{ t('common.refresh') }}
            </el-button>
          </div>
        </div>
        <el-table
          v-loading="processesLoading"
          :data="filteredProcesses"
          :default-sort="defaultSort"
          max-height="380"
          row-key="pid"
        >
          <el-table-column :label="t('monitor.pid')" prop="pid" width="92" sortable>
            <template #default="{ row }"><span class="pos-mono">{{ row.pid }}</span></template>
          </el-table-column>
          <el-table-column :label="t('monitor.processName')" prop="name" min-width="164" sortable show-overflow-tooltip>
            <template #default="{ row }"><span class="proc-name">{{ row.name }}</span></template>
          </el-table-column>
          <el-table-column :label="t('monitor.user')" prop="user" width="112" show-overflow-tooltip />
          <el-table-column :label="t('common.status')" width="92">
            <template #default="{ row }">
              <el-tag size="small" effect="plain" :type="stateTag(row.state)">{{ row.state || '—' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('monitor.cpuUsage')" prop="cpu" width="96" sortable align="right">
            <template #default="{ row }">
              <span class="num" :class="{ hot: Number(row.cpu || 0) >= 50 }">{{ Number(row.cpu || 0).toFixed(1) }}%</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('monitor.memUsage')" prop="mem" width="108" sortable align="right">
            <template #default="{ row }"><span class="num">{{ formatBytes(row.mem) }}</span></template>
          </el-table-column>
          <el-table-column :label="t('monitor.threads')" prop="threads" width="88" sortable align="right" />
          <el-table-column v-if="auth.isAdmin" :label="t('common.actions')" width="124" align="right">
            <template #default="{ row }">
              <el-button link type="danger" size="small" :icon="CircleClose" @click="askKill(row)">
                {{ t('monitor.kill') }}
              </el-button>
            </template>
          </el-table-column>
          <template #empty>
            <EmptyState :title="t('monitor.noProcesses')" icon="Monitor" />
          </template>
        </el-table>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElCheckbox, ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search, CircleClose, InfoFilled, Sunny } from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import StatCard from '../components/StatCard.vue'
import EChart from '../components/EChart.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, errorMessage, tokenStore } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { formatBytes, formatDuration, formatSpeed } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()
const ui = useUiStore()

const snapshot = ref({})
const samples = ref([])
const processes = ref([])
const processSearch = ref('')
const loading = ref(false)
const processesLoading = ref(false)
const live = ref(false)
const ready = ref(false)
const forceKill = ref(false)
const defaultSort = { prop: 'cpu', order: 'descending' }

let source = null
let pollTimer = null

const mem = computed(() => snapshot.value.mem || {})
const net = computed(() => snapshot.value.net || {})
const diskIO = computed(() => snapshot.value.disk_io || {})
const loadValues = computed(() => (Array.isArray(snapshot.value.load) ? snapshot.value.load : []))
const temps = computed(() => (Array.isArray(snapshot.value.temps) ? snapshot.value.temps : []))

const loadText = computed(() => loadValues.value.map((value) => Number(value).toFixed(2)).join(' / ') || '—')
const loadAvg = computed(() => (loadValues.value.length ? Number(loadValues.value[0]).toFixed(2) : '—'))
const memPercent = computed(() => Number(mem.value.percent || 0))
const netDown = computed(() => Number(net.value.rx_rate || 0))
const netUp = computed(() => Number(net.value.tx_rate || 0))
const diskRead = computed(() => Number(diskIO.value.read_rate || 0))
const diskWrite = computed(() => Number(diskIO.value.write_rate || 0))
const processCount = computed(() => snapshot.value.processes ?? 0)

const filteredProcesses = computed(() => {
  const query = processSearch.value.trim().toLowerCase()
  const list = Array.isArray(processes.value) ? processes.value : []
  if (!query) return list
  return list.filter((row) =>
    String(row.pid).includes(query) ||
    String(row.name || '').toLowerCase().includes(query) ||
    String(row.user || '').toLowerCase().includes(query)
  )
})

const chartLabels = computed(() =>
  samples.value.map((sample) => {
    const date = new Date(sample.time)
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleTimeString()
  })
)
const chartText = computed(() => (ui.theme === 'dark' ? '#8ba1b4' : '#5c7186'))
const chartGrid = computed(() => (ui.theme === 'dark' ? 'rgba(139,161,180,.12)' : 'rgba(92,113,134,.15)'))
const chartTooltip = computed(() => ({
  backgroundColor: ui.theme === 'dark' ? 'rgba(17,26,35,.94)' : 'rgba(255,255,255,.96)',
  borderColor: ui.theme === 'dark' ? '#22303c' : '#dde6ee',
  textStyle: { color: ui.theme === 'dark' ? '#e6edf3' : '#101b26', fontSize: 11 }
}))
const labelInterval = computed(() => Math.max(0, Math.floor(chartLabels.value.length / 6)))

function baseAxisOption() {
  return {
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: chartLabels.value,
      axisLine: { lineStyle: { color: chartGrid.value } },
      axisLabel: { color: chartText.value, fontSize: 10, interval: labelInterval.value },
      axisTick: { show: false }
    }
  }
}

function areaGradient(color) {
  return {
    color: {
      type: 'linear',
      x: 0,
      y: 0,
      x2: 0,
      y2: 1,
      colorStops: [
        { offset: 0, color },
        { offset: 1, color: 'rgba(0,0,0,0)' }
      ]
    }
  }
}

const cpuOption = computed(() => ({
  backgroundColor: 'transparent',
  animationDuration: 300,
  grid: { left: 46, right: 16, top: 22, bottom: 26 },
  tooltip: {
    trigger: 'axis',
    valueFormatter: (value) => `${Number(value || 0).toFixed(1)}%`,
    ...chartTooltip.value
  },
  xAxis: baseAxisOption().xAxis,
  yAxis: {
    type: 'value',
    min: 0,
    max: 100,
    axisLabel: { color: chartText.value, fontSize: 10, formatter: (value) => `${value}%` },
    splitLine: { lineStyle: { color: chartGrid.value } }
  },
  series: [
    {
      name: t('monitor.cpu'),
      type: 'line',
      smooth: true,
      symbol: 'none',
      data: samples.value.map((sample) => Number(sample.cpu || 0)),
      lineStyle: { width: 2, color: '#2dd4bf' },
      areaStyle: areaGradient('rgba(45,212,191,.30)')
    },
    {
      name: t('monitor.memory'),
      type: 'line',
      smooth: true,
      symbol: 'none',
      data: samples.value.map((sample) => Number(sample.mem?.percent || 0)),
      lineStyle: { width: 2, color: '#60a5fa' },
      areaStyle: areaGradient('rgba(96,165,250,.24)')
    }
  ]
}))

const netOption = computed(() => ({
  backgroundColor: 'transparent',
  animationDuration: 300,
  grid: { left: 52, right: 16, top: 22, bottom: 26 },
  tooltip: {
    trigger: 'axis',
    valueFormatter: (value) => formatSpeed(value),
    ...chartTooltip.value
  },
  xAxis: baseAxisOption().xAxis,
  yAxis: {
    type: 'value',
    axisLabel: { color: chartText.value, fontSize: 10, formatter: (value) => formatBytes(value, 0) },
    splitLine: { lineStyle: { color: chartGrid.value } }
  },
  series: [
    {
      name: t('dashboard.down'),
      type: 'line',
      smooth: true,
      symbol: 'none',
      data: samples.value.map((sample) => Number(sample.net?.rx_rate || 0)),
      lineStyle: { width: 2, color: '#2dd4bf' },
      areaStyle: areaGradient('rgba(45,212,191,.30)')
    },
    {
      name: t('dashboard.up'),
      type: 'line',
      smooth: true,
      symbol: 'none',
      data: samples.value.map((sample) => Number(sample.net?.tx_rate || 0)),
      lineStyle: { width: 2, color: '#60a5fa' },
      areaStyle: areaGradient('rgba(96,165,250,.24)')
    }
  ]
}))

function applySnapshot(next, push = true) {
  if (!next || typeof next !== 'object') return
  snapshot.value = next
  if (!push) return
  const time = next.time || new Date().toISOString()
  const last = samples.value[samples.value.length - 1]
  if (last && last.time === time && last.cpu === next.cpu && last.net?.rx_bytes === next.net?.rx_bytes) return
  samples.value = [...samples.value, { ...next, time }].slice(-60)
}

function onMetrics(event) {
  try {
    const payload = JSON.parse(event.data)
    live.value = true
    stopPolling()
    applySnapshot(payload)
  } catch (error) {
    // Ignore malformed frames and keep the previous sample.
  }
}

async function seedHistory() {
  try {
    const rows = await api.monitor.history(60)
    if (Array.isArray(rows) && rows.length) {
      samples.value = rows.slice(-60)
      if (!snapshot.value.time) applySnapshot(rows[rows.length - 1], false)
    }
  } catch (error) {
    // Chart history is optional; the live stream fills the chart.
  }
}

async function loadOverview(showError = true) {
  try {
    const payload = await api.monitor.overview()
    applySnapshot(payload?.snapshot || payload)
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  }
}

async function loadProcesses(showError = true) {
  processesLoading.value = true
  try {
    const rows = await api.monitor.processes()
    processes.value = Array.isArray(rows) ? rows : []
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    processesLoading.value = false
  }
}

async function pollOverview() {
  try {
    const payload = await api.monitor.overview()
    applySnapshot(payload?.snapshot || payload)
  } catch (error) {
    // Keep the last known snapshot while polling.
  }
}

function startPolling() {
  if (pollTimer) return
  pollTimer = setInterval(pollOverview, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function disconnect() {
  if (source) {
    source.removeEventListener('metrics', onMetrics)
    source.close()
    source = null
  }
  live.value = false
}

function connect() {
  disconnect()
  if (typeof EventSource === 'undefined') {
    startPolling()
    return
  }
  try {
    source = new EventSource('/api/v1/monitor/stream?access_token=' + tokenStore.access)
  } catch (error) {
    startPolling()
    return
  }
  source.addEventListener('metrics', onMetrics)
  source.onopen = () => {
    live.value = true
    stopPolling()
  }
  source.onerror = () => {
    live.value = false
    if (source) {
      source.close()
      source = null
    }
    startPolling()
  }
}

async function refreshAll() {
  loading.value = true
  try {
    await Promise.all([
      loadOverview(true),
      loadProcesses(true),
      samples.value.length ? Promise.resolve() : seedHistory()
    ])
    if (!live.value) {
      stopPolling()
      connect()
    }
  } finally {
    loading.value = false
  }
}

function tempPercent(value) {
  return Math.max(0, Math.min(100, Number(value || 0)))
}

function tempColor(value) {
  const celsius = Number(value || 0)
  if (celsius >= 70) return '#f87171'
  if (celsius >= 55) return '#fbbf24'
  return '#2dd4bf'
}

function tempTag(value) {
  const celsius = Number(value || 0)
  if (celsius >= 70) return 'danger'
  if (celsius >= 55) return 'warning'
  return 'success'
}

function stateTag(state) {
  const value = String(state || '').toUpperCase()
  if (value === 'R') return 'success'
  if (value === 'D' || value === 'T') return 'warning'
  if (value === 'Z') return 'danger'
  return 'info'
}

const KillConfirm = defineComponent({
  name: 'KillConfirm',
  props: {
    row: { type: Object, required: true }
  },
  setup(props) {
    return () =>
      h('div', [
        h(
          'p',
          { style: 'margin: 0 0 10px; line-height: 1.6;' },
          t('monitor.killConfirm', { pid: props.row?.pid ?? '' })
        ),
        h(
          'p',
          { class: 'pos-mono', style: 'margin: 0 0 14px; color: var(--pos-muted);' },
          [props.row?.name, props.row?.user, `${t('monitor.pid')} ${props.row?.pid ?? ''}`].filter(Boolean).join(' · ')
        ),
        h(
          ElCheckbox,
          {
            modelValue: forceKill.value,
            'onUpdate:modelValue': (value) => {
              forceKill.value = value
            }
          },
          () => t('monitor.killForce')
        )
      ])
  }
})

async function askKill(row) {
  forceKill.value = false
  try {
    await ElMessageBox.confirm(h(KillConfirm, { row }), t('monitor.kill'), {
      type: 'warning',
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel'),
      confirmButtonClass: 'el-button--danger',
      closeOnClickModal: false
    })
  } catch (error) {
    return
  }
  try {
    await api.monitor.kill(row.pid, forceKill.value)
    ElMessage.success(t('monitor.killed'))
    await loadProcesses(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

onMounted(async () => {
  loading.value = true
  try {
    await Promise.all([seedHistory(), loadOverview(true), loadProcesses(true)])
  } finally {
    loading.value = false
    ready.value = true
  }
  connect()
})

onBeforeUnmount(() => {
  disconnect()
  stopPolling()
})
</script>

<style scoped>
.monitor-page {
  gap: 16px;
}
.cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}
.vitals {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}
.charts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.bottom-grid {
  display: grid;
  grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.7fr);
  gap: 16px;
  align-items: start;
}
section.pos-panel {
  padding: 18px 20px;
}
.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}
.panel-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.live-badge {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 11px;
  border-radius: 999px;
  font-size: 12px;
  color: var(--pos-success);
  background: rgba(52, 211, 153, 0.1);
  border: 1px solid rgba(52, 211, 153, 0.24);
  white-space: nowrap;
}
.live-badge.offline {
  color: var(--pos-muted);
  background: var(--pos-panel-2);
  border-color: var(--pos-border);
}
.live-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: currentColor;
  animation: live-pulse 1.7s ease-in-out infinite;
}
@keyframes live-pulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.35;
    transform: scale(0.75);
  }
}
.refresh-note {
  white-space: nowrap;
}
.dev-note {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 14px;
  border-radius: 10px;
  background: rgba(96, 165, 250, 0.08);
  border: 1px solid rgba(96, 165, 250, 0.2);
  color: var(--pos-muted);
  font-size: 12px;
}
.dev-note .el-icon {
  color: var(--pos-blue);
}
.legend {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  color: var(--pos-muted);
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
}
.legend .dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  margin-right: 5px;
}
.dot.cpu,
.dot.down {
  background: #2dd4bf;
}
.dot.mem,
.dot.up {
  background: #60a5fa;
}
.chart-wrap {
  position: relative;
}
.chart-empty {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}
.temp-list {
  display: flex;
  flex-direction: column;
  gap: 15px;
  max-height: 340px;
  overflow: auto;
  padding-right: 4px;
}
.temp-row {
  display: flex;
  flex-direction: column;
  gap: 7px;
}
.temp-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}
.temp-name {
  font-size: 12.5px;
  color: var(--pos-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.temp-row :deep(.el-progress-bar__outer) {
  background: rgba(139, 161, 180, 0.16);
}
.temp-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  padding: 46px 0;
  color: var(--pos-muted);
}
.proc-title {
  display: flex;
  align-items: center;
  gap: 8px;
}
.proc-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}
.proc-search {
  width: 200px;
}
.proc-name {
  font-weight: 500;
}
.num {
  font-variant-numeric: tabular-nums;
}
.num.hot {
  color: var(--pos-warning);
}
.proc-panel :deep(.empty-state) {
  padding: 26px 12px;
}

@media (max-width: 1200px) {
  .cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .charts,
  .bottom-grid {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 820px) {
  .vitals {
    grid-template-columns: 1fr;
  }
  .proc-tools {
    width: 100%;
  }
  .proc-search {
    flex: 1;
    width: auto;
  }
}
@media (max-width: 640px) {
  .cards {
    grid-template-columns: 1fr;
  }
  .refresh-note {
    display: none;
  }
  .temp-list {
    max-height: none;
  }
}
</style>
