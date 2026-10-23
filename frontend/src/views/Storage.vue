<template>
  <div class="pos-page storage-page">
    <PageHeader :title="t('storage.title')" :subtitle="t('storage.subtitle')">
      <el-button :icon="Refresh" :loading="loading" @click="reload">{{ t('common.refresh') }}</el-button>
      <el-button type="primary" :icon="Plus" @click="openCreatePool">{{ t('storage.createPool') }}</el-button>
    </PageHeader>

    <el-alert
      v-if="auth.dev"
      :title="t('storage.devHint')"
      type="info"
      show-icon
      :closable="false"
      class="dev-alert"
    />

    <div class="stat-grid">
      <StatCard
        :label="t('storage.disks')"
        :value="disks.length"
        icon="Coin"
        color="#2dd4bf"
        :hint="`${t('storage.healthPassed')} ${passedCount} · ${t('storage.healthFailing')} ${failingCount}`"
      />
      <StatCard
        :label="t('storage.total')"
        :value="formatBytes(totalBytes)"
        icon="DataLine"
        color="#60a5fa"
        :hint="`${t('storage.used')} ${formatBytes(usedBytes)}`"
      />
      <StatCard
        :label="t('storage.pools')"
        :value="pools.length"
        icon="Grid"
        color="#c084fc"
        :hint="`${t('common.running')} ${activePools}`"
      />
      <StatCard
        :label="t('storage.health')"
        :value="failingCount"
        :icon="failingCount ? 'WarningFilled' : 'CircleCheck'"
        :color="failingCount ? '#f87171' : '#34d399'"
        :hint="failingCount ? t('storage.healthFailing') : t('storage.healthPassed')"
      />
    </div>

    <section class="pos-panel" v-loading="loading">
      <div class="panel-head">
        <div>
          <div class="pos-card-title">{{ t('storage.disks') }}</div>
          <div class="pos-hint">{{ t('storage.diskCount', { n: disks.length }) }}</div>
        </div>
        <span class="pos-hint smart-hint">{{ t('storage.smartHint') }}</span>
      </div>

      <EmptyState
        v-if="!disks.length && !loading"
        :title="t('storage.noDisks')"
        :description="t('storage.smartHint')"
        icon="Coin"
      />
      <el-table v-else :data="disks" row-key="path" class="disk-table">
        <el-table-column :label="t('storage.model')" min-width="200">
          <template #default="{ row }">
            <div class="model-cell">
              <span class="model-name">{{ row.model || '—' }}</span>
              <span class="pos-hint pos-mono">{{ row.name }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.serial')" min-width="150">
          <template #default="{ row }"><span class="pos-mono">{{ row.serial || '—' }}</span></template>
        </el-table-column>
        <el-table-column :label="t('files.path')" min-width="140">
          <template #default="{ row }"><span class="pos-mono">{{ row.path }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.capacity')" width="118" align="right">
          <template #default="{ row }"><span class="num">{{ formatBytes(row.size_bytes) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('files.type')" width="104">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" type="info">{{ row.rotational ? 'HDD' : 'SSD' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.health')" width="100">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="healthMeta(row.health).type">
              {{ t(healthMeta(row.health).key) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.temperature')" width="96" align="right">
          <template #default="{ row }">{{ row.temperature ? `${row.temperature} °C` : '—' }}</template>
        </el-table-column>
        <el-table-column :label="t('storage.powerOn')" width="126" align="right">
          <template #default="{ row }"><span class="num">{{ formatPowerOn(row.power_on_hours) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.mountpoint')" min-width="150">
          <template #default="{ row }">
            <span v-if="row.mountpoint" class="pos-mono">{{ row.mountpoint }}</span>
            <span v-else class="pos-hint">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.percent')" min-width="160">
          <template #default="{ row }">
            <div v-if="row.mountpoint" class="usage-cell">
              <el-progress
                :percentage="percentValue(row.used_percent)"
                :stroke-width="6"
                :show-text="false"
                :status="progressStatus(row.used_percent)"
              />
              <span class="pos-hint">{{ formatPercent(percentValue(row.used_percent)) }} · {{ formatBytes(row.used_bytes) }}</span>
            </div>
            <span v-else class="pos-hint">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="130" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :icon="Odometer" @click="openSmart(row)">{{ t('storage.smartTest') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section class="pos-panel" v-loading="loading">
      <div class="panel-head">
        <div>
          <div class="pos-card-title">{{ t('storage.pools') }}</div>
          <div class="pos-hint">{{ t('common.running') }} {{ activePools }} · {{ t('storage.capacity') }} {{ formatBytes(managedPoolBytes) }}</div>
        </div>
        <el-button type="primary" plain :icon="Plus" @click="openCreatePool">{{ t('storage.createPool') }}</el-button>
      </div>

      <EmptyState
        v-if="!pools.length && !loading"
        :title="t('storage.emptyPools')"
        :description="t('storage.emptyPoolsHint')"
        icon="Coin"
      >
        <el-button type="primary" :icon="Plus" @click="openCreatePool">{{ t('storage.createPool') }}</el-button>
      </EmptyState>
      <el-table v-else :data="pools" row-key="id" class="pool-table">
        <el-table-column :label="t('storage.poolName')" min-width="130" prop="name" />
        <el-table-column :label="t('storage.raidLevel')" width="100">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" type="info">{{ row.level || '—' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.filesystem')" width="96">
          <template #default="{ row }"><span class="pos-mono">{{ row.fs || '—' }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.devices')" min-width="180">
          <template #default="{ row }">
            <div class="device-list">
              <el-tag v-for="device in row.devices || []" :key="device" size="small" effect="plain" class="pos-mono">
                {{ device }}
              </el-tag>
              <span v-if="!(row.devices || []).length" class="pos-hint">—</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.mountpoint')" min-width="150">
          <template #default="{ row }">
            <span v-if="row.mountpoint" class="pos-mono">{{ row.mountpoint }}</span>
            <span v-else class="pos-hint">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.status')" width="104">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="poolStatusMeta(row).type">{{ poolStatusMeta(row).label }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('storage.capacity')" width="118" align="right">
          <template #default="{ row }"><span class="num">{{ formatBytes(row.size_bytes) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.percent')" min-width="150">
          <template #default="{ row }">
            <div v-if="row.mountpoint" class="usage-cell">
              <el-progress
                :percentage="percentValue(row.used_percent)"
                :stroke-width="6"
                :show-text="false"
                :status="progressStatus(row.used_percent)"
              />
              <span class="pos-hint">{{ formatPercent(percentValue(row.used_percent)) }}</span>
            </div>
            <span v-else class="pos-hint">—</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="220" fixed="right">
          <template #default="{ row }">
            <el-button
              v-if="isPoolActive(row)"
              link
              type="primary"
              :loading="poolBusy === row.id"
              :disabled="!!poolBusy && poolBusy !== row.id"
              @click="changeMount(row, false)"
            >
              {{ t('storage.unmount') }}
            </el-button>
            <el-button
              v-else
              link
              type="primary"
              :loading="poolBusy === row.id"
              :disabled="!!poolBusy && poolBusy !== row.id"
              @click="changeMount(row, true)"
            >
              {{ t('storage.mount') }}
            </el-button>
            <el-button link type="danger" :disabled="!!poolBusy" @click="removePool(row)">
              {{ t('common.delete') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <section class="pos-panel" v-loading="loading">
      <div class="panel-head">
        <div>
          <div class="pos-card-title">{{ t('storage.mounts') }}</div>
          <div class="pos-hint">{{ t('common.total') }} {{ mounts.length }}</div>
        </div>
      </div>

      <EmptyState v-if="!mounts.length && !loading" :title="t('common.empty')" icon="FolderOpened" />
      <el-table v-else :data="mounts" row-key="mountpoint">
        <el-table-column :label="t('storage.devices')" min-width="160">
          <template #default="{ row }"><span class="pos-mono">{{ row.device || '—' }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.filesystem')" width="110">
          <template #default="{ row }"><el-tag size="small" effect="plain" type="info">{{ row.fstype || '—' }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="t('storage.mountpoint')" min-width="170">
          <template #default="{ row }"><span class="pos-mono">{{ row.mountpoint || '—' }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.total')" width="120" align="right">
          <template #default="{ row }"><span class="num">{{ formatBytes(row.size) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.used')" width="120" align="right">
          <template #default="{ row }"><span class="num">{{ formatBytes(row.used) }}</span></template>
        </el-table-column>
        <el-table-column :label="t('storage.percent')" min-width="190">
          <template #default="{ row }">
            <div class="usage-cell">
              <el-progress
                :percentage="percentValue(row.percent)"
                :stroke-width="6"
                :show-text="false"
                :status="progressStatus(row.percent)"
              />
              <span class="pos-hint">{{ formatPercent(percentValue(row.percent)) }}</span>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <el-drawer
      v-model="smartOpen"
      :title="t('storage.smartTest')"
      size="min(680px, 96vw)"
      destroy-on-close
    >
      <div v-if="smartDisk" v-loading="smartLoading" class="smart-wrap">
        <div class="smart-head">
          <div class="smart-title">
            <div class="pos-card-title">{{ smartModel }}</div>
            <div class="pos-hint pos-mono">{{ smartDisk.path }} · {{ smartSerial }}</div>
          </div>
          <el-tag size="small" effect="plain" :type="healthMeta(smartDisk.health).type">
            {{ t(healthMeta(smartDisk.health).key) }}
          </el-tag>
        </div>

        <el-descriptions v-if="smartDoc" :column="2" border size="small">
          <el-descriptions-item :label="t('storage.model')">{{ smartModel }}</el-descriptions-item>
          <el-descriptions-item :label="t('storage.serial')"><span class="pos-mono">{{ smartSerial }}</span></el-descriptions-item>
          <el-descriptions-item :label="t('storage.health')">
            <el-tag size="small" effect="plain" :type="smartStatusMeta.type">{{ t(smartStatusMeta.key) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('storage.temperature')">{{ smartTemperature }}</el-descriptions-item>
          <el-descriptions-item :label="t('storage.powerOn')">{{ smartPowerOn }}</el-descriptions-item>
          <el-descriptions-item :label="t('files.path')"><span class="pos-mono">{{ smartDisk.path }}</span></el-descriptions-item>
        </el-descriptions>
        <div v-else class="pos-hint">{{ t('common.empty') }}</div>

        <div class="smart-actions">
          <el-button
            type="primary"
            plain
            :icon="Timer"
            :loading="testing === 'short'"
            :disabled="!!testing && testing !== 'short'"
            @click="startSmartTest('short')"
          >
            {{ t('storage.smartShort') }}
          </el-button>
          <el-button
            plain
            :icon="Clock"
            :loading="testing === 'long'"
            :disabled="!!testing && testing !== 'long'"
            @click="startSmartTest('long')"
          >
            {{ t('storage.smartLong') }}
          </el-button>
          <span class="pos-hint">{{ t('storage.smartHint') }}</span>
        </div>

        <div v-if="ataAttributes.length" class="smart-section">
          <div class="pos-card-title">{{ t('storage.health') }}</div>
          <div class="attr-list">
            <div v-for="attribute in ataAttributes" :key="`${attribute.id}-${attribute.name}`" class="attr-row">
              <span class="attr-name pos-mono">#{{ attribute.id }} {{ attribute.name }}</span>
              <span class="attr-value pos-mono">
                <span class="attr-meta">{{ attrSummary(attribute) }}</span>
                <span>{{ attrRaw(attribute) }}</span>
              </span>
            </div>
          </div>
        </div>

        <div class="smart-section">
          <div class="panel-head">
            <div class="pos-card-title">{{ t('storage.smartResult') }}</div>
            <el-button link :icon="Refresh" :loading="testsLoading" @click="loadSmartTests(smartDisk.path)">
              {{ t('common.refresh') }}
            </el-button>
          </div>
          <el-table v-if="smartTests.length" :data="smartTests" size="small" row-key="id">
            <el-table-column :label="t('common.test')" width="110">
              <template #default="{ row }">{{ testKindLabel(row.kind) }}</template>
            </el-table-column>
            <el-table-column :label="t('common.status')" width="100">
              <template #default="{ row }">
                <el-tag size="small" effect="plain" :type="testStatusMeta(row.status).type">
                  {{ t(testStatusMeta(row.status).key) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.details')" min-width="180">
              <template #default="{ row }"><span class="pos-hint">{{ row.summary || '—' }}</span></template>
            </el-table-column>
            <el-table-column :label="t('logs.time')" width="150">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </el-table-column>
          </el-table>
          <div v-else class="pos-hint">{{ t('common.empty') }}</div>
        </div>
      </div>
    </el-drawer>

    <el-dialog
      v-model="poolOpen"
      :title="t('storage.createPool')"
      width="min(620px, 94vw)"
      destroy-on-close
    >
      <el-alert :title="t('storage.createConfirm')" type="warning" show-icon :closable="false" class="dialog-alert" />
      <el-form label-position="top" @submit.prevent>
        <div class="form-grid">
          <el-form-item :label="t('storage.poolName')" required>
            <el-input v-model="poolForm.name" :placeholder="t('storage.poolName')" />
          </el-form-item>
          <el-form-item :label="t('storage.raidLevel')" required>
            <el-select v-model="poolForm.level" style="width: 100%">
              <el-option v-for="level in levels" :key="level" :label="level" :value="level" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('storage.filesystem')" required>
            <el-select v-model="poolForm.fs" style="width: 100%">
              <el-option v-for="item in filesystems" :key="item" :label="item" :value="item" />
            </el-select>
          </el-form-item>
          <el-form-item :label="t('storage.mountpoint')">
            <el-input v-model="poolForm.mountpoint" placeholder="/mnt/..." class="pos-mono" />
          </el-form-item>
        </div>
        <el-form-item :label="t('storage.devices')" required>
          <el-select
            v-model="poolForm.devices"
            multiple
            filterable
            :placeholder="t('storage.devices')"
            style="width: 100%"
          >
            <el-option
              v-for="disk in disks"
              :key="disk.path"
              :label="`${disk.path} · ${disk.model || disk.name} (${formatBytes(disk.size_bytes)})`"
              :value="disk.path"
            />
          </el-select>
          <div class="pos-hint field-hint">{{ t('storage.createConfirm') }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="poolOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="creating" @click="createPool">{{ t('common.create') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Plus, Odometer, Timer, Clock } from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import StatCard from '../components/StatCard.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { formatBytes, formatDate, formatPercent } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()

const loading = ref(false)
const disks = ref([])
const pools = ref([])
const mounts = ref([])
const poolBusy = ref('')

const poolOpen = ref(false)
const creating = ref(false)
const levels = ['raid0', 'raid1', 'raid5', 'raid6', 'raid10', 'linear']
const filesystems = ['ext4', 'xfs', 'btrfs']
const poolForm = reactive({ name: '', level: 'raid1', fs: 'ext4', devices: [], mountpoint: '' })

const smartOpen = ref(false)
const smartDisk = ref(null)
const smartDoc = ref(null)
const smartTests = ref([])
const smartLoading = ref(false)
const testsLoading = ref(false)
const testing = ref('')

const HEALTH_META = {
  passed: { key: 'storage.healthPassed', type: 'success' },
  failing: { key: 'storage.healthFailing', type: 'danger' },
  unknown: { key: 'storage.healthUnknown', type: 'info' },
  unavailable: { key: 'storage.unavailable', type: 'info' }
}

const totalBytes = computed(() => disks.value.reduce((sum, disk) => sum + Number(disk.size_bytes || 0), 0))
const usedBytes = computed(() => disks.value.reduce((sum, disk) => sum + Number(disk.used_bytes || 0), 0))
const failingCount = computed(() => disks.value.filter((disk) => disk.health === 'failing').length)
const passedCount = computed(() => disks.value.filter((disk) => disk.health === 'passed').length)
const activePools = computed(() => pools.value.filter((pool) => isPoolActive(pool)).length)
const managedPoolBytes = computed(() => pools.value.reduce((sum, pool) => sum + Number(pool.size_bytes || 0), 0))

const smartStatus = computed(() => {
  const status = smartDoc.value?.smart_status
  if (status === null || status === undefined) return 'unknown'
  if (typeof status === 'boolean') return status ? 'passed' : 'failing'
  if (typeof status === 'object') {
    if (status.passed === true) return 'passed'
    if (status.passed === false) return 'failing'
    const text = String(status.string || '').toLowerCase()
    if (text.includes('pass')) return 'passed'
    if (text.includes('fail')) return 'failing'
  }
  return 'unknown'
})
const smartStatusMeta = computed(() => healthMeta(smartStatus.value))
const ataAttributes = computed(() => {
  const table = smartDoc.value?.ata_smart_attributes?.table
  return Array.isArray(table) ? table : []
})
const smartModel = computed(() => smartDoc.value?.model || smartDisk.value?.model || '—')
const smartSerial = computed(() => smartDoc.value?.serial || smartDisk.value?.serial || '—')
const smartTemperature = computed(() => {
  const temperature = smartDoc.value?.temperature
  if (typeof temperature === 'number') return `${temperature} °C`
  if (temperature && typeof temperature === 'object' && temperature.current !== undefined && temperature.current !== null) {
    return `${temperature.current} °C`
  }
  return '—'
})
const smartPowerOn = computed(() => {
  const powerOn = smartDoc.value?.power_on_time
  const hours = powerOn && typeof powerOn === 'object' ? powerOn.hours : powerOn
  if (hours === undefined || hours === null || hours === '') return '—'
  return `${hours} ${t('storage.hours')}`
})

function healthMeta(health) {
  return HEALTH_META[health] || HEALTH_META.unknown
}

function isPoolActive(pool) {
  return Boolean(pool.active) || pool.status === 'active'
}

function poolStatusMeta(pool) {
  if (isPoolActive(pool)) return { label: t('common.running'), type: 'success' }
  if (pool.status === 'creating') return { label: t('common.loading'), type: 'warning' }
  if (pool.status === 'inactive') return { label: t('common.stopped'), type: 'info' }
  return { label: t('common.unknown'), type: 'info' }
}

function formatPowerOn(hours) {
  const value = Number(hours || 0)
  return value > 0 ? `${value} ${t('storage.hours')}` : '—'
}

function percentValue(value) {
  const num = Number(value || 0)
  return Number.isFinite(num) ? Math.max(0, Math.min(100, num)) : 0
}

function progressStatus(value) {
  const percent = percentValue(value)
  if (percent >= 90) return 'exception'
  if (percent >= 75) return 'warning'
  return undefined
}

function testKindLabel(kind) {
  if (kind === 'short') return t('storage.smartShort')
  if (kind === 'long') return t('storage.smartLong')
  return kind || '—'
}

function testStatusMeta(status) {
  return status === 'completed'
    ? { key: 'logs.success', type: 'success' }
    : { key: 'logs.failed', type: 'danger' }
}

function attrSummary(attribute) {
  const parts = [attribute?.value, attribute?.worst, attribute?.thresh]
    .filter((value) => value !== undefined && value !== null)
  return parts.length ? parts.join(' / ') : '—'
}

function attrRaw(attribute) {
  const raw = attribute?.raw
  if (raw === undefined || raw === null) return '—'
  if (typeof raw !== 'object') return String(raw)
  if (raw.value !== undefined && raw.value !== null) {
    return raw.string ? `${raw.value} (${raw.string})` : String(raw.value)
  }
  const entries = Object.entries(raw).map(([key, value]) => `${key}=${rawValue(value)}`)
  return entries.length ? entries.join(' · ') : '—'
}

function rawValue(value) {
  if (value === undefined || value === null) return '—'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

async function loadAll(showError = true) {
  loading.value = true
  try {
    const [diskResult, poolResult, mountResult] = await Promise.allSettled([
      api.storage.disks(),
      api.storage.pools(),
      api.storage.mounts()
    ])
    if (diskResult.status === 'fulfilled') disks.value = diskResult.value || []
    if (poolResult.status === 'fulfilled') pools.value = poolResult.value || []
    if (mountResult.status === 'fulfilled') mounts.value = mountResult.value || []
    const failure = [diskResult, poolResult, mountResult].find((result) => result.status === 'rejected')
    if (failure && showError) ElMessage.error(errorMessage(failure.reason))
  } catch (error) {
    if (showError) ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

function reload() {
  loadAll(true)
}

function openCreatePool() {
  Object.assign(poolForm, { name: '', level: 'raid1', fs: 'ext4', devices: [], mountpoint: '' })
  poolOpen.value = true
  if (!disks.value.length) loadAll(false)
}

async function createPool() {
  const name = poolForm.name.trim()
  if (!name || !poolForm.devices.length) {
    ElMessage.warning(t('common.required'))
    return
  }
  try {
    await ElMessageBox.prompt(
      `${t('storage.createConfirm')} ${t('storage.confirmCreate')}`,
      t('storage.createPool'),
      {
        type: 'warning',
        confirmButtonText: t('common.create'),
        cancelButtonText: t('common.cancel'),
        inputPlaceholder: 'CREATE',
        inputValidator: (value) => value === 'CREATE' || t('storage.confirmCreate')
      }
    )
  } catch (error) {
    return
  }
  creating.value = true
  try {
    await api.storage.createPool({
      name,
      level: poolForm.level,
      fs: poolForm.fs,
      devices: [...poolForm.devices],
      mountpoint: poolForm.mountpoint.trim(),
      confirm: 'CREATE'
    })
    ElMessage.success(t('storage.poolCreated'))
    poolOpen.value = false
    await loadAll(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    creating.value = false
  }
}

async function changeMount(pool, mount) {
  if (poolBusy.value) return
  poolBusy.value = pool.id
  try {
    if (mount) await api.storage.mountPool(pool.id)
    else await api.storage.unmountPool(pool.id)
    ElMessage.success(t('common.operationSuccess'))
    await loadAll(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    poolBusy.value = ''
  }
}

async function removePool(pool) {
  try {
    await ElMessageBox.prompt(
      `${t('storage.deleteConfirm')} ${t('storage.confirmDelete')}`,
      t('storage.deletePool'),
      {
        type: 'warning',
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
        inputPlaceholder: 'DELETE',
        inputValidator: (value) => value === 'DELETE' || t('storage.confirmDelete')
      }
    )
  } catch (error) {
    return
  }
  poolBusy.value = pool.id
  try {
    await api.storage.deletePool(pool.id, { confirm: 'DELETE', wipe: false })
    ElMessage.success(t('storage.poolDeleted'))
    await loadAll(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    poolBusy.value = ''
  }
}

async function openSmart(disk) {
  smartDisk.value = disk
  smartDoc.value = null
  smartTests.value = []
  smartOpen.value = true
  smartLoading.value = true
  try {
    const [docResult, testsResult] = await Promise.allSettled([
      api.storage.smart(disk.path),
      api.storage.smartTests(disk.path)
    ])
    if (docResult.status === 'fulfilled') smartDoc.value = docResult.value || null
    if (testsResult.status === 'fulfilled') {
      smartTests.value = Array.isArray(testsResult.value) ? testsResult.value : []
    }
    if (docResult.status === 'rejected') ElMessage.error(errorMessage(docResult.reason))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    smartLoading.value = false
  }
}

async function loadSmartTests(device) {
  if (!device) return
  testsLoading.value = true
  try {
    const tests = await api.storage.smartTests(device)
    smartTests.value = Array.isArray(tests) ? tests : []
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    testsLoading.value = false
  }
}

async function startSmartTest(kind) {
  if (!smartDisk.value || testing.value) return
  testing.value = kind
  try {
    await api.storage.smartTest({ device: smartDisk.value.path, kind })
    ElMessage.success(t('storage.smartStarted'))
    await loadSmartTests(smartDisk.value.path)
    loadAll(false)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    testing.value = ''
  }
}

onMounted(() => {
  loadAll(true)
})
</script>

<style scoped>
.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 16px;
}

.dev-alert {
  border-radius: 12px;
}

section.pos-panel {
  padding: 18px 20px;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.panel-head > div:first-child {
  min-width: 0;
}

.panel-head .pos-hint {
  margin-top: 3px;
}

.smart-hint {
  max-width: 460px;
  text-align: right;
}

.model-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.model-name {
  font-size: 12.5px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.num {
  font-variant-numeric: tabular-nums;
}

.device-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.device-list :deep(.el-tag) {
  margin: 0;
  font-size: 10.5px;
}

.usage-cell {
  display: flex;
  flex-direction: column;
  gap: 5px;
  min-width: 90px;
}

.usage-cell :deep(.el-progress) {
  width: 100%;
}

.smart-wrap {
  display: flex;
  flex-direction: column;
  gap: 18px;
  min-height: 200px;
}

.smart-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.smart-title {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.smart-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 14px;
  border: 1px solid var(--pos-border);
  border-radius: 12px;
  background: var(--pos-panel-2);
}

.smart-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.attr-list {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--pos-border);
  border-radius: 10px;
  overflow: hidden;
}

.attr-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 9px 12px;
  background: var(--pos-panel-2);
  border-bottom: 1px solid var(--pos-border);
}

.attr-row:last-child {
  border-bottom: none;
}

.attr-name {
  color: var(--pos-text);
  font-size: 12px;
  word-break: break-word;
}

.attr-value {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
  color: var(--pos-muted);
  font-size: 11.5px;
  text-align: right;
  word-break: break-word;
  max-width: 64%;
}

.attr-meta {
  color: var(--pos-accent);
}

.dialog-alert {
  margin-bottom: 16px;
  border-radius: 10px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.field-hint {
  margin-top: 6px;
}

@media (max-width: 1200px) {
  .stat-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .stat-grid {
    grid-template-columns: 1fr;
  }

  section.pos-panel {
    padding: 14px;
  }

  .smart-hint {
    text-align: left;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .smart-head {
    flex-direction: column;
  }

  .attr-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .attr-value {
    align-items: flex-start;
    text-align: left;
    max-width: 100%;
  }
}
</style>
