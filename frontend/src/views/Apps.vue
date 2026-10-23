<template>
  <div class="pos-page apps-page">
    <PageHeader :title="t('apps.title')" :subtitle="t('apps.subtitle')">
      <span class="filter-label">{{ t('apps.category') }}</span>
      <el-select v-model="category" class="category-select" :placeholder="t('apps.category')">
        <el-option v-for="option in categoryOptions" :key="option.value" :label="t(option.label)" :value="option.value" />
      </el-select>
      <el-button :icon="Refresh" :loading="loading" @click="load">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <div v-if="dockerHint" class="docker-banner" role="alert">
      <el-icon :size="18"><WarningFilled /></el-icon>
      <span>{{ t('apps.dockerRequired') }}</span>
      <div class="pos-spacer" />
      <el-button link size="small" @click="dockerHint = false">{{ t('common.close') }}</el-button>
    </div>

    <div class="filter-row">
      <el-radio-group v-model="stateFilter" size="small">
        <el-radio-button value="all">{{ t('common.all') }}</el-radio-button>
        <el-radio-button value="installed">{{ t('apps.installed') }} · {{ installedCount }}</el-radio-button>
        <el-radio-button value="available">{{ t('apps.available') }} · {{ availableCount }}</el-radio-button>
      </el-radio-group>
      <div class="pos-spacer" />
      <span class="pos-hint">{{ t('common.total') }} {{ filteredApps.length }} {{ t('common.items') }}</span>
    </div>

    <div v-loading="loading" class="apps-body">
      <template v-if="filteredApps.length">
        <section v-for="group in groups" :key="group.key" class="app-section">
          <div class="section-head">
            <span class="pos-card-title">{{ t(group.label) }}</span>
            <el-tag size="small" effect="plain" :type="group.key === 'installed' ? 'success' : 'info'">
              {{ group.apps.length }}
            </el-tag>
            <span class="section-line" />
          </div>

          <div class="app-grid">
            <article v-for="app in group.apps" :key="app.slug" class="pos-panel app-card" :class="{ 'is-installed': app.installed }">
              <div class="card-head">
                <div class="app-icon" :class="toneFor(app)">
                  <el-icon :size="24"><component :is="appIcon(app)" /></el-icon>
                </div>
                <div class="app-headings">
                  <div class="app-name" :title="app.name">{{ app.name }}</div>
                  <div class="app-tags">
                    <el-tag size="small" effect="plain" type="info">{{ categoryLabel(app.category) }}</el-tag>
                    <el-tag v-if="app.installed" size="small" effect="plain" type="success">{{ t('apps.installed') }}</el-tag>
                  </div>
                </div>
                <span v-if="app.installed" class="status-pill" :class="{ live: isRunning(app) }">
                  <i class="status-dot" />
                  {{ statusLabel(app) }}
                </span>
              </div>

              <p class="app-desc">{{ app.description }}</p>

              <div class="app-meta">
                <span v-if="app.image" class="image-tag pos-mono" :title="app.image">{{ app.image }}</span>
                <div v-if="app.ports && app.ports.length" class="port-row">
                  <el-tag v-for="port in app.ports" :key="port" size="small" effect="plain" type="info" class="port-tag">
                    {{ port }}
                  </el-tag>
                </div>
              </div>

              <div class="card-foot">
                <el-tag v-if="!app.installed" size="small" effect="plain" type="info">{{ t('apps.available') }}</el-tag>
                <div class="pos-spacer" />
                <div class="card-actions">
                  <template v-if="!app.installed">
                    <el-button
                      type="primary"
                      size="small"
                      :icon="Download"
                      :loading="busySlug === app.slug && busyAction === 'install'"
                      @click="install(app)"
                    >
                      {{ t('apps.install') }}
                    </el-button>
                  </template>
                  <template v-else>
                    <el-button
                      size="small"
                      :icon="isRunning(app) ? VideoPause : VideoPlay"
                      :loading="busySlug === app.slug && busyAction === 'toggle'"
                      @click="toggleRun(app)"
                    >
                      {{ isRunning(app) ? t('apps.stop') : t('apps.start') }}
                    </el-button>
                    <el-button size="small" :icon="Setting" @click="openManage(app)">{{ t('apps.manage') }}</el-button>
                    <el-button
                      size="small"
                      type="danger"
                      plain
                      :icon="Delete"
                      :loading="busySlug === app.slug && busyAction === 'uninstall'"
                      @click="uninstall(app)"
                    >
                      {{ t('apps.uninstall') }}
                    </el-button>
                  </template>
                </div>
              </div>
            </article>
          </div>
        </section>
      </template>

      <div v-else-if="!loading" class="pos-panel empty-panel">
        <EmptyState :title="t('apps.noApps')" :description="t('apps.subtitle')" icon="Grid">
          <el-button v-if="hasActiveFilters" :icon="Close" @click="resetFilters">{{ t('common.clear') }}</el-button>
          <el-button v-else :icon="Refresh" @click="load">{{ t('common.refresh') }}</el-button>
        </EmptyState>
      </div>
    </div>

    <el-drawer v-model="manageOpen" :title="t('apps.manage')" size="min(420px, 92vw)">
      <div v-if="manageApp" class="manage">
        <div class="manage-head">
          <div class="app-icon" :class="toneFor(manageApp)">
            <el-icon :size="24"><component :is="appIcon(manageApp)" /></el-icon>
          </div>
          <div class="manage-titles">
            <div class="manage-name">{{ manageApp.name }}</div>
            <el-tag size="small" effect="plain" type="info">{{ categoryLabel(manageApp.category) }}</el-tag>
          </div>
          <span class="status-pill" :class="{ live: isRunning(manageApp) }">
            <i class="status-dot" />
            {{ statusLabel(manageApp) }}
          </span>
        </div>

        <div v-loading="manageLoading" class="manage-body">
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('common.status')">
              <span class="status-inline">
                <i class="status-dot" :class="{ live: isRunning(manageApp) }" />
                {{ statusLabel(manageApp) }}
              </span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('apps.port')">
              <span v-if="managePorts.length" class="tag-row">
                <el-tag v-for="port in managePorts" :key="port" size="small" effect="plain" type="info">{{ port }}</el-tag>
              </span>
              <span v-else class="pos-hint">{{ t('common.none') }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('apps.dataDir')">
              <span class="pos-mono manage-path">{{ manageData?.compose_dir || '—' }}</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('apps.resources')">
              {{ t('apps.running') }} {{ containersRunning }} · {{ t('apps.stopped') }} {{ containersStopped }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('common.total')">
              {{ containersTotal }} {{ t('common.items') }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('apps.installedAt')">
              {{ formatDate(manageApp.installed_at) }}
            </el-descriptions-item>
          </el-descriptions>

          <div v-if="manageApp.image" class="manage-image">
            <span class="pos-hint">{{ manageApp.image }}</span>
          </div>
        </div>

        <div class="manage-actions">
          <el-button
            :type="isRunning(manageApp) ? 'warning' : 'primary'"
            :icon="isRunning(manageApp) ? VideoPause : VideoPlay"
            :loading="busySlug === manageApp.slug && busyAction === 'toggle'"
            @click="toggleRun(manageApp)"
          >
            {{ isRunning(manageApp) ? t('apps.stop') : t('apps.start') }}
          </el-button>
          <el-button
            type="danger"
            plain
            :icon="Delete"
            :loading="busySlug === manageApp.slug && busyAction === 'uninstall'"
            @click="uninstall(manageApp)"
          >
            {{ t('apps.uninstall') }}
          </el-button>
        </div>
      </div>
      <template #footer>
        <el-button @click="manageOpen = false">{{ t('common.close') }}</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh, Download, VideoCamera, Cloudy, Lock, Folder, Odometer, Grid,
  Setting, Delete, VideoPlay, VideoPause, WarningFilled, Close
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, errorMessage } from '../api'
import { formatDate } from '../utils/format'

const { t } = useI18n()

const apps = ref([])
const loading = ref(false)
const category = ref('all')
const stateFilter = ref('all')
const dockerHint = ref(false)
const busySlug = ref('')
const busyAction = ref('')

const manageOpen = ref(false)
const manageApp = ref(null)
const manageData = ref(null)
const manageLoading = ref(false)

const categoryOptions = [
  { value: 'all', label: 'common.all' },
  { value: 'download', label: 'apps.download' },
  { value: 'media', label: 'apps.media' },
  { value: 'tools', label: 'apps.tools' },
  { value: 'security', label: 'apps.security' }
]

const categoryKeys = {
  download: 'apps.download',
  media: 'apps.media',
  tools: 'apps.tools',
  security: 'apps.security'
}

const toneMap = {
  download: 'tone-blue',
  media: 'tone-violet',
  tools: 'tone-teal',
  security: 'tone-amber'
}

const iconMap = {
  download: Download,
  film: VideoCamera,
  cloud: Cloudy,
  shield: Lock,
  folder: Folder,
  sync: Refresh,
  activity: Odometer
}

const scopedApps = computed(() =>
  apps.value.filter((app) => category.value === 'all' || app.category === category.value)
)
const installedCount = computed(() => scopedApps.value.filter((app) => app.installed).length)
const availableCount = computed(() => scopedApps.value.filter((app) => !app.installed).length)

const filteredApps = computed(() =>
  scopedApps.value.filter((app) => {
    if (stateFilter.value === 'installed' && !app.installed) return false
    if (stateFilter.value === 'available' && app.installed) return false
    return true
  })
)

const installedApps = computed(() => filteredApps.value.filter((app) => app.installed))
const availableApps = computed(() => filteredApps.value.filter((app) => !app.installed))

const groups = computed(() => {
  const list = []
  if (stateFilter.value !== 'available' && installedApps.value.length) {
    list.push({ key: 'installed', label: 'apps.installed', apps: installedApps.value })
  }
  if (stateFilter.value !== 'installed' && availableApps.value.length) {
    list.push({ key: 'available', label: 'apps.available', apps: availableApps.value })
  }
  return list
})

const hasActiveFilters = computed(() => category.value !== 'all' || stateFilter.value !== 'all')

const managePorts = computed(() => manageData.value?.ports || manageApp.value?.ports || [])
const containersTotal = computed(() => Number(manageData.value?.containers || 0))
const containersRunning = computed(() => Number(manageData.value?.running || 0))
const containersStopped = computed(() => Math.max(0, containersTotal.value - containersRunning.value))

function categoryLabel(value) {
  return categoryKeys[value] ? t(categoryKeys[value]) : t('common.unknown')
}

function appIcon(app) {
  return iconMap[app?.icon] || Grid
}

function toneFor(app) {
  return toneMap[app?.category] || 'tone-default'
}

function appStatus(app) {
  if (!app) return 'stopped'
  if (manageOpen.value && manageData.value && manageApp.value?.slug === app.slug) {
    return manageData.value.status || app.status
  }
  return app.status
}

function isRunning(app) {
  return appStatus(app) === 'running'
}

function statusLabel(app) {
  return isRunning(app) ? t('apps.statusRunning') : t('apps.statusStopped')
}

function isDockerFailure(error) {
  const status = error?.response?.status
  const code = error?.response?.data?.error?.code
  return status === 502 || code === 'docker_unavailable' || code === 'install_failed' || code === 'compose_error'
}

async function load() {
  loading.value = true
  try {
    apps.value = (await api.apps.list()) || []
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  category.value = 'all'
  stateFilter.value = 'all'
}

async function install(app) {
  try {
    await ElMessageBox.confirm(
      t('apps.installConfirm', { name: app.name }),
      t('apps.install'),
      {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel')
      }
    )
  } catch (error) {
    return
  }
  busySlug.value = app.slug
  busyAction.value = 'install'
  try {
    await api.apps.install(app.slug)
    dockerHint.value = false
    ElMessage.success(t('apps.installSuccess'))
    await load()
  } catch (error) {
    ElMessage.error(errorMessage(error))
    if (isDockerFailure(error)) {
      dockerHint.value = true
      ElMessage.warning(t('apps.dockerRequired'))
    }
  } finally {
    busySlug.value = ''
    busyAction.value = ''
  }
}

async function uninstall(app) {
  try {
    await ElMessageBox.confirm(
      t('apps.uninstallConfirm', { name: app.name }),
      t('apps.uninstall'),
      {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel')
      }
    )
  } catch (error) {
    return
  }
  busySlug.value = app.slug
  busyAction.value = 'uninstall'
  try {
    await api.apps.uninstall(app.slug)
    ElMessage.success(t('apps.uninstallSuccess'))
    if (manageOpen.value && manageApp.value?.slug === app.slug) manageOpen.value = false
    await load()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busySlug.value = ''
    busyAction.value = ''
  }
}

async function toggleRun(app) {
  const running = isRunning(app)
  busySlug.value = app.slug
  busyAction.value = 'toggle'
  try {
    if (running) await api.apps.stop(app.slug)
    else await api.apps.start(app.slug)
    ElMessage.success(t('common.operationSuccess'))
    await load()
    if (manageOpen.value && manageApp.value?.slug === app.slug) {
      manageApp.value = apps.value.find((item) => item.slug === app.slug) || manageApp.value
      await loadStatus(app.slug)
    }
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busySlug.value = ''
    busyAction.value = ''
  }
}

async function openManage(app) {
  manageApp.value = app
  manageData.value = null
  manageOpen.value = true
  await loadStatus(app.slug)
}

async function loadStatus(slug) {
  manageLoading.value = true
  try {
    const data = await api.apps.status(slug)
    if (manageApp.value?.slug === slug) manageData.value = data
  } catch (error) {
    if (manageApp.value?.slug === slug) ElMessage.error(errorMessage(error))
  } finally {
    if (manageApp.value?.slug === slug) manageLoading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.apps-page {
  gap: 16px;
}
.filter-label {
  font-size: 12px;
  color: var(--pos-muted);
}
.category-select {
  width: 158px;
}
.docker-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border-radius: 12px;
  border: 1px solid rgba(251, 191, 36, 0.28);
  background: rgba(251, 191, 36, 0.08);
  color: var(--pos-warning);
  font-size: 12.5px;
}
.filter-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.filter-row :deep(.el-radio-button__inner) {
  background: var(--pos-panel-2);
  border-color: var(--pos-border);
  color: var(--pos-muted);
}
.filter-row :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  color: var(--pos-accent);
  background: var(--pos-accent-soft);
  border-color: rgba(45, 212, 191, 0.35);
  box-shadow: -1px 0 0 0 rgba(45, 212, 191, 0.35);
}
.apps-body {
  min-height: 220px;
}
.app-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 20px;
}
.app-section:last-child {
  margin-bottom: 0;
}
.section-head {
  display: flex;
  align-items: center;
  gap: 10px;
}
.section-line {
  flex: 1;
  height: 1px;
  background: var(--pos-border);
}
.app-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 14px;
}
.app-card {
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  border-radius: var(--pos-radius);
  box-shadow: var(--pos-shadow);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: border-color 0.16s ease, transform 0.16s ease;
}
.app-card:hover {
  border-color: rgba(45, 212, 191, 0.35);
  transform: translateY(-2px);
}
.app-card.is-installed {
  background:
    linear-gradient(180deg, rgba(45, 212, 191, 0.05), transparent 42%),
    var(--pos-panel);
  border-color: rgba(45, 212, 191, 0.22);
}
.card-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}
.app-icon {
  width: 46px;
  height: 46px;
  border-radius: 13px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.tone-teal {
  color: #2dd4bf;
  background: rgba(45, 212, 191, 0.13);
}
.tone-blue {
  color: #60a5fa;
  background: rgba(96, 165, 250, 0.13);
}
.tone-violet {
  color: #c084fc;
  background: rgba(192, 132, 252, 0.13);
}
.tone-amber {
  color: #fbbf24;
  background: rgba(251, 191, 36, 0.13);
}
.tone-default {
  color: var(--pos-muted);
  background: var(--pos-panel-2);
}
.app-headings {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.app-name {
  font-size: 14.5px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.app-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 9px;
  border-radius: 999px;
  border: 1px solid var(--pos-border);
  background: var(--pos-panel-2);
  color: var(--pos-muted);
  font-size: 10.5px;
  white-space: nowrap;
  flex-shrink: 0;
}
.status-pill.live {
  color: var(--pos-success);
  border-color: rgba(52, 211, 153, 0.28);
  background: rgba(52, 211, 153, 0.08);
}
.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--pos-muted);
  display: inline-block;
  flex-shrink: 0;
}
.status-pill.live .status-dot,
.status-dot.live {
  background: var(--pos-success);
  box-shadow: 0 0 8px var(--pos-success);
}
.app-desc {
  margin: 0;
  color: var(--pos-muted);
  font-size: 12.5px;
  line-height: 1.65;
  min-height: 41px;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.app-meta {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.image-tag {
  display: block;
  max-width: 100%;
  padding: 4px 8px;
  border-radius: 7px;
  border: 1px solid var(--pos-border);
  background: var(--pos-panel-2);
  color: var(--pos-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.port-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.port-tag {
  margin: 0;
}
.card-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: auto;
  padding-top: 11px;
  border-top: 1px solid var(--pos-border);
}
.card-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.card-actions :deep(.el-button) {
  margin-left: 0;
}
.empty-panel {
  padding: 8px;
}
.manage {
  display: flex;
  flex-direction: column;
  gap: 18px;
}
.manage-head {
  display: flex;
  align-items: center;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--pos-border);
}
.manage-titles {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
}
.manage-name {
  font-size: 15px;
  font-weight: 600;
  word-break: break-word;
}
.manage-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 140px;
}
.manage-body :deep(.el-descriptions__label) {
  color: var(--pos-muted);
  white-space: nowrap;
}
.manage-path {
  word-break: break-all;
}
.status-inline {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.tag-row {
  display: inline-flex;
  flex-wrap: wrap;
  gap: 6px;
}
.manage-image {
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--pos-panel-2);
  border: 1px dashed var(--pos-border);
  word-break: break-all;
}
.manage-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
.manage-actions :deep(.el-button) {
  margin-left: 0;
}

@media (max-width: 720px) {
  .app-grid {
    grid-template-columns: 1fr;
  }
  .category-select {
    width: 132px;
  }
  .filter-row {
    gap: 8px;
  }
  .status-pill {
    order: 3;
  }
}
</style>
