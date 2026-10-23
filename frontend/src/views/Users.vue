<template>
  <div class="pos-page users-page">
    <PageHeader :title="t('users.title')" :subtitle="t('users.subtitle')">
      <el-tag effect="plain" :type="auth.pro ? 'success' : 'info'">
        {{ auth.pro ? t('app.pro') : t('app.normal') }}
      </el-tag>
      <el-button type="primary" :icon="Plus" @click="openCreate">{{ t('users.addUser') }}</el-button>
      <el-button :icon="Refresh" :loading="loading" @click="reload">{{ t('common.refresh') }}</el-button>
    </PageHeader>

    <section class="pos-panel users-panel">
      <div class="panel-toolbar">
        <el-input
          v-model="searchQuery"
          class="search"
          clearable
          :prefix-icon="Search"
          :placeholder="t('common.searchPlaceholder')"
        />
        <div class="pos-spacer" />
        <span class="pos-hint">{{ t('common.total') }} {{ filteredUsers.length }}</span>
      </div>

      <el-table
        v-if="users.length || loading"
        v-loading="loading"
        :data="filteredUsers"
        row-key="id"
        class="users-table"
      >
        <el-table-column :label="t('users.username')" min-width="210">
          <template #default="{ row }">
            <div class="user-cell">
              <span class="avatar" :class="{ admin: row.role === 'admin' }">{{ avatarText(row) }}</span>
              <span class="user-name">{{ row.username }}</span>
              <el-tag v-if="isSelf(row)" size="small" type="success" effect="plain">{{ t('users.self') }}</el-tag>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('users.displayName')" min-width="150">
          <template #default="{ row }">{{ row.display_name || '—' }}</template>
        </el-table-column>

        <el-table-column :label="t('users.role')" width="108">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="row.role === 'admin' ? 'warning' : 'info'">
              {{ row.role === 'admin' ? t('users.admin') : t('users.user') }}
            </el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('users.status')" width="120">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="statusType(row)">{{ statusLabel(row) }}</el-tag>
          </template>
        </el-table-column>

        <el-table-column :label="t('users.quota')" width="150">
          <template #default="{ row }">
            <el-tooltip v-if="row.quota_hidden" :content="t('users.quotaHidden')" placement="top">
              <span class="quota-hidden">—</span>
            </el-tooltip>
            <span v-else-if="!row.quota_bytes" class="pos-hint">{{ t('users.quotaUnlimited') }}</span>
            <span v-else class="num">{{ formatBytes(row.quota_bytes) }}</span>
          </template>
        </el-table-column>

        <el-table-column :label="t('users.lastLogin')" width="170">
          <template #default="{ row }">
            <span class="last-login">
              {{ row.last_login_at ? formatDate(row.last_login_at) : t('users.neverLoggedIn') }}
            </span>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.actions')" width="90" align="right">
          <template #default="{ row }">
            <el-dropdown trigger="click" @command="(command) => handleCommand(command, row)">
              <el-button link :icon="MoreFilled" class="row-more" />
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit" :icon="Edit">{{ t('common.edit') }}</el-dropdown-item>
                  <el-dropdown-item
                    command="password"
                    :icon="Key"
                    :disabled="isSelf(row)"
                    :title="isSelf(row) ? t('users.self') : undefined"
                  >
                    {{ t('users.resetPassword') }}
                  </el-dropdown-item>
                  <el-dropdown-item command="unlock" :icon="Unlock" :disabled="!isLocked(row)">
                    {{ t('users.unlock') }}
                  </el-dropdown-item>
                  <el-dropdown-item
                    command="usage"
                    :icon="DataAnalysis"
                    :disabled="usageDisabled(row)"
                    :title="usageDisabled(row) ? t('users.quotaHidden') : undefined"
                  >
                    {{ t('users.quota') }}
                  </el-dropdown-item>
                  <el-dropdown-item
                    command="delete"
                    :icon="Delete"
                    divided
                    :disabled="deleteDisabled(row)"
                    :title="deleteDisabledHint(row)"
                  >
                    {{ t('users.delete') }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>

        <template #empty>
          <div class="pos-hint table-empty">{{ t('common.empty') }}</div>
        </template>
      </el-table>

      <div v-if="!users.length && !loading" class="empty-wrap">
        <EmptyState :title="t('common.empty')" :description="t('users.subtitle')" icon="User">
          <el-button type="primary" :icon="Plus" @click="openCreate">{{ t('users.addUser') }}</el-button>
        </EmptyState>
      </div>
    </section>

    <section v-if="auth.pro" class="pos-panel deleted-panel">
      <div class="panel-head">
        <div>
          <div class="pos-card-title">{{ t('users.deletedUsers') }}</div>
          <div class="pos-hint">{{ t('users.deleteProHint') }}</div>
        </div>
        <div class="pos-spacer" />
        <el-button size="small" :icon="Refresh" :loading="deletedLoading" @click="loadDeleted">
          {{ t('common.refresh') }}
        </el-button>
      </div>

      <el-table
        v-if="deletedUsers.length || deletedLoading"
        v-loading="deletedLoading"
        :data="deletedUsers"
        row-key="id"
        class="users-table"
      >
        <el-table-column :label="t('users.username')" min-width="190">
          <template #default="{ row }">
            <div class="user-cell">
              <span class="avatar">{{ avatarText(row) }}</span>
              <span class="user-name">{{ row.username }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('users.displayName')" min-width="150">
          <template #default="{ row }">{{ row.display_name || '—' }}</template>
        </el-table-column>

        <el-table-column :label="t('users.status')" min-width="250">
          <template #default="{ row }">
            <div class="cooling-cell">
              <el-tag size="small" type="warning" effect="plain">{{ t('users.pendingDeletion') }}</el-tag>
              <span class="pos-hint" :title="formatDate(row.delete_after)">{{ coolingText(row) }}</span>
            </div>
          </template>
        </el-table-column>

        <el-table-column :label="t('common.actions')" width="130" align="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              size="small"
              :icon="RefreshLeft"
              :loading="restoringId === row.id"
              @click="restoreUser(row)"
            >
              {{ t('users.restore') }}
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <EmptyState v-else :title="t('users.noDeleted')" :description="t('users.deleteProHint')" icon="Delete" />
    </section>

    <!-- usage drawer -->
    <el-drawer v-model="usageOpen" :title="usageTitle" size="min(440px, 94vw)">
      <div v-loading="usageLoading" class="usage-drawer">
        <template v-if="usage">
          <div class="usage-user">
            <span class="avatar" :class="{ admin: usageTarget?.role === 'admin' }">{{ avatarText(usageTarget) }}</span>
            <div class="usage-identity">
              <strong>{{ usage.username }}</strong>
              <span class="pos-hint">{{ usageTarget?.display_name || '—' }}</span>
            </div>
          </div>

          <UsageBar
            :label="t('users.quota')"
            :used="usage.usage?.used_bytes || 0"
            :quota="usage.usage?.quota_bytes || 0"
          />

          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('common.items')">
              {{ usage.usage?.file_count ?? 0 }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('files.trash')">
              {{ formatBytes(usage.usage?.trash_bytes || 0) }}
            </el-descriptions-item>
            <el-descriptions-item :label="t('users.lastLogin')">
              {{ usageTarget?.last_login_at ? formatDate(usageTarget.last_login_at) : t('users.neverLoggedIn') }}
            </el-descriptions-item>
          </el-descriptions>
        </template>
        <EmptyState v-else-if="!usageLoading" :title="t('common.empty')" icon="DataAnalysis" />
      </div>
    </el-drawer>

    <!-- add / edit user -->
    <el-dialog
      v-model="dialogOpen"
      :title="editing ? t('users.editUser') : t('users.addUser')"
      width="min(620px, 94vw)"
      @closed="onDialogClosed"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-position="top" class="user-form">
        <div class="form-grid">
          <el-form-item :label="t('users.username')" prop="username">
            <div class="field-stack">
              <el-input
                v-model="form.username"
                :disabled="!!editing"
                maxlength="32"
                autocomplete="off"
                :placeholder="t('users.usernameHint')"
              />
              <div v-if="!editing" class="pos-hint">{{ t('users.usernameHint') }}</div>
            </div>
          </el-form-item>

          <el-form-item :label="t('users.displayName')">
            <el-input v-model="form.display_name" maxlength="64" :placeholder="t('users.displayName')" />
          </el-form-item>

          <el-form-item v-if="!editing" :label="t('users.newPassword')" prop="password">
            <div class="field-stack">
              <el-input
                v-model="form.password"
                type="password"
                show-password
                autocomplete="new-password"
                :placeholder="t('users.passwordHint')"
              />
              <div class="pos-hint">{{ t('users.passwordHint') }}</div>
            </div>
          </el-form-item>

          <el-form-item :label="t('users.role')">
            <div class="field-stack">
              <el-select v-model="form.role" class="full" :disabled="isEditingSelf" @change="onRoleChange">
                <el-option :label="t('users.user')" value="user" :disabled="isEditingLastAdmin" />
                <el-option :label="t('users.admin')" value="admin" />
              </el-select>
              <div v-if="isEditingLastAdmin" class="pos-hint">{{ t('users.lastAdmin') }}</div>
            </div>
          </el-form-item>

          <el-form-item v-if="editing" :label="t('users.status')">
            <el-select v-model="form.status" class="full" :disabled="isEditingSelf">
              <el-option :label="t('users.active')" value="active" />
              <el-option :label="t('users.disabled')" value="disabled" />
            </el-select>
          </el-form-item>

          <el-form-item :label="t('users.quota')">
            <div class="field-stack">
              <div class="quota-row">
                <el-input-number
                  v-model="form.quotaGb"
                  class="quota-input"
                  :min="0"
                  :max="1048576"
                  :step="1"
                  :precision="2"
                  :disabled="quotaHidden"
                  controls-position="right"
                />
                <span class="quota-unit">GB</span>
              </div>
              <div class="pos-hint">{{ quotaHidden ? t('users.quotaHidden') : t('users.quotaHint') }}</div>
            </div>
          </el-form-item>
        </div>

        <el-form-item :label="t('users.permissions')">
          <div class="perm-block">
            <el-checkbox-group v-model="permissionKeys" :disabled="form.role === 'admin'" class="perm-group">
              <el-checkbox v-for="option in permissionOptions" :key="option.key" :value="option.key">
                {{ t(option.labelKey) }}
              </el-checkbox>
            </el-checkbox-group>
            <div v-if="form.role === 'admin'" class="pos-hint">{{ t('users.permissionsHint') }}</div>
          </div>
        </el-form-item>
      </el-form>

      <template #footer>
        <el-button @click="dialogOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="submitForm">
          {{ editing ? t('common.save') : t('common.create') }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Refresh, Plus, Search, MoreFilled, Edit, Key, Unlock, DataAnalysis, Delete, RefreshLeft
} from '@element-plus/icons-vue'
import PageHeader from '../components/PageHeader.vue'
import EmptyState from '../components/EmptyState.vue'
import UsageBar from '../components/UsageBar.vue'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { formatBytes, formatDate } from '../utils/format'

const { t } = useI18n()
const auth = useAuthStore()

const BYTES_PER_GB = 1024 ** 3
const USERNAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9_-]{2,31}$/
const permissionOptions = [
  { key: 'files', labelKey: 'users.filesPerm' },
  { key: 'photos', labelKey: 'users.photosPerm' },
  { key: 'movies', labelKey: 'users.moviesPerm' },
  { key: 'docker', labelKey: 'users.dockerPerm' },
  { key: 'apps', labelKey: 'users.appsPerm' },
  { key: 'share', labelKey: 'users.sharePerm' }
]

const loading = ref(false)
const users = ref([])
const deletedUsers = ref([])
const deletedLoading = ref(false)
const searchQuery = ref('')
const restoringId = ref(null)
const dialogOpen = ref(false)
const saving = ref(false)
const editing = ref(null)
const formRef = ref(null)

const usageOpen = ref(false)
const usageLoading = ref(false)
const usageTarget = ref(null)
const usage = ref(null)

const form = reactive({
  username: '',
  display_name: '',
  password: '',
  role: 'user',
  status: 'active',
  quotaGb: 0,
  permissions: defaultPermissions()
})

const formRules = computed(() => ({
  username: editing.value
    ? []
    : [
        { required: true, message: t('common.required'), trigger: 'blur' },
        { pattern: USERNAME_RE, message: t('users.usernameHint'), trigger: 'blur' }
      ],
  password: editing.value
    ? []
    : [
        { required: true, message: t('common.required'), trigger: 'blur' },
        { min: 8, message: t('users.passwordHint'), trigger: 'blur' }
      ]
}))

const filteredUsers = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return users.value
  return users.value.filter((user) => `${user.username} ${user.display_name || ''}`.toLowerCase().includes(query))
})

const adminCount = computed(() => users.value.filter((user) => user.role === 'admin').length)
const isEditingSelf = computed(() => !!editing.value && isSelf(editing.value))
const isEditingLastAdmin = computed(
  () => !!editing.value && editing.value.role === 'admin' && adminCount.value <= 1
)
const quotaHidden = computed(() => !!editing.value?.quota_hidden)
const permissionKeys = computed({
  get() {
    return permissionOptions.filter((option) => form.permissions[option.key]).map((option) => option.key)
  },
  set(keys) {
    permissionOptions.forEach((option) => {
      form.permissions[option.key] = keys.includes(option.key)
    })
  }
})
const usageTitle = computed(() => `${t('users.quota')} · ${usageTarget.value?.username || ''}`)

function defaultPermissions() {
  return { files: true, photos: true, movies: true, share: true, docker: false, apps: false }
}

function allPermissions() {
  return { files: true, photos: true, movies: true, share: true, docker: true, apps: true }
}

function avatarText(row) {
  const name = row?.display_name || row?.username || '?'
  return name.trim().slice(0, 1).toUpperCase()
}

function isSelf(row) {
  if (!row) return false
  return row.id === auth.user?.id || row.username === auth.username
}

function isLastAdmin(row) {
  return row?.role === 'admin' && adminCount.value <= 1
}

function isLocked(row) {
  return !!row?.locked_until && new Date(row.locked_until).getTime() > Date.now()
}

function statusKey(row) {
  if (isLocked(row)) return 'locked'
  return row?.status || ''
}

function statusLabel(row) {
  switch (statusKey(row)) {
    case 'active':
      return t('users.active')
    case 'disabled':
      return t('users.disabled')
    case 'locked':
      return t('users.locked')
    case 'pending_deletion':
      return t('users.pendingDeletion')
    default:
      return row?.status || t('common.unknown')
  }
}

function statusType(row) {
  switch (statusKey(row)) {
    case 'active':
      return 'success'
    case 'disabled':
      return 'info'
    case 'locked':
      return 'danger'
    case 'pending_deletion':
      return 'warning'
    default:
      return 'info'
  }
}

function usageDisabled(row) {
  return auth.pro && !isSelf(row)
}

function deleteDisabled(row) {
  return isSelf(row) || isLastAdmin(row)
}

function deleteDisabledHint(row) {
  if (isSelf(row)) return t('users.self')
  if (isLastAdmin(row)) return t('users.lastAdmin')
  return undefined
}

function coolingDays(row) {
  if (!row?.delete_after) return 0
  const remaining = new Date(row.delete_after).getTime() - Date.now()
  return Math.max(0, Math.ceil(remaining / 86400000))
}

function coolingText(row) {
  return t('users.coolingDays', { n: coolingDays(row) })
}

async function loadUsers() {
  loading.value = true
  try {
    users.value = (await api.users.list()) || []
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadDeleted() {
  if (!auth.pro) return
  deletedLoading.value = true
  try {
    deletedUsers.value = (await api.users.deleted()) || []
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    deletedLoading.value = false
  }
}

async function reload() {
  await loadUsers()
  if (auth.pro) await loadDeleted()
}

function openCreate() {
  editing.value = null
  form.username = ''
  form.display_name = ''
  form.password = ''
  form.role = 'user'
  form.status = 'active'
  form.quotaGb = 0
  form.permissions = defaultPermissions()
  dialogOpen.value = true
  nextTick(() => formRef.value?.clearValidate())
}

function openEdit(row) {
  editing.value = row
  form.username = row.username
  form.display_name = row.display_name || ''
  form.password = ''
  form.role = row.role === 'admin' ? 'admin' : 'user'
  form.status = row.status === 'disabled' ? 'disabled' : 'active'
  form.quotaGb = row.quota_bytes ? Math.round((row.quota_bytes / BYTES_PER_GB) * 100) / 100 : 0
  const permissions = row.permissions || {}
  form.permissions = row.role === 'admin'
    ? allPermissions()
    : {
        files: !!permissions.files,
        photos: !!permissions.photos,
        movies: !!permissions.movies,
        share: !!permissions.share,
        docker: !!permissions.docker,
        apps: !!permissions.apps
      }
  dialogOpen.value = true
  nextTick(() => formRef.value?.clearValidate())
}

function onRoleChange(role) {
  form.permissions = role === 'admin' ? allPermissions() : defaultPermissions()
}

function onDialogClosed() {
  formRef.value?.clearValidate()
}

async function submitForm() {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch (error) {
    return
  }

  const username = form.username.trim()
  if (!editing.value && !USERNAME_RE.test(username)) {
    ElMessage.warning(t('users.usernameHint'))
    return
  }
  if (!editing.value && form.password.length < 8) {
    ElMessage.warning(t('users.passwordHint'))
    return
  }

  const payload = {
    display_name: form.display_name.trim(),
    role: form.role,
    permissions: { ...form.permissions }
  }
  if (!quotaHidden.value) {
    payload.quota_bytes = Math.round(Number(form.quotaGb || 0) * BYTES_PER_GB)
  }

  saving.value = true
  try {
    if (editing.value) {
      payload.status = form.status
      await api.users.update(editing.value.id, payload)
      ElMessage.success(t('users.updateSuccess'))
    } else {
      await api.users.create({ ...payload, username, password: form.password })
      ElMessage.success(t('users.addSuccess'))
    }
    dialogOpen.value = false
    await reload()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    saving.value = false
  }
}

function handleCommand(command, row) {
  if (command === 'edit') openEdit(row)
  else if (command === 'password') resetPassword(row)
  else if (command === 'unlock') unlockUser(row)
  else if (command === 'usage') openUsage(row)
  else if (command === 'delete') deleteUser(row)
}

async function resetPassword(row) {
  try {
    const { value } = await ElMessageBox.prompt(t('users.newPassword'), t('users.resetPassword'), {
      inputType: 'password',
      inputPlaceholder: t('users.passwordHint'),
      inputValidator: (input) => (!!input && input.length >= 8) || t('users.passwordHint'),
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel')
    })
    await api.users.resetPassword(row.id, value)
    ElMessage.success(t('users.passwordReset'))
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error))
  }
}

async function unlockUser(row) {
  try {
    await api.users.unlock(row.id)
    ElMessage.success(t('users.unlocked'))
    await loadUsers()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function openUsage(row) {
  usageTarget.value = row
  usage.value = null
  usageOpen.value = true
  usageLoading.value = true
  try {
    usage.value = await api.users.usage(row.id)
  } catch (error) {
    ElMessage.error(errorMessage(error))
    usageOpen.value = false
  } finally {
    usageLoading.value = false
  }
}

async function deleteUser(row) {
  if (deleteDisabled(row)) return
  try {
    await ElMessageBox.confirm(
      t('users.deleteConfirm', { name: row.username }),
      t('users.delete'),
      {
        type: 'warning',
        description: auth.pro ? t('users.deleteProHint') : t('users.deleteNormalHint'),
        confirmButtonText: t('common.delete'),
        cancelButtonText: t('common.cancel'),
        confirmButtonClass: 'el-button--danger'
      }
    )
  } catch (error) {
    return
  }
  try {
    await api.users.remove(row.id)
    ElMessage.success(t('users.deleteSuccess'))
    await reload()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function restoreUser(row) {
  if (restoringId.value) return
  restoringId.value = row.id
  try {
    await api.users.restore(row.id)
    ElMessage.success(t('users.restoreSuccess'))
    await reload()
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    restoringId.value = null
  }
}

onMounted(() => {
  loadUsers()
  if (auth.pro) loadDeleted()
})
</script>

<style scoped>
.users-page {
  min-height: 100%;
}
.users-panel,
.deleted-panel {
  padding: 16px 18px;
}
.panel-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
}
.search {
  width: 240px;
}
.users-table {
  width: 100%;
}
.user-cell {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}
.avatar {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  font-size: 12px;
  font-weight: 600;
  flex: none;
}
.avatar.admin {
  background: rgba(251, 191, 36, 0.14);
  color: var(--pos-warning);
}
.user-name {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.quota-hidden {
  color: var(--pos-muted);
  cursor: help;
}
.num {
  font-variant-numeric: tabular-nums;
  font-size: 12.5px;
}
.last-login {
  font-size: 12px;
  color: var(--pos-muted);
}
.row-more {
  padding: 4px;
}
.table-empty {
  padding: 34px 0;
}
.empty-wrap {
  padding: 6px 0;
}
.panel-head {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}
.panel-head .pos-hint {
  margin-top: 4px;
}
.cooling-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.usage-drawer {
  min-height: 220px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.usage-user {
  display: flex;
  align-items: center;
  gap: 10px;
}
.usage-identity {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}
.usage-identity strong {
  font-size: 14px;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}
.field-stack {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field-stack .pos-hint {
  line-height: 1.5;
  margin-top: -2px;
}
.full {
  width: 100%;
}
.quota-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.quota-input {
  flex: 1;
  width: 100%;
}
.quota-input :deep(.el-input__inner) {
  text-align: left;
}
.quota-unit {
  font-size: 12px;
  color: var(--pos-muted);
  flex: none;
}
.perm-block {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.perm-group {
  display: flex;
  flex-wrap: wrap;
  gap: 4px 18px;
}
.perm-group :deep(.el-checkbox) {
  margin-right: 0;
}

@media (max-width: 900px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
  .search {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .users-panel,
  .deleted-panel {
    padding: 12px;
  }
  .panel-head {
    flex-wrap: wrap;
  }
}
</style>
