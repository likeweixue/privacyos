<template>
  <div class="pos-page docker-page">
    <PageHeader :title="t('docker.title')" :subtitle="t('docker.subtitle')">
      <el-tag v-if="dockerStatus.available && dockerStatus.version" size="small" effect="plain" class="version-chip">
        <el-icon><Box /></el-icon>
        <span>{{ t('docker.title') }} {{ dockerStatus.version }}</span>
        <span v-if="dockerStatus.api_version" class="api-version">API {{ dockerStatus.api_version }}</span>
      </el-tag>
      <el-button :icon="Refresh" :loading="refreshing" @click="refreshCurrent">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <el-alert
      v-if="auth.dev"
      :title="t('docker.devMock')"
      type="info"
      :closable="false"
      show-icon
    />

    <el-alert
      v-if="daemonDown"
      class="daemon-alert"
      type="error"
      :closable="false"
      show-icon
      :title="t('docker.daemonUnavailable')"
      :description="dockerStatus.error || dockerStatus.socket || ''"
    />

    <section v-if="daemonDown" class="pos-panel daemon-panel">
      <EmptyState :title="t('docker.daemonUnavailable')" :description="t('docker.emptyHint')" icon="Box">
        <el-button type="primary" :icon="Refresh" :loading="statusLoading" @click="retryStatus">
          {{ t('common.retry') }}
        </el-button>
      </EmptyState>
    </section>

    <section v-else-if="!statusChecked" v-loading="true" class="pos-panel boot-panel" />

    <section v-else class="pos-panel docker-panel">
      <el-tabs v-model="activeTab" class="docker-tabs">
        <el-tab-pane name="containers">
          <template #label>
            <span class="tab-label">
              {{ t('docker.containers') }}
              <span v-if="containers.length" class="tab-count">{{ containers.length }}</span>
            </span>
          </template>

          <div class="pane-toolbar">
            <span class="pos-hint">{{ t('common.total') }} {{ containers.length }}</span>
            <div class="pos-spacer" />
            <el-button :icon="Refresh" size="small" :loading="containersLoading" @click="loadContainers()">
              {{ t('common.refresh') }}
            </el-button>
          </div>

          <el-table :data="containers" v-loading="containersLoading" row-key="id" class="docker-table">
            <el-table-column :label="t('docker.name')" min-width="200">
              <template #default="{ row }">
                <div class="name-cell">
                  <span class="row-icon"><el-icon><Box /></el-icon></span>
                  <div class="name-text">
                    <span class="primary">{{ row.name || '—' }}</span>
                    <span class="pos-mono secondary">{{ shortID(row.id) }}</span>
                  </div>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.image')" min-width="170" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="ellipsis" :title="row.image">{{ row.image }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.state')" min-width="150">
              <template #default="{ row }">
                <div class="state-cell">
                  <el-tag size="small" effect="plain" :type="stateType(row.state)">{{ stateLabel(row.state) }}</el-tag>
                  <span v-if="row.status" class="state-text">{{ row.status }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.ports')" min-width="150">
              <template #default="{ row }">
                <div v-if="row.ports && row.ports.length" class="port-list">
                  <el-tag v-for="(port, idx) in row.ports" :key="idx" size="small" effect="plain" class="port-tag">
                    {{ portText(port) }}
                  </el-tag>
                </div>
                <span v-else class="pos-hint">{{ t('docker.portsEmpty') }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.cpu')" width="78" align="right">
              <template #default="{ row }">
                <span class="num">{{ row.state === 'running' ? formatPercent(row.cpu || 0) : '—' }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.memory')" width="100" align="right">
              <template #default="{ row }">
                <span class="num">{{ row.state === 'running' && row.memory ? formatBytes(row.memory) : '—' }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('common.actions')" width="156" align="right" fixed="right">
              <template #default="{ row }">
                <div class="row-actions icon-actions">
                  <el-tooltip :content="row.state === 'running' ? t('docker.stop') : t('docker.start')" placement="top">
                    <el-button
                      circle
                      size="small"
                      :type="row.state === 'running' ? 'warning' : 'success'"
                      plain
                      :icon="row.state === 'running' ? VideoPause : VideoPlay"
                      :loading="actionBusy[row.id] === 'start' || actionBusy[row.id] === 'stop'"
                      :disabled="!!actionBusy[row.id]"
                      @click="containerAction(row, row.state === 'running' ? 'stop' : 'start')"
                    />
                  </el-tooltip>
                  <el-tooltip :content="t('docker.restart')" placement="top">
                    <el-button
                      circle
                      size="small"
                      plain
                      :icon="RefreshRight"
                      :loading="actionBusy[row.id] === 'restart'"
                      :disabled="!!actionBusy[row.id] || row.state !== 'running'"
                      @click="containerAction(row, 'restart')"
                    />
                  </el-tooltip>
                  <el-tooltip :content="t('docker.logs')" placement="top">
                    <el-button circle size="small" plain :icon="Document" @click="openContainerLogs(row)" />
                  </el-tooltip>
                  <el-dropdown trigger="click" @command="(cmd) => containerMenu(cmd, row)">
                    <el-button circle size="small" plain :icon="MoreFilled" />
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="stats"><el-icon><DataAnalysis /></el-icon>{{ t('docker.stats') }}</el-dropdown-item>
                        <el-dropdown-item command="remove" divided><el-icon><Delete /></el-icon>{{ t('docker.remove') }}</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>

            <template #empty>
              <EmptyState :title="t('docker.empty')" :description="t('docker.emptyHint')" icon="Box">
                <el-button :icon="Refresh" @click="loadContainers()">{{ t('common.refresh') }}</el-button>
              </EmptyState>
            </template>
          </el-table>
        </el-tab-pane>

        <el-tab-pane name="images">
          <template #label>
            <span class="tab-label">
              {{ t('docker.images') }}
              <span v-if="images.length" class="tab-count">{{ images.length }}</span>
            </span>
          </template>

          <div class="pane-toolbar">
            <span class="pos-hint">{{ t('common.total') }} {{ images.length }}</span>
            <div class="pos-spacer" />
            <el-button type="primary" :icon="Download" @click="openPullDialog">{{ t('docker.pull') }}</el-button>
            <el-button :icon="Refresh" :loading="imagesLoading" @click="loadImages">{{ t('common.refresh') }}</el-button>
          </div>

          <el-table :data="images" v-loading="imagesLoading" row-key="id" class="docker-table">
            <el-table-column :label="t('docker.image')" min-width="280">
              <template #default="{ row }">
                <div v-if="row.tags && row.tags.length" class="tag-list">
                  <el-tag v-for="tag in row.tags" :key="tag" size="small" effect="plain">{{ tag }}</el-tag>
                </div>
                <span v-else class="pos-mono">{{ shortID(row.id, 24) }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('files.size')" width="130" align="right">
              <template #default="{ row }">
                <span class="num">{{ formatBytes(row.size) }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.created')" width="180">
              <template #default="{ row }">{{ formatDate(row.created * 1000) }}</template>
            </el-table-column>

            <el-table-column :label="t('common.actions')" width="110" align="right">
              <template #default="{ row }">
                <el-button
                  link
                  type="danger"
                  size="small"
                  :loading="!!imageBusy[row.id]"
                  @click="removeImage(row)"
                >
                  {{ t('common.delete') }}
                </el-button>
              </template>
            </el-table-column>

            <template #empty>
              <EmptyState :title="t('docker.empty')" icon="Box">
                <el-button type="primary" :icon="Download" @click="openPullDialog">{{ t('docker.pull') }}</el-button>
              </EmptyState>
            </template>
          </el-table>
        </el-tab-pane>

        <el-tab-pane name="compose">
          <template #label>
            <span class="tab-label">
              {{ t('docker.compose') }}
              <span v-if="composeProjects.length" class="tab-count">{{ composeProjects.length }}</span>
            </span>
          </template>

          <div class="pane-toolbar">
            <span class="pos-hint">{{ t('common.total') }} {{ composeProjects.length }}</span>
            <div class="pos-spacer" />
            <el-button type="primary" :icon="Plus" @click="openComposeDialog">{{ t('docker.newCompose') }}</el-button>
            <el-button :icon="Refresh" :loading="composeLoading" @click="loadCompose">{{ t('common.refresh') }}</el-button>
          </div>

          <el-table :data="composeProjects" v-loading="composeLoading" row-key="name" class="docker-table">
            <el-table-column :label="t('docker.project')" min-width="160">
              <template #default="{ row }">
                <div class="name-cell">
                  <span class="row-icon"><el-icon><Files /></el-icon></span>
                  <span class="primary">{{ row.name }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.services')" min-width="180">
              <template #default="{ row }">
                <div v-if="row.services && row.services.length" class="tag-list">
                  <el-tag v-for="service in row.services" :key="service" size="small" effect="plain">
                    {{ service }}
                  </el-tag>
                </div>
                <span v-else class="pos-hint">{{ t('common.none') }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('docker.updated')" width="156">
              <template #default="{ row }">{{ formatDate(row.updated) }}</template>
            </el-table-column>

            <el-table-column :label="t('files.path')" min-width="220">
              <template #default="{ row }">
                <span class="file-path pos-mono" :title="row.file">{{ row.file }}</span>
              </template>
            </el-table-column>

            <el-table-column :label="t('common.actions')" width="272" align="right" fixed="right">
              <template #default="{ row }">
                <div class="row-actions">
                  <el-button
                    link
                    type="primary"
                    size="small"
                    :loading="composeBusy[row.name] === 'up'"
                    :disabled="!!composeBusy[row.name]"
                    @click="composeAction(row, 'up')"
                  >
                    {{ t('docker.deploy') }}
                  </el-button>
                  <el-button
                    link
                    size="small"
                    :loading="composeBusy[row.name] === 'down'"
                    :disabled="!!composeBusy[row.name]"
                    @click="composeAction(row, 'down')"
                  >
                    {{ t('docker.down') }}
                  </el-button>
                  <el-button link type="primary" size="small" @click="openComposeLogs(row)">
                    {{ t('docker.logs') }}
                  </el-button>
                  <el-button
                    link
                    type="danger"
                    size="small"
                    :loading="composeBusy[row.name] === 'delete'"
                    :disabled="!!composeBusy[row.name]"
                    @click="removeCompose(row)"
                  >
                    {{ t('common.delete') }}
                  </el-button>
                </div>
              </template>
            </el-table-column>

            <template #empty>
              <EmptyState :title="t('docker.noCompose')" :description="t('docker.emptyHint')" icon="Files">
                <el-button type="primary" :icon="Plus" @click="openComposeDialog">{{ t('docker.newCompose') }}</el-button>
              </EmptyState>
            </template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </section>

    <!-- container stats -->
    <el-dialog v-model="statsOpen" :title="statsTitle" width="min(540px, 94vw)">
      <div v-loading="statsLoading" class="stats-body">
        <template v-if="containerStats">
          <div class="stats-grid">
            <div class="stat-tile">
              <span class="stat-label">{{ t('docker.cpu') }}</span>
              <strong class="stat-value">{{ formatPercent(containerStats.cpu || 0) }}</strong>
            </div>
            <div class="stat-tile">
              <span class="stat-label">{{ t('docker.memory') }}</span>
              <strong class="stat-value">{{ formatBytes(containerStats.mem_usage || 0) }}</strong>
              <span class="pos-hint">{{ formatBytes(containerStats.mem_limit || 0) }}</span>
            </div>
          </div>

          <UsageBar
            :label="t('docker.memory')"
            :used="containerStats.mem_usage || 0"
            :quota="containerStats.mem_limit || 0"
          />

          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('dashboard.network')">
              <span class="stat-inline">
                <span>{{ t('dashboard.down') }} {{ formatBytes(containerStats.net_rx || 0) }}</span>
                <span>{{ t('dashboard.up') }} {{ formatBytes(containerStats.net_tx || 0) }}</span>
              </span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('monitor.diskIO')">
              <span class="stat-inline">
                <span>{{ t('monitor.read') }} {{ formatBytes(containerStats.block_read || 0) }}</span>
                <span>{{ t('monitor.write') }} {{ formatBytes(containerStats.block_write || 0) }}</span>
              </span>
            </el-descriptions-item>
          </el-descriptions>
        </template>
        <div v-else class="pos-hint stats-empty">{{ t('common.empty') }}</div>
      </div>
      <template #footer>
        <el-button @click="statsOpen = false">{{ t('common.close') }}</el-button>
        <el-button type="primary" :icon="Refresh" :loading="statsLoading" @click="reloadStats">
          {{ t('common.refresh') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- logs drawer -->
    <el-drawer v-model="logState.open" :title="logTitle" size="min(820px, 94vw)">
      <div class="log-toolbar">
        <span class="pos-hint">{{ logState.name }}</span>
        <div class="pos-spacer" />
        <el-button size="small" :icon="Refresh" :loading="logState.loading" @click="fetchLogs">
          {{ t('common.refresh') }}
        </el-button>
      </div>
      <pre v-loading="logState.loading" class="log-pre">{{ logState.text || t('common.empty') }}</pre>
    </el-drawer>

    <!-- pull image -->
    <el-dialog v-model="pullOpen" :title="t('docker.pull')" width="min(480px, 94vw)" @closed="resetPull">
      <el-form label-position="top" @submit.prevent="submitPull">
        <el-form-item :label="t('docker.imageName')">
          <el-input
            v-model="pullImage"
            :placeholder="t('docker.pullHint')"
            :disabled="pullLoading || pullStarted"
            @keyup.enter="submitPull"
          />
        </el-form-item>
      </el-form>
      <template v-if="pullStarted">
        <el-alert
          type="success"
          :closable="false"
          show-icon
          :title="t('docker.pullStarted')"
          :description="t('common.loading')"
        />
        <div v-if="pullJob" class="pull-job">
          <span class="pos-hint">{{ t('common.details') }}</span>
          <span class="pos-mono">{{ pullJob.id }}</span>
          <el-tag v-if="pullJob.status === 'running'" size="small" effect="plain" type="success">
            {{ t('common.running') }}
          </el-tag>
        </div>
      </template>
      <template #footer>
        <el-button @click="pullOpen = false">{{ pullStarted ? t('common.close') : t('common.cancel') }}</el-button>
        <el-button
          v-if="!pullStarted"
          type="primary"
          :icon="Download"
          :loading="pullLoading"
          @click="submitPull"
        >
          {{ t('docker.pull') }}
        </el-button>
        <el-button v-else :icon="Refresh" :loading="imagesLoading" @click="loadImages">
          {{ t('common.refresh') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- new compose project -->
    <el-dialog v-model="composeDialog" :title="t('docker.newCompose')" width="min(680px, 94vw)">
      <el-form label-position="top" @submit.prevent="submitCompose">
        <el-form-item :label="t('docker.composeName')">
          <el-input v-model="composeForm.name" maxlength="60" :placeholder="t('docker.composeName')" />
        </el-form-item>
        <el-form-item :label="t('docker.composeContent')">
          <el-input
            v-model="composeForm.content"
            type="textarea"
            :rows="12"
            class="yaml-editor"
            :placeholder="composeSample"
            spellcheck="false"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="composeDialog = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="composeSaving" @click="submitCompose">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Box, Files, Download, Plus, VideoPlay, VideoPause, RefreshRight, Document, DataAnalysis, Delete, MoreFilled } from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import UsageBar from '../components/UsageBar.vue'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { formatBytes, formatDate, formatPercent, shortID } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()

const POLL_INTERVAL = 8000
const composeSample = `services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
    restart: unless-stopped`

const dockerStatus = ref({})
const statusChecked = ref(false)
const statusLoading = ref(false)
const refreshing = ref(false)
const activeTab = ref('containers')

const containers = ref([])
const containersLoading = ref(false)
const images = ref([])
const imagesLoading = ref(false)
const imagesLoaded = ref(false)
const composeProjects = ref([])
const composeLoading = ref(false)
const composeLoaded = ref(false)

const actionBusy = reactive({})
const imageBusy = reactive({})
const composeBusy = reactive({})

const statsOpen = ref(false)
const statsLoading = ref(false)
const statsTarget = ref(null)
const containerStats = ref(null)

const logState = reactive({ open: false, loading: false, text: '', ref: '', name: '', kind: 'container' })

const pullOpen = ref(false)
const pullImage = ref('')
const pullLoading = ref(false)
const pullStarted = ref(false)
const pullJob = ref(null)

const composeDialog = ref(false)
const composeSaving = ref(false)
const composeForm = reactive({ name: '', content: '' })

let pollTimer = null
let containersInFlight = false
let containersQueued = false

const daemonDown = computed(() => statusChecked.value && !dockerStatus.value.available && !auth.dev)
const statsTitle = computed(() => `${t('docker.statsTitle')} · ${statsTarget.value?.name || ''}`)
const logTitle = computed(() => {
  const base = logState.kind === 'compose' ? t('docker.compose') : t('docker.logTitle')
  return `${base} · ${logState.name}`
})

function stateType(state) {
  if (state === 'running' || state === 'restarting') return 'success'
  if (state === 'paused') return 'warning'
  if (state === 'dead') return 'danger'
  return 'info'
}

function stateLabel(state) {
  if (state === 'running' || state === 'restarting') return t('docker.running')
  if (state === 'exited') return t('docker.exited')
  if (state === 'created' || state === 'paused') return t('common.stopped')
  if (state === 'dead') return t('common.error')
  return state || t('common.unknown')
}

function portText(port) {
  if (!port) return ''
  const type = port.type || 'tcp'
  return port.public ? `${port.public}→${port.private}/${type}` : `${port.private}/${type}`
}

async function loadStatus(manual = false) {
  statusLoading.value = true
  try {
    dockerStatus.value = (await api.docker.status()) || {}
  } catch (error) {
    dockerStatus.value = { available: false, error: errorMessage(error) }
    if (manual) ElMessage.error(errorMessage(error))
  } finally {
    statusChecked.value = true
    statusLoading.value = false
    if (!daemonDown.value) {
      if (!containers.value.length) loadContainers()
      startPolling()
    } else {
      stopPolling()
    }
  }
}

function retryStatus() {
  loadStatus(true)
}

async function loadContainers(silent = false) {
  if (containersInFlight) {
    if (!silent) containersQueued = true
    return
  }
  containersInFlight = true
  if (!silent) containersLoading.value = true
  try {
    containers.value = (await api.docker.containers({ stats: 1 })) || []
  } catch (error) {
    if (!silent) ElMessage.error(errorMessage(error))
  } finally {
    containersInFlight = false
    containersLoading.value = false
    if (containersQueued) {
      containersQueued = false
      loadContainers(true)
    }
  }
}

async function loadImages() {
  imagesLoading.value = true
  try {
    images.value = (await api.docker.images()) || []
    imagesLoaded.value = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    imagesLoading.value = false
  }
}

async function loadCompose() {
  composeLoading.value = true
  try {
    composeProjects.value = (await api.docker.compose()) || []
    composeLoaded.value = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    composeLoading.value = false
  }
}

async function loadActive() {
  if (activeTab.value === 'containers') await loadContainers()
  else if (activeTab.value === 'images') await loadImages()
  else await loadCompose()
}

async function refreshCurrent() {
  refreshing.value = true
  try {
    if (!statusChecked.value || daemonDown.value) {
      await loadStatus(true)
      return
    }
    await loadActive()
  } finally {
    refreshing.value = false
  }
}

function startPolling() {
  stopPolling()
  if (!statusChecked.value || daemonDown.value) return
  pollTimer = setInterval(() => {
    if (activeTab.value !== 'containers') return
    if (document.visibilityState === 'hidden') return
    loadContainers(true)
  }, POLL_INTERVAL)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

function onVisibilityChange() {
  if (document.visibilityState !== 'visible') return
  if (!statusChecked.value || daemonDown.value || activeTab.value !== 'containers') return
  loadContainers(true)
}

watch(activeTab, (tab) => {
  if (tab === 'containers') {
    if (statusChecked.value && !daemonDown.value) loadContainers(true)
    startPolling()
    return
  }
  stopPolling()
  if (tab === 'images' && !imagesLoaded.value) loadImages()
  if (tab === 'compose' && !composeLoaded.value) loadCompose()
})

async function containerAction(row, action) {
  if (actionBusy[row.id]) return
  actionBusy[row.id] = action
  try {
    await api.docker.action(row.id, action)
    ElMessage.success(t('common.operationSuccess'))
    await loadContainers(true)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    delete actionBusy[row.id]
  }
}

async function containerMenu(command, row) {
  if (command === 'stats') openStats(row)
  else if (command === 'remove') removeContainer(row)
}

async function removeContainer(row) {
  try {
    await ElMessageBox.confirm(t('docker.removeConfirm'), t('docker.remove'), {
      type: 'warning',
      confirmButtonText: t('common.delete'),
      cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  if (actionBusy[row.id]) return
  actionBusy[row.id] = 'remove'
  try {
    await api.docker.remove(row.id, true)
    ElMessage.success(t('common.deleteSuccess'))
    await loadContainers(true)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    delete actionBusy[row.id]
  }
}

function openStats(row) {
  statsTarget.value = row
  containerStats.value = null
  statsOpen.value = true
  loadStats()
}

async function loadStats() {
  const row = statsTarget.value
  if (!row) return
  statsLoading.value = true
  try {
    containerStats.value = await api.docker.stats(row.id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
    statsOpen.value = false
  } finally {
    statsLoading.value = false
  }
}

function reloadStats() {
  loadStats()
}

function openContainerLogs(row) {
  logState.kind = 'container'
  logState.ref = row.id
  logState.name = row.name || shortID(row.id)
  logState.text = ''
  logState.open = true
  fetchLogs()
}

function openComposeLogs(project) {
  logState.kind = 'compose'
  logState.ref = project.name
  logState.name = project.name
  logState.text = ''
  logState.open = true
  fetchLogs()
}

async function fetchLogs() {
  if (!logState.ref) return
  logState.loading = true
  try {
    const payload = logState.kind === 'compose'
      ? await api.docker.composeLogs(logState.ref, 200)
      : await api.docker.logs(logState.ref, 300)
    logState.text = payload?.logs || ''
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    logState.loading = false
  }
}

function openPullDialog() {
  pullImage.value = ''
  pullStarted.value = false
  pullJob.value = null
  pullOpen.value = true
}

function resetPull() {
  pullImage.value = ''
  pullLoading.value = false
  pullStarted.value = false
  pullJob.value = null
}

async function submitPull() {
  if (pullStarted.value) return
  const image = pullImage.value.trim()
  if (!image) {
    ElMessage.warning(t('common.required'))
    return
  }
  pullLoading.value = true
  try {
    pullJob.value = (await api.docker.pull(image)) || null
    pullStarted.value = true
    ElMessage.success(t('docker.pullStarted'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    pullLoading.value = false
  }
}

async function removeImage(row) {
  const label = row.tags && row.tags.length ? row.tags[0] : shortID(row.id, 18)
  try {
    await ElMessageBox.confirm(`${t('common.delete')} ${label}`, t('docker.images'), {
      type: 'warning',
      confirmButtonText: t('common.delete'),
      cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  if (imageBusy[row.id]) return
  imageBusy[row.id] = true
  try {
    await api.docker.removeImage(row.id)
    ElMessage.success(t('common.deleteSuccess'))
    await loadImages()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    imageBusy[row.id] = false
  }
}

function openComposeDialog() {
  composeForm.name = ''
  composeForm.content = ''
  composeDialog.value = true
}

async function submitCompose() {
  const name = composeForm.name.trim()
  if (!name || !composeForm.content.trim()) {
    ElMessage.warning(t('common.required'))
    return
  }
  composeSaving.value = true
  try {
    await api.docker.createCompose({ name, content: composeForm.content })
    ElMessage.success(t('docker.composeCreated'))
    composeDialog.value = false
    await loadCompose()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    composeSaving.value = false
  }
}

async function composeAction(project, action) {
  const name = project.name
  if (composeBusy[name]) return
  composeBusy[name] = action
  try {
    if (action === 'up') {
      await api.docker.composeUp(name)
      ElMessage.success(t('docker.deploySuccess'))
    } else {
      await api.docker.composeDown(name)
      ElMessage.success(t('docker.downSuccess'))
    }
    await loadCompose()
    await loadContainers(true)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    delete composeBusy[name]
  }
}

async function removeCompose(project) {
  try {
    await ElMessageBox.confirm(t('docker.deleteProjectConfirm'), t('common.delete'), {
      type: 'warning',
      confirmButtonText: t('common.delete'),
      cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  const name = project.name
  if (composeBusy[name]) return
  composeBusy[name] = 'delete'
  try {
    await api.docker.deleteCompose(name)
    ElMessage.success(t('common.deleteSuccess'))
    await loadCompose()
    await loadContainers(true)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    delete composeBusy[name]
  }
}

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  loadStatus()
})

onBeforeUnmount(() => {
  stopPolling()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<style scoped>
.docker-page {
  min-height: 100%;
}
.version-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}
.version-chip .api-version {
  color: var(--pos-muted);
  font-size: 11px;
  padding-left: 7px;
  border-left: 1px solid var(--pos-border);
}
.daemon-alert {
  border-radius: 12px;
}
.daemon-panel {
  padding: 12px;
}
.boot-panel {
  min-height: 260px;
}
.docker-panel {
  padding: 8px 18px 18px;
}
.docker-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
}
.docker-tabs :deep(.el-tabs__nav-wrap::after) {
  background-color: var(--pos-border);
}
.tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.tab-count {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 6px;
  background: var(--pos-panel-2);
  border: 1px solid var(--pos-border);
  color: var(--pos-muted);
  font-size: 10px;
  line-height: 16px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.pane-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 2px 0 12px;
}
.pane-toolbar :deep(.el-button + .el-button) {
  margin-left: 0;
}
.docker-table {
  width: 100%;
}
.name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.row-icon {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  flex: none;
}
.name-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.primary {
  font-size: 13px;
  font-weight: 500;
  color: var(--pos-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.secondary {
  color: var(--pos-muted);
  font-size: 10.5px;
}
.ellipsis {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12.5px;
}
.state-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  min-width: 0;
}
.state-text {
  font-size: 11px;
  color: var(--pos-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}
.port-list,
.tag-list {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
.port-tag {
  font-family: 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
  font-size: 10.5px;
}
.num {
  font-variant-numeric: tabular-nums;
  font-size: 12.5px;
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 2px 8px;
}
.icon-actions {
  flex-wrap: nowrap;
  align-items: center;
  gap: 4px;
}
.icon-actions :deep(.el-button) {
  margin-left: 0;
}
.row-actions :deep(.el-button) {
  margin-left: 0;
  padding: 2px 0;
  height: auto;
  font-size: 12px;
}
.file-path {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--pos-muted);
}
.stats-body {
  min-height: 180px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.stats-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 160px;
}
.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.stat-tile {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px;
  border-radius: 12px;
  background: var(--pos-panel-2);
  border: 1px solid var(--pos-border);
}
.stat-label {
  font-size: 11px;
  color: var(--pos-muted);
}
.stat-value {
  font-size: 22px;
  font-variant-numeric: tabular-nums;
  letter-spacing: 0.2px;
}
.stat-inline {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 18px;
  font-variant-numeric: tabular-nums;
}
.log-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 10px;
}
.log-pre {
  margin: 0;
  min-height: 240px;
  height: calc(100vh - 210px);
  overflow: auto;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--pos-border);
  background: var(--pos-bg);
  color: var(--pos-text);
  font-family: 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
  tab-size: 2;
}
.pull-job {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 12px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--pos-panel-2);
  border: 1px solid var(--pos-border);
}
.yaml-editor :deep(.el-textarea__inner) {
  font-family: 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.65;
  tab-size: 2;
}

@media (max-width: 900px) {
  .docker-panel {
    padding: 6px 12px 14px;
  }
  .version-chip .api-version {
    display: none;
  }
  .stats-grid {
    grid-template-columns: 1fr;
  }
  .log-pre {
    height: calc(100vh - 230px);
  }
}

@media (max-width: 640px) {
  .docker-panel {
    padding: 4px 8px 12px;
  }
  .pane-toolbar {
    gap: 8px;
  }
  .version-chip {
    display: none;
  }
}
</style>
