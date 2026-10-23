<template>
  <div class="pos-page logs-page">
    <PageHeader :title="t('logs.title')" :subtitle="t('logs.subtitle')">
      <span class="pos-hint retention-hint">{{ t('logs.retentionHint') }}</span>
      <el-button type="primary" :icon="Download" @click="exportCsv">{{ t('logs.exportCsv') }}</el-button>
    </PageHeader>

    <section class="pos-panel logs-panel">
      <el-tabs v-model="activeTab" class="logs-tabs">
        <el-tab-pane :label="t('logs.audit')" name="audit">
          <div class="filters">
            <el-select
              v-model="auditFilters.category"
              class="filter-select"
              clearable
              :placeholder="t('logs.category')"
            >
              <el-option :label="t('common.all')" value="" />
              <el-option
                v-for="category in categories"
                :key="category"
                :label="categoryLabel(category)"
                :value="category"
              />
            </el-select>
            <el-input
              v-model="auditFilters.user"
              class="filter-user"
              clearable
              :prefix-icon="Search"
              :placeholder="t('logs.user')"
              @keyup.enter="applyFilters"
            />
            <el-date-picker
              v-model="auditFilters.range"
              class="filter-range"
              type="datetimerange"
              unlink-panels
              value-format="YYYY-MM-DDTHH:mm:ss"
              :start-placeholder="t('logs.from')"
              :end-placeholder="t('logs.to')"
              :default-time="defaultTime"
            />
            <el-button type="primary" :icon="Search" @click="applyFilters">{{ t('logs.applyFilter') }}</el-button>
            <el-button :icon="RefreshLeft" @click="resetFilters">{{ t('logs.reset') }}</el-button>
          </div>

          <el-table v-loading="auditLoading" :data="auditRows" row-key="id" max-height="540">
            <el-table-column :label="t('logs.time')" width="174">
              <template #default="{ row }">
                <span class="time-cell">{{ formatDate(row.created_at, 'YYYY-MM-DD HH:mm:ss') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logs.user')" width="132" show-overflow-tooltip>
              <template #default="{ row }">{{ row.username || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('logs.category')" width="112">
              <template #default="{ row }">
                <el-tag size="small" effect="plain" :type="categoryType(row.category)">
                  {{ categoryLabel(row.category) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('logs.action')" min-width="164" show-overflow-tooltip>
              <template #default="{ row }"><span class="pos-mono">{{ row.action || '—' }}</span></template>
            </el-table-column>
            <el-table-column :label="t('logs.target')" min-width="168" show-overflow-tooltip>
              <template #default="{ row }">{{ row.target || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('logs.detail')" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">{{ row.detail || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('logs.ip')" width="136">
              <template #default="{ row }"><span class="pos-mono">{{ row.ip || '—' }}</span></template>
            </el-table-column>
            <el-table-column :label="t('logs.result')" width="104">
              <template #default="{ row }">
                <el-tag size="small" effect="plain" :type="resultType(row.result)">
                  {{ resultLabel(row.result) }}
                </el-tag>
              </template>
            </el-table-column>
            <template #empty>
              <EmptyState :title="t('logs.noLogs')" icon="Document" />
            </template>
          </el-table>

          <div class="table-foot">
            <el-pagination
              v-model:current-page="auditPage"
              v-model:page-size="auditPageSize"
              background
              layout="total, sizes, prev, pager, next"
              :total="auditTotal"
              :page-sizes="[20, 50, 100]"
              @current-change="loadAudit"
              @size-change="onAuditSizeChange"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane :label="t('logs.loginLog')" name="login">
          <div class="tab-toolbar">
            <span class="pos-hint">{{ t('common.total') }} {{ loginRows.length }} {{ t('common.items') }}</span>
            <div class="pos-spacer" />
            <el-button size="small" :icon="Refresh" :loading="loginLoading" @click="loadLogin">
              {{ t('common.refresh') }}
            </el-button>
          </div>
          <el-table v-loading="loginLoading" :data="loginRows" row-key="id" max-height="540">
            <el-table-column :label="t('logs.time')" width="174">
              <template #default="{ row }">
                <span class="time-cell">{{ formatDate(row.created_at, 'YYYY-MM-DD HH:mm:ss') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logs.user')" width="170" show-overflow-tooltip>
              <template #default="{ row }">{{ row.username || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('logs.ip')" width="150">
              <template #default="{ row }"><span class="pos-mono">{{ row.ip || '—' }}</span></template>
            </el-table-column>
            <el-table-column :label="t('logs.result')" width="132">
              <template #default="{ row }">
                <el-tag size="small" effect="plain" :type="row.success ? 'success' : 'danger'">
                  {{ row.success ? t('logs.loginSuccess') : t('logs.loginFailed') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('logs.detail')" min-width="240" show-overflow-tooltip>
              <template #default="{ row }">{{ row.reason || '—' }}</template>
            </el-table-column>
            <template #empty>
              <EmptyState :title="t('logs.noLogs')" icon="User" />
            </template>
          </el-table>
        </el-tab-pane>

        <el-tab-pane :label="t('logs.systemLog')" name="system">
          <div v-loading="systemLoading" class="system-body">
            <div class="tab-toolbar">
              <span class="pos-hint">{{ t('common.total') }} {{ systemEvents.length }} {{ t('common.items') }}</span>
              <div class="pos-spacer" />
              <el-button size="small" :icon="Refresh" :loading="systemLoading" @click="loadSystem">
                {{ t('common.refresh') }}
              </el-button>
            </div>

            <el-table :data="systemEvents" row-key="id" max-height="340">
              <el-table-column :label="t('logs.time')" width="174">
                <template #default="{ row }">
                  <span class="time-cell">{{ formatDate(row.created_at, 'YYYY-MM-DD HH:mm:ss') }}</span>
                </template>
              </el-table-column>
              <el-table-column :label="t('logs.user')" width="132" show-overflow-tooltip>
                <template #default="{ row }">{{ row.username || '—' }}</template>
              </el-table-column>
              <el-table-column :label="t('logs.category')" width="112">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain" :type="categoryType(row.category)">
                    {{ categoryLabel(row.category) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column :label="t('logs.action')" min-width="160" show-overflow-tooltip>
                <template #default="{ row }"><span class="pos-mono">{{ row.action || '—' }}</span></template>
              </el-table-column>
              <el-table-column :label="t('logs.target')" min-width="168" show-overflow-tooltip>
                <template #default="{ row }">{{ row.target || '—' }}</template>
              </el-table-column>
              <el-table-column :label="t('logs.detail')" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">{{ row.detail || '—' }}</template>
              </el-table-column>
              <el-table-column :label="t('logs.result')" width="104">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain" :type="resultType(row.result)">
                    {{ resultLabel(row.result) }}
                  </el-tag>
                </template>
              </el-table-column>
              <template #empty>
                <EmptyState :title="t('logs.noLogs')" icon="Document" />
              </template>
            </el-table>

            <div class="journal-head">
              <span class="pos-card-title">{{ t('logs.journal') }}</span>
              <el-tag v-if="journalLines.length" size="small" effect="plain">{{ journalLines.length }}</el-tag>
            </div>
            <pre v-if="journalLines.length" class="journal">{{ journalText }}</pre>
            <div v-else class="journal-empty pos-hint">{{ t('logs.noLogs') }}</div>
          </div>
        </el-tab-pane>

        <el-tab-pane v-if="auth.pro" :label="t('logs.destroyLog')" name="destroy">
          <div class="tab-toolbar">
            <span class="pos-hint">{{ t('common.total') }} {{ destroyRows.length }} {{ t('common.items') }}</span>
            <div class="pos-spacer" />
            <el-button size="small" :icon="Refresh" :loading="destroyLoading" @click="loadDestroy">
              {{ t('common.refresh') }}
            </el-button>
          </div>
          <el-table v-loading="destroyLoading" :data="destroyRows" row-key="id" max-height="540">
            <el-table-column :label="t('logs.time')" width="174">
              <template #default="{ row }">
                <span class="time-cell">{{ formatDate(row.created_at, 'YYYY-MM-DD HH:mm:ss') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logs.user')" width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ row.username || '—' }}</template>
            </el-table-column>
            <el-table-column :label="t('logs.trigger')" min-width="150" show-overflow-tooltip>
              <template #default="{ row }"><span class="pos-mono">{{ row.trigger || '—' }}</span></template>
            </el-table-column>
            <el-table-column :label="t('logs.method')" min-width="150" show-overflow-tooltip>
              <template #default="{ row }"><span class="pos-mono">{{ row.method || '—' }}</span></template>
            </el-table-column>
            <el-table-column :label="t('common.status')" width="110">
              <template #default="{ row }">
                <el-tag size="small" effect="plain" :type="statusType(row.status)">
                  {{ statusLabel(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('logs.detail')" min-width="260" show-overflow-tooltip>
              <template #default="{ row }">{{ row.detail || '—' }}</template>
            </el-table-column>
            <template #empty>
              <EmptyState :title="t('logs.destroyEmpty')" icon="Delete" />
            </template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Download, Refresh, RefreshLeft, Search } from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import { api, apiURL, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { formatDate } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()

const activeTab = ref('audit')
const loadedTabs = reactive({ login: false, system: false, destroy: false })

const categories = [
  'auth', 'file', 'user', 'system', 'photo', 'movie',
  'docker', 'storage', 'network', 'app', 'destroy'
]
const CATEGORY_KEYS = {
  auth: 'logs.categoryAuth',
  file: 'logs.categoryFile',
  user: 'logs.categoryUser',
  system: 'logs.categorySystem',
  photo: 'logs.categoryPhoto',
  movie: 'logs.categoryMovie',
  docker: 'logs.categoryDocker',
  storage: 'logs.categoryStorage',
  network: 'logs.categoryNetwork',
  app: 'logs.categoryApp',
  destroy: 'logs.categoryDestroy'
}
const CATEGORY_TYPES = {
  auth: 'warning',
  file: 'success',
  user: 'primary',
  system: 'info',
  photo: 'success',
  movie: 'warning',
  docker: 'primary',
  storage: 'warning',
  network: 'primary',
  app: 'info',
  destroy: 'danger'
}

const auditFilters = reactive({ category: '', user: '', range: [] })
const auditRows = ref([])
const auditLoading = ref(false)
const auditTotal = ref(0)
const auditPage = ref(1)
const auditPageSize = ref(50)

const loginRows = ref([])
const loginLoading = ref(false)

const systemEvents = ref([])
const journalLines = ref([])
const systemLoading = ref(false)

const destroyRows = ref([])
const destroyLoading = ref(false)

const defaultTime = [new Date(2000, 1, 1, 0, 0, 0), new Date(2000, 1, 1, 23, 59, 59)]

const journalText = computed(() => journalLines.value.join('\n'))

function categoryLabel(value) {
  if (!value) return '—'
  const key = CATEGORY_KEYS[String(value).toLowerCase()]
  return key ? t(key) : String(value)
}

function categoryType(value) {
  return CATEGORY_TYPES[String(value || '').toLowerCase()] || 'info'
}

function isOk(value) {
  const result = String(value || 'ok').toLowerCase()
  return ['ok', 'success', 'done', 'completed', 'true', '1'].includes(result)
}

function resultType(value) {
  return isOk(value) ? 'success' : 'danger'
}

function resultLabel(value) {
  return isOk(value) ? t('logs.success') : t('logs.failed')
}

function statusType(value) {
  const status = String(value || '').toLowerCase()
  if (['ok', 'success', 'done', 'completed'].includes(status)) return 'success'
  if (['failed', 'fail', 'error', 'denied'].includes(status)) return 'danger'
  return 'info'
}

function statusLabel(value) {
  const status = String(value || '').toLowerCase()
  if (['ok', 'success', 'done', 'completed'].includes(status)) return t('logs.success')
  if (['failed', 'fail', 'error', 'denied'].includes(status)) return t('logs.failed')
  if (status === 'running') return t('common.running')
  return value || '—'
}

function filterParams() {
  const params = {}
  if (auditFilters.category) params.category = auditFilters.category
  const user = auditFilters.user.trim()
  if (user) params.user = user
  if (Array.isArray(auditFilters.range) && auditFilters.range.length === 2) {
    params.from = auditFilters.range[0]
    params.to = auditFilters.range[1]
  }
  return params
}

async function loadAudit() {
  auditLoading.value = true
  try {
    const payload = await api.logs.audit({
      page: auditPage.value,
      page_size: auditPageSize.value,
      ...filterParams()
    })
    const rows = Array.isArray(payload?.data) ? payload.data : []
    auditRows.value = rows
    auditTotal.value = Number(payload?.pagination?.total ?? rows.length)
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    auditLoading.value = false
  }
}

function applyFilters() {
  auditPage.value = 1
  loadAudit()
}

function resetFilters() {
  auditFilters.category = ''
  auditFilters.user = ''
  auditFilters.range = []
  auditPage.value = 1
  loadAudit()
}

function onAuditSizeChange() {
  auditPage.value = 1
  loadAudit()
}

async function loadLogin() {
  loginLoading.value = true
  try {
    const rows = await api.logs.login({ limit: 200 })
    loginRows.value = Array.isArray(rows) ? rows : []
    loadedTabs.login = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loginLoading.value = false
  }
}

async function loadSystem() {
  systemLoading.value = true
  try {
    const payload = await api.logs.system()
    systemEvents.value = Array.isArray(payload?.events) ? payload.events : []
    const journal = payload?.journal
    if (Array.isArray(journal)) {
      journalLines.value = journal
    } else if (typeof journal === 'string' && journal.trim()) {
      journalLines.value = journal.split('\n')
    } else {
      journalLines.value = []
    }
    loadedTabs.system = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    systemLoading.value = false
  }
}

async function loadDestroy() {
  destroyLoading.value = true
  try {
    const rows = await api.logs.destroy({ limit: 100 })
    destroyRows.value = Array.isArray(rows) ? rows : []
    loadedTabs.destroy = true
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    destroyLoading.value = false
  }
}

function ensureTabLoaded(tab) {
  if (tab === 'login' && !loadedTabs.login) loadLogin()
  else if (tab === 'system' && !loadedTabs.system) loadSystem()
  else if (tab === 'destroy' && !loadedTabs.destroy) loadDestroy()
}

function exportCsv() {
  const query = new URLSearchParams(filterParams()).toString()
  window.open(apiURL(`/logs/audit/export${query ? `?${query}` : ''}`), '_blank')
}

watch(activeTab, (tab) => ensureTabLoaded(tab))
onMounted(() => loadAudit())
</script>

<style scoped>
.logs-page {
  gap: 16px;
}
.logs-panel {
  padding: 10px 20px 20px;
}
.logs-tabs :deep(.el-tabs__header) {
  margin-bottom: 16px;
}
.logs-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background-color: var(--pos-border);
}
.logs-tabs :deep(.el-tabs__item) {
  font-size: 13px;
}
.filters {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}
.filter-select {
  width: 156px;
}
.filter-user {
  width: 200px;
}
.filter-range {
  width: 366px;
}
.tab-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.table-foot {
  display: flex;
  justify-content: flex-end;
}
.time-cell {
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.system-body {
  min-height: 180px;
}
.journal-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 20px 0 10px;
}
.journal {
  margin: 0;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--pos-border);
  background: var(--pos-bg-soft);
  color: var(--pos-muted);
  font-family: 'SF Mono', 'JetBrains Mono', Menlo, Consolas, monospace;
  font-size: 11.5px;
  line-height: 1.7;
  max-height: 260px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
.journal-empty {
  padding: 22px;
  border: 1px dashed var(--pos-border);
  border-radius: 10px;
  text-align: center;
}
.retention-hint {
  white-space: nowrap;
}
.logs-panel :deep(.empty-state) {
  padding: 34px 16px;
}

@media (max-width: 900px) {
  .filter-range {
    width: 100%;
  }
  .filter-select,
  .filter-user {
    flex: 1;
    min-width: 150px;
    width: auto;
  }
}
@media (max-width: 640px) {
  .logs-panel {
    padding: 8px 12px 14px;
  }
  .retention-hint {
    display: none;
  }
}
</style>
