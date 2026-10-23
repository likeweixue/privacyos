<template>
  <div class="pos-page network-page">
    <PageHeader :title="t('network.title')" :subtitle="t('network.subtitle')">
      <el-tag v-if="hostname" effect="plain" type="info" class="hostname-tag">
        {{ t('network.hostname') }} · {{ hostname }}
      </el-tag>
      <el-button :icon="Refresh" :loading="refreshing" @click="refreshActive">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <section class="pos-panel tabs-panel">
      <el-tabs v-model="activeTab" class="net-tabs">
        <el-tab-pane :label="t('network.interfaces')" name="interfaces">
          <el-alert
            :title="t('network.riskHint')"
            type="warning"
            show-icon
            :closable="false"
            class="tab-alert"
          />

          <div v-loading="interfacesLoading" class="iface-wrap">
            <EmptyState
              v-if="!interfaces.length && !interfacesLoading"
              :title="t('common.empty')"
              icon="Connection"
            />
            <div v-else class="iface-grid">
              <article v-for="iface in interfaces" :key="iface.name" class="iface-card">
                <div class="iface-head">
                  <div class="iface-name">
                    <span class="iface-icon">
                      <el-icon :size="16"><Connection /></el-icon>
                    </span>
                    <strong class="pos-mono">{{ iface.name }}</strong>
                  </div>
                  <el-tag size="small" effect="plain" :type="ifaceStateType(iface.state)">
                    {{ iface.state || t('common.unknown') }}
                  </el-tag>
                </div>

                <div class="iface-rows">
                  <div class="kv-row">
                    <span class="kv-label">{{ t('network.mac') }}</span>
                    <span class="kv-value pos-mono">{{ iface.mac || '—' }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-label">{{ t('network.mtu') }}</span>
                    <span class="kv-value pos-mono">{{ iface.mtu || '—' }}</span>
                  </div>
                  <div class="kv-row">
                    <span class="kv-label">{{ t('network.mode') }}</span>
                    <el-tag size="small" effect="plain" type="info">{{ modeLabel(ifaceMode(iface)) }}</el-tag>
                  </div>
                  <div class="kv-row">
                    <span class="kv-label">{{ t('network.address') }}</span>
                    <div class="address-list">
                      <span v-for="address in iface.addresses || []" :key="address" class="address-pill pos-mono">
                        {{ address }}
                      </span>
                      <span v-if="!(iface.addresses || []).length" class="pos-hint">—</span>
                    </div>
                  </div>
                  <div v-if="ifaceGateway(iface)" class="kv-row">
                    <span class="kv-label">{{ t('network.gateway') }}</span>
                    <span class="kv-value pos-mono">{{ ifaceGateway(iface) }}</span>
                  </div>
                </div>

                <div class="iface-foot">
                  <el-button size="small" :icon="Edit" :disabled="!canEdit(iface)" @click="openInterface(iface)">
                    {{ t('common.edit') }}
                  </el-button>
                </div>
              </article>
            </div>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('network.shares')" name="shares">
          <div class="tab-head">
            <div class="tab-title">
              <div class="pos-card-title">{{ t('network.shares') }}</div>
              <div class="pos-hint path-hint">
                <span class="path-item">{{ t('network.sambaConf') }}: <code class="pos-mono">{{ sambaPath || '—' }}</code></span>
                <span class="path-item">{{ t('network.exports') }}: <code class="pos-mono">{{ exportsPath || '—' }}</code></span>
              </div>
            </div>
            <div class="pos-toolbar">
              <el-input
                v-model="shareQuery"
                class="filter-input"
                :placeholder="t('network.searchPlaceholder')"
                clearable
                :prefix-icon="Search"
              />
              <el-button :icon="Upload" :loading="applying" @click="applyShares">{{ t('network.applyShares') }}</el-button>
              <el-button type="primary" :icon="Plus" @click="openShare()">{{ t('network.addShare') }}</el-button>
            </div>
          </div>

          <el-alert
            v-if="applyWarnings.length"
            :title="t('common.warning')"
            type="warning"
            show-icon
            class="tab-alert"
            @close="clearWarnings"
          >
            <div v-for="(warning, index) in applyWarnings" :key="index" class="warning-line">{{ warning }}</div>
          </el-alert>

          <div v-loading="sharesLoading" class="table-wrap">
            <EmptyState
              v-if="!shares.length && !sharesLoading"
              :title="t('network.noShares')"
              icon="Share"
            >
              <el-button type="primary" :icon="Plus" @click="openShare()">{{ t('network.addShare') }}</el-button>
            </EmptyState>
            <div v-else-if="!filteredShares.length && !sharesLoading" class="pos-hint empty-hint">
              {{ t('common.empty') }}
            </div>
            <el-table v-else :data="filteredShares" row-key="id" class="share-table">
              <el-table-column :label="t('network.shareName')" min-width="130" prop="name" />
              <el-table-column :label="t('network.sharePath')" min-width="180">
                <template #default="{ row }"><span class="pos-mono">{{ row.path }}</span></template>
              </el-table-column>
              <el-table-column :label="t('network.comment')" min-width="140">
                <template #default="{ row }"><span class="pos-hint">{{ row.comment || '—' }}</span></template>
              </el-table-column>
              <el-table-column :label="t('network.protocols')" width="140">
                <template #default="{ row }">
                  <el-tag
                    v-for="protocol in row.protocols || []"
                    :key="protocol"
                    size="small"
                    effect="plain"
                    class="proto-tag"
                    :type="protocolType(protocol)"
                  >
                    {{ String(protocol).toUpperCase() }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t('network.readOnly')" width="96">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain" :type="row.read_only ? 'success' : 'warning'">
                    {{ row.read_only ? t('common.yes') : t('common.no') }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t('network.validUsers')" min-width="130">
                <template #default="{ row }">
                  <span class="pos-hint">{{ row.valid_users || t('common.all') }}</span>
                </template>
              </el-table-column>
              <el-table-column :label="t('network.guestOk')" width="100">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain" :type="row.guest_ok ? 'warning' : 'success'">
                    {{ row.guest_ok ? t('common.yes') : t('common.no') }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t('common.actions')" width="170" fixed="right">
                <template #default="{ row }">
                  <el-button link type="primary" :icon="Edit" @click="openShare(row)">{{ t('common.edit') }}</el-button>
                  <el-button link type="danger" :icon="Delete" @click="removeShare(row)">{{ t('common.delete') }}</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('network.ports')" name="ports">
          <div class="tab-head">
            <div class="tab-title">
              <div class="pos-card-title">{{ t('network.ports') }}</div>
              <div class="pos-hint">{{ t('common.total') }} {{ filteredPorts.length }}</div>
            </div>
            <div class="pos-toolbar">
              <el-input
                v-model="portQuery"
                class="filter-input"
                :placeholder="t('common.searchPlaceholder')"
                clearable
                :prefix-icon="Search"
              />
            </div>
          </div>

          <div v-loading="portsLoading" class="table-wrap">
            <EmptyState
              v-if="!ports.length && !portsLoading"
              :title="t('common.empty')"
              icon="Connection"
            />
            <div v-else-if="!filteredPorts.length && !portsLoading" class="pos-hint empty-hint">
              {{ t('common.empty') }}
            </div>
            <el-table v-else :data="filteredPorts" class="port-table">
              <el-table-column :label="t('network.protocol')" width="100">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain" :type="protocolType(row.protocol)">
                    {{ String(row.protocol || '').toUpperCase() }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t('network.address')" min-width="160">
                <template #default="{ row }"><span class="pos-mono">{{ row.address || '—' }}</span></template>
              </el-table-column>
              <el-table-column :label="t('network.port')" width="100" align="right">
                <template #default="{ row }"><span class="num">{{ row.port ?? '—' }}</span></template>
              </el-table-column>
              <el-table-column :label="t('network.process')" min-width="180">
                <template #default="{ row }"><span class="pos-mono">{{ row.process || '—' }}</span></template>
              </el-table-column>
              <el-table-column :label="t('monitor.pid')" width="100" align="right">
                <template #default="{ row }"><span class="num">{{ row.pid || '—' }}</span></template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane :label="ddnsTabLabel" name="ddns">
          <div class="ddns-grid">
            <section class="sub-panel">
              <div class="tab-head">
                <div class="tab-title">
                  <div class="pos-card-title">{{ t('network.ddns') }}</div>
                </div>
                <el-tag effect="plain" :type="ddnsForm.enabled ? 'success' : 'info'">
                  {{ ddnsForm.enabled ? t('common.enabled') : t('common.disabled') }}
                </el-tag>
              </div>

              <div v-loading="ddnsLoading">
                <el-form label-position="top" @submit.prevent>
                  <el-form-item :label="t('network.enableDDNS')">
                    <el-switch v-model="ddnsForm.enabled" />
                  </el-form-item>
                  <div class="form-grid">
                    <el-form-item :label="t('network.provider')">
                      <el-select v-model="ddnsForm.provider" style="width: 100%">
                        <el-option label="DuckDNS" value="duckdns" />
                        <el-option label="Cloudflare" value="cloudflare" />
                        <el-option label="Custom" value="custom" />
                      </el-select>
                    </el-form-item>
                    <el-form-item :label="t('network.domain')">
                      <el-input v-model="ddnsForm.domain" placeholder="nas.example.com" class="pos-mono" />
                    </el-form-item>
                    <el-form-item :label="t('network.interval')">
                      <el-input-number
                        v-model="ddnsForm.interval"
                        :min="60"
                        :step="60"
                        controls-position="right"
                        style="width: 100%"
                      />
                    </el-form-item>
                  </div>
                  <el-form-item :label="t('network.token')">
                    <el-input
                      v-model="ddnsForm.token"
                      show-password
                      :placeholder="ddnsTokenPlaceholder"
                      class="pos-mono"
                    />
                    <div v-if="savedToken" class="pos-hint field-hint">{{ t('network.unsavedSecrets') }}</div>
                  </el-form-item>
                  <el-form-item v-if="ddnsForm.provider === 'custom'" :label="t('network.address')">
                    <el-input
                      v-model="ddnsForm.url"
                      placeholder="https://example.com/update"
                      class="pos-mono"
                    />
                  </el-form-item>

                  <div class="ddns-meta">
                    <div class="meta-item">
                      <span>{{ t('network.lastIP') }}</span>
                      <strong class="pos-mono">{{ ddns.last_ip || '—' }}</strong>
                    </div>
                    <div class="meta-item">
                      <span>{{ t('network.lastUpdate') }}</span>
                      <strong>{{ ddns.last_run ? formatDate(ddns.last_run) : t('common.never') }}</strong>
                    </div>
                    <div v-if="ddns.last_err" class="meta-item error">
                      <span>{{ t('common.error') }}</span>
                      <strong>{{ ddns.last_err }}</strong>
                    </div>
                  </div>

                  <div class="form-actions">
                    <el-button type="primary" :loading="savingDDNS" @click="saveDDNS">{{ t('common.save') }}</el-button>
                    <el-button
                      :loading="updatingDDNS"
                      :disabled="!ddnsForm.enabled"
                      @click="updateDDNSNow"
                    >
                      {{ t('network.updateNow') }}
                    </el-button>
                  </div>
                </el-form>
              </div>
            </section>

            <section class="sub-panel">
              <div class="tab-head">
                <div class="tab-title">
                  <div class="pos-card-title">{{ t('network.dns') }}</div>
                </div>
                <el-button type="primary" plain :loading="savingDNS" @click="saveDNS">{{ t('network.saveDns') }}</el-button>
              </div>

              <div v-loading="dnsLoading" class="dns-body">
                <el-form label-position="top" @submit.prevent>
                  <el-form-item :label="t('network.dnsServers')">
                    <el-select
                      v-model="dnsServers"
                      multiple
                      filterable
                      allow-create
                      default-first-option
                      :reserve-keyword="false"
                      :placeholder="t('network.dnsServers')"
                      style="width: 100%"
                    >
                      <el-option v-for="server in dnsOptions" :key="server" :label="server" :value="server" />
                    </el-select>
                  </el-form-item>
                </el-form>

                <div class="dns-meta">
                  <span>{{ t('network.hostname') }} <strong class="pos-mono">{{ dnsHostname || hostname || '—' }}</strong></span>
                  <span v-if="dnsDomain">{{ t('network.domain') }} <strong class="pos-mono">{{ dnsDomain }}</strong></span>
                </div>
              </div>
            </section>
          </div>
        </el-tab-pane>
      </el-tabs>
    </section>

    <el-dialog
      v-model="ifaceOpen"
      :title="t('network.applyConfig')"
      width="min(520px, 94vw)"
      destroy-on-close
    >
      <el-alert :title="t('network.riskHint')" type="warning" show-icon :closable="false" class="dialog-alert" />
      <el-form label-position="top" @submit.prevent>
        <el-form-item :label="t('network.interface')">
          <el-input :model-value="ifaceForm.name" disabled class="pos-mono" />
        </el-form-item>
        <el-form-item :label="t('network.mode')">
          <el-radio-group v-model="ifaceForm.mode">
            <el-radio-button value="dhcp">{{ t('network.dhcp') }}</el-radio-button>
            <el-radio-button value="static">{{ t('network.static') }}</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <template v-if="ifaceForm.mode === 'static'">
          <el-form-item :label="t('network.address')">
            <el-input v-model="ifaceForm.address" placeholder="192.168.1.20/24" class="pos-mono" />
          </el-form-item>
          <el-form-item :label="t('network.gateway')">
            <el-input v-model="ifaceForm.gateway" placeholder="192.168.1.1" class="pos-mono" />
          </el-form-item>
        </template>
        <el-form-item :label="t('network.dns')">
          <el-input v-model="ifaceForm.dns" placeholder="1.1.1.1 8.8.8.8" class="pos-mono" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ifaceOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="applyingInterface" @click="applyInterfaceConfig">{{ t('common.apply') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="shareOpen"
      :title="shareForm.id ? t('common.edit') : t('network.addShare')"
      width="min(560px, 94vw)"
      destroy-on-close
    >
      <el-form label-position="top" @submit.prevent>
        <div class="form-grid">
          <el-form-item :label="t('network.shareName')" required>
            <el-input v-model="shareForm.name" />
          </el-form-item>
          <el-form-item :label="t('network.sharePath')" required>
            <el-input v-model="shareForm.path" placeholder="/data/..." class="pos-mono" />
          </el-form-item>
        </div>
        <el-form-item :label="t('network.comment')">
          <el-input v-model="shareForm.comment" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item :label="t('network.protocols')">
            <el-select v-model="shareForm.protocols" multiple style="width: 100%">
              <el-option label="SMB" value="smb" />
              <el-option label="NFS" value="nfs" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('network.validUsers')">
            <el-input v-model="shareForm.valid_users" placeholder="@users" />
          </el-form-item>
        </div>
        <div class="switch-row">
          <el-form-item :label="t('network.readOnly')">
            <el-switch v-model="shareForm.read_only" />
          </el-form-item>
          <el-form-item :label="t('network.guestOk')">
            <el-switch v-model="shareForm.guest_ok" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="shareOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="savingShare" @click="saveShare">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Edit, Delete, Plus, Search, Connection, Upload } from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, errorMessage } from '../api'
import { formatDate } from '../utils/format'

const { t } = useI18n()
const KEEP_SECRET = '__keep__'

const activeTab = ref('interfaces')
const refreshing = ref(false)
const hostname = ref('')

const interfaces = ref([])
const interfaceConfig = ref({})
const interfacesLoading = ref(false)

const ports = ref([])
const portsLoading = ref(false)
const portQuery = ref('')

const shares = ref([])
const sharesLoading = ref(false)
const shareQuery = ref('')
const sambaPath = ref('')
const exportsPath = ref('')
const applying = ref(false)
const applyWarnings = ref([])

const ddns = ref({})
const ddnsForm = reactive({ enabled: false, provider: 'duckdns', domain: '', token: '', interval: 300, url: '' })
const savedToken = ref(false)
const ddnsLoading = ref(false)
const savingDDNS = ref(false)
const updatingDDNS = ref(false)

const dnsServers = ref([])
const dnsHostname = ref('')
const dnsDomain = ref('')
const dnsLoading = ref(false)
const savingDNS = ref(false)

const ifaceOpen = ref(false)
const applyingInterface = ref(false)
const ifaceForm = reactive({ name: '', mode: 'dhcp', address: '', gateway: '', dns: '' })

const shareOpen = ref(false)
const savingShare = ref(false)
const shareForm = reactive({
  id: '',
  name: '',
  path: '',
  comment: '',
  protocols: ['smb'],
  valid_users: '',
  read_only: false,
  guest_ok: false
})

const loadedTabs = reactive({ interfaces: false, shares: false, ports: false, ddns: false, dns: false })

const filteredShares = computed(() => {
  const query = shareQuery.value.trim().toLowerCase()
  if (!query) return shares.value
  return shares.value.filter((share) =>
    [share.name, share.path, share.comment, share.valid_users, ...(share.protocols || [])]
      .some((value) => String(value || '').toLowerCase().includes(query))
  )
})

const filteredPorts = computed(() => {
  const query = portQuery.value.trim().toLowerCase()
  if (!query) return ports.value
  return ports.value.filter((port) =>
    [port.protocol, port.address, port.port, port.process, port.pid]
      .some((value) => String(value ?? '').toLowerCase().includes(query))
  )
})

const dnsOptions = computed(() => [...new Set(dnsServers.value)])
const ddnsTabLabel = computed(() => `${t('network.ddns')} + ${t('network.dns')}`)
const ddnsTokenPlaceholder = computed(() =>
  savedToken.value && !ddnsForm.token ? t('network.unsavedSecrets') : t('network.token')
)

function ifaceStateType(state) {
  const value = String(state || '').toUpperCase()
  if (value === 'UP') return 'success'
  if (value === 'DOWN') return 'info'
  return 'warning'
}

function modeLabel(mode) {
  if (mode === 'dhcp') return t('network.dhcp')
  if (mode === 'static') return t('network.static')
  return mode || t('common.unknown')
}

function ifaceMode(iface) {
  return interfaceConfig.value[iface.name]?.mode || iface.mode || 'dhcp'
}

function ifaceGateway(iface) {
  return interfaceConfig.value[iface.name]?.gateway || iface.gateway || ''
}

function canEdit(iface) {
  return iface.name !== 'lo' && iface.mode !== 'loopback'
}

function protocolType(protocol) {
  const value = String(protocol || '').toLowerCase()
  if (value === 'smb' || value === 'tcp') return 'success'
  if (value === 'nfs' || value === 'udp') return 'warning'
  return 'info'
}

async function loadInterfaces(showError = true) {
  interfacesLoading.value = true
  try {
    const payload = await api.network.interfaces()
    interfaces.value = Array.isArray(payload?.interfaces) ? payload.interfaces : []
    interfaceConfig.value = payload?.config?.interfaces || {}
    if (payload?.config?.hostname) hostname.value = payload.config.hostname
    loadedTabs.interfaces = true
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    interfacesLoading.value = false
  }
}

async function loadShares(showError = true) {
  sharesLoading.value = true
  try {
    const payload = await api.network.shares()
    shares.value = Array.isArray(payload?.shares) ? payload.shares : []
    sambaPath.value = payload?.samba_path || ''
    exportsPath.value = payload?.exports_path || ''
    if (payload?.hostname) hostname.value = payload.hostname
    loadedTabs.shares = true
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    sharesLoading.value = false
  }
}

async function loadPorts(showError = true) {
  portsLoading.value = true
  try {
    const payload = await api.network.ports()
    ports.value = Array.isArray(payload) ? payload : []
    loadedTabs.ports = true
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    portsLoading.value = false
  }
}

async function loadDDNS(showError = true) {
  ddnsLoading.value = true
  try {
    const payload = await api.network.ddns()
    ddns.value = payload || {}
    savedToken.value = payload?.token === KEEP_SECRET
    Object.assign(ddnsForm, {
      enabled: Boolean(payload?.enabled),
      provider: payload?.provider || 'duckdns',
      domain: payload?.domain || '',
      token: payload?.token === KEEP_SECRET ? '' : (payload?.token || ''),
      interval: Number(payload?.interval) || 300,
      url: payload?.url || ''
    })
    loadedTabs.ddns = true
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    ddnsLoading.value = false
  }
}

async function loadDNS(showError = true) {
  dnsLoading.value = true
  try {
    const payload = await api.network.dns()
    dnsServers.value = Array.isArray(payload?.servers) ? [...payload.servers] : []
    dnsHostname.value = payload?.hostname || ''
    dnsDomain.value = payload?.domain || ''
    loadedTabs.dns = true
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    dnsLoading.value = false
  }
}

function ensureTab(tab) {
  if (tab === 'ports' && !loadedTabs.ports) loadPorts()
  if (tab === 'ddns') {
    if (!loadedTabs.ddns) loadDDNS()
    if (!loadedTabs.dns) loadDNS()
  }
}

async function refreshActive() {
  refreshing.value = true
  try {
    if (activeTab.value === 'interfaces') await loadInterfaces()
    else if (activeTab.value === 'shares') await loadShares()
    else if (activeTab.value === 'ports') await loadPorts()
    else if (activeTab.value === 'ddns') await Promise.all([loadDDNS(), loadDNS()])
  } finally {
    refreshing.value = false
  }
}

function openInterface(iface) {
  const config = interfaceConfig.value[iface.name] || {}
  const configuredMode = config.mode === 'static' || config.mode === 'dhcp' ? config.mode : ''
  Object.assign(ifaceForm, {
    name: iface.name,
    mode: configuredMode || (iface.mode === 'static' ? 'static' : 'dhcp'),
    address: config.address || (iface.addresses || [])[0] || '',
    gateway: config.gateway || iface.gateway || '',
    dns: config.dns || ''
  })
  ifaceOpen.value = true
}

async function applyInterfaceConfig() {
  if (!ifaceForm.name) return
  if (ifaceForm.mode === 'static' && !ifaceForm.address.trim()) {
    ElMessage.warning(t('common.required'))
    return
  }
  try {
    await ElMessageBox.prompt(
      `${t('network.riskHint')} ${t('network.confirmApply')}`,
      t('network.applyConfig'),
      {
        type: 'warning',
        confirmButtonText: t('common.apply'),
        cancelButtonText: t('common.cancel'),
        inputPlaceholder: 'APPLY',
        inputValidator: (value) => value === 'APPLY' || t('network.confirmApply')
      }
    )
  } catch (error) {
    return
  }
  applyingInterface.value = true
  try {
    await api.network.applyInterface(ifaceForm.name, {
      mode: ifaceForm.mode,
      address: ifaceForm.mode === 'static' ? ifaceForm.address.trim() : '',
      gateway: ifaceForm.mode === 'static' ? ifaceForm.gateway.trim() : '',
      dns: ifaceForm.dns.trim(),
      confirm: 'APPLY'
    })
    ElMessage.success(t('network.applySuccess'))
    ifaceOpen.value = false
    await loadInterfaces(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    applyingInterface.value = false
  }
}

function handleApplyResult(result) {
  if (!result) return
  if (result.apply?.samba_path) sambaPath.value = result.apply.samba_path
  if (result.apply?.exports_path) exportsPath.value = result.apply.exports_path
  const warnings = Array.isArray(result.apply?.warnings) ? [...result.apply.warnings] : []
  if (result.apply_error) warnings.push(result.apply_error)
  applyWarnings.value = warnings
  if (warnings.length) ElMessage.warning(t('common.warning'))
}

function clearWarnings() {
  applyWarnings.value = []
}

async function applyShares() {
  applying.value = true
  try {
    const result = await api.network.apply()
    applyWarnings.value = Array.isArray(result?.warnings) ? result.warnings : []
    if (result?.samba_path) sambaPath.value = result.samba_path
    if (result?.exports_path) exportsPath.value = result.exports_path
    if (applyWarnings.value.length) ElMessage.warning(t('common.warning'))
    else ElMessage.success(t('network.applySuccess'))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    applying.value = false
  }
}

function openShare(row = null) {
  Object.assign(shareForm, {
    id: row?.id || '',
    name: row?.name || '',
    path: row?.path || '',
    comment: row?.comment || '',
    protocols: Array.isArray(row?.protocols) && row.protocols.length ? [...row.protocols] : ['smb'],
    valid_users: row?.valid_users || '',
    read_only: Boolean(row?.read_only),
    guest_ok: Boolean(row?.guest_ok)
  })
  shareOpen.value = true
}

async function saveShare() {
  const name = shareForm.name.trim()
  const path = shareForm.path.trim()
  if (!name || !path) {
    ElMessage.warning(t('common.required'))
    return
  }
  const editingID = shareForm.id
  const payload = {
    name,
    path,
    comment: shareForm.comment.trim(),
    protocols: shareForm.protocols.length ? [...shareForm.protocols] : ['smb'],
    valid_users: shareForm.valid_users.trim(),
    read_only: shareForm.read_only,
    guest_ok: shareForm.guest_ok
  }
  savingShare.value = true
  try {
    const result = editingID
      ? await api.network.updateShare(editingID, payload)
      : await api.network.createShare(payload)
    ElMessage.success(editingID ? t('network.shareUpdated') : t('network.shareCreated'))
    handleApplyResult(result)
    shareOpen.value = false
    await loadShares(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingShare.value = false
  }
}

async function removeShare(row) {
  try {
    await ElMessageBox.confirm(
      `${t('network.shareName')}: ${row.name}`,
      t('common.delete'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel')
      }
    )
  } catch (error) {
    return
  }
  try {
    await api.network.deleteShare(row.id)
    ElMessage.success(t('network.shareDeleted'))
    await loadShares(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function saveDDNS() {
  savingDDNS.value = true
  try {
    await api.network.setDDNS({
      enabled: ddnsForm.enabled,
      provider: ddnsForm.provider,
      domain: ddnsForm.domain.trim(),
      token: !ddnsForm.token && savedToken.value ? KEEP_SECRET : ddnsForm.token,
      interval: Number(ddnsForm.interval) || 300,
      url: ddnsForm.url.trim()
    })
    ElMessage.success(t('network.ddnsSaved'))
    await loadDDNS(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingDDNS.value = false
  }
}

async function updateDDNSNow() {
  updatingDDNS.value = true
  try {
    await api.network.updateDDNS()
    ElMessage.success(t('common.operationSuccess'))
    await loadDDNS(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
    await loadDDNS(false)
  } finally {
    updatingDDNS.value = false
  }
}

async function saveDNS() {
  const servers = dnsServers.value.map((server) => String(server).trim()).filter(Boolean)
  if (!servers.length) {
    ElMessage.warning(t('common.required'))
    return
  }
  savingDNS.value = true
  try {
    await api.network.setDNS(servers)
    ElMessage.success(t('network.configSaved'))
    await loadDNS(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    savingDNS.value = false
  }
}

watch(activeTab, ensureTab)

onMounted(() => {
  loadInterfaces()
  loadShares()
})
</script>

<style scoped>
.hostname-tag {
  font-family: 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
  font-size: 11px;
}

.tabs-panel {
  padding: 10px 20px 20px;
}

.net-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}

.net-tabs :deep(.el-tabs__nav-wrap::after) {
  background-color: var(--pos-border);
}

.tab-alert {
  margin-bottom: 16px;
  border-radius: 10px;
}

.warning-line {
  line-height: 1.65;
  word-break: break-word;
}

.iface-wrap {
  min-height: 160px;
}

.iface-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 14px;
}

.iface-card {
  background: var(--pos-panel-2);
  border: 1px solid var(--pos-border);
  border-radius: 12px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-width: 0;
}

.iface-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.iface-name {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  font-size: 13px;
}

.iface-icon {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  flex-shrink: 0;
}

.iface-rows {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.kv-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  font-size: 12px;
  min-width: 0;
}

.kv-label {
  color: var(--pos-muted);
  white-space: nowrap;
}

.kv-value {
  text-align: right;
  word-break: break-all;
}

.address-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  justify-content: flex-end;
}

.address-pill {
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  font-size: 10.5px;
}

.iface-foot {
  border-top: 1px dashed var(--pos-border);
  padding-top: 12px;
}

.tab-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.tab-title {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.path-hint {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 18px;
}

.path-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.path-item code {
  color: var(--pos-text);
}

.filter-input {
  width: 220px;
}

.table-wrap {
  min-height: 150px;
}

.empty-hint {
  padding: 30px 0;
  text-align: center;
}

.proto-tag + .proto-tag {
  margin-left: 4px;
}

.num {
  font-variant-numeric: tabular-nums;
}

.ddns-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr);
  gap: 16px;
}

.sub-panel {
  border: 1px solid var(--pos-border);
  border-radius: 12px;
  padding: 16px;
  background: var(--pos-panel-2);
  min-width: 0;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.ddns-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 22px;
  padding: 12px;
  border-radius: 10px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  margin-bottom: 16px;
}

.meta-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 120px;
  font-size: 11.5px;
}

.meta-item span {
  color: var(--pos-muted);
}

.meta-item strong {
  font-weight: 500;
  word-break: break-all;
}

.meta-item.error strong {
  color: var(--pos-danger);
}

.form-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.field-hint {
  margin-top: 5px;
}

.dns-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.dns-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  color: var(--pos-muted);
  font-size: 12px;
}

.dns-meta strong {
  font-weight: 500;
  color: var(--pos-text);
}

.dialog-alert {
  margin-bottom: 16px;
  border-radius: 10px;
}

.switch-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.sub-panel :deep(.el-form-item:last-child) {
  margin-bottom: 0;
}

@media (max-width: 1100px) {
  .ddns-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .tabs-panel {
    padding: 10px 14px 16px;
  }

  .iface-grid {
    grid-template-columns: 1fr;
  }

  .filter-input {
    width: 100%;
  }

  .tab-head {
    flex-direction: column;
    align-items: stretch;
  }

  .pos-toolbar {
    flex-wrap: wrap;
  }

  .form-grid,
  .switch-row {
    grid-template-columns: 1fr;
  }

  .address-list {
    justify-content: flex-start;
  }
}
</style>
