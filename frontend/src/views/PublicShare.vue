<template>
  <div class="share-page">
    <div class="orb orb-a" />
    <div class="orb orb-b" />

    <div class="share-shell">
      <header class="share-brand">
        <img src="/logo.svg" :alt="t('app.name')" />
        <div class="brand-text">
          <strong>{{ t('app.name') }}</strong>
          <span>{{ t('share.title') }}</span>
        </div>
      </header>

      <section class="pos-panel share-card">
        <div v-if="loading" class="share-loading">
          <el-icon class="is-loading" :size="22"><Loading /></el-icon>
          <span class="pos-hint">{{ t('share.loading') }}</span>
        </div>

        <div v-else-if="errorState" class="share-empty">
          <EmptyState :title="errorTitle" :icon="errorIcon">
            <el-button v-if="errorState === 'network' || errorState === 'error'" :icon="Refresh" @click="load">
              {{ t('common.retry') }}
            </el-button>
            <el-button v-else :icon="HomeFilled" @click="goHome">{{ t('common.back') }}</el-button>
          </EmptyState>
        </div>

        <div v-else-if="needsPassword" class="gate">
          <div class="gate-icon">
            <el-icon :size="28"><Lock /></el-icon>
          </div>
          <h2>{{ t('share.passwordTitle') }}</h2>
          <p class="pos-hint gate-hint">{{ t('share.passwordHint') }}</p>
          <el-input
            v-model="password"
            type="password"
            show-password
            size="large"
            autocomplete="current-password"
            :placeholder="t('login.passwordPlaceholder')"
            :prefix-icon="Lock"
            @input="wrongPassword = false"
            @keyup.enter="verify"
          />
          <p v-if="wrongPassword" class="gate-error">{{ t('share.wrongPassword') }}</p>
          <el-button type="primary" size="large" class="gate-btn" :loading="verifying" @click="verify">
            {{ t('common.confirm') }}
          </el-button>
        </div>

        <div v-else-if="share" class="file">
          <div class="file-icon" :class="{ folder: isFolder }">
            <el-icon :size="36">
              <FolderOpened v-if="isFolder" />
              <Document v-else />
            </el-icon>
          </div>
          <h1 class="file-name">{{ share.name }}</h1>
          <div class="file-tags">
            <el-tag size="small" effect="plain">
              {{ isFolder ? t('share.folder') : t('share.file') }}
            </el-tag>
          </div>

          <dl class="share-stats">
            <div class="stat">
              <dt>{{ t('share.size') }}</dt>
              <dd>{{ formatBytes(share.size || 0) }}</dd>
            </div>
            <div class="stat">
              <dt>{{ t('share.downloads') }}</dt>
              <dd>{{ downloadsText }}</dd>
            </div>
            <div class="stat">
              <dt>{{ t('share.expires') }}</dt>
              <dd>{{ expiresText }}</dd>
            </div>
          </dl>

          <el-button type="primary" size="large" class="download-btn" :icon="Download" @click="download">
            {{ t('share.download') }}
          </el-button>
        </div>
      </section>

      <footer class="share-foot">
        <span>{{ t('share.poweredBy') }}</span>
        <span class="sep">·</span>
        <span>{{ t('app.version') }} v{{ brand.version }}</span>
        <span class="sep">·</span>
        <a :href="websiteHref" target="_blank" rel="noreferrer">{{ brand.website }}</a>
      </footer>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Download, Lock, Loading, Refresh, HomeFilled, FolderOpened, Document } from '@element-plus/icons-vue'
import EmptyState from '../components/EmptyState.vue'
import { api, errorMessage } from '../api'
import { useUiStore } from '../stores/ui'
import { formatBytes, formatDate } from '../utils/format'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const ui = useUiStore()

const token = String(route.params.token || '')
const share = ref(null)
const brand = ref({ name: 'PrivacyOS', version: '1.2.1', website: 'privacyos.local' })
const loading = ref(true)
const errorState = ref('')
const authorized = ref(false)
const password = ref('')
const verifying = ref(false)
const wrongPassword = ref(false)

const isFolder = computed(() => share.value?.kind === 'folder' || share.value?.kind === 'dir')
const needsPassword = computed(() => !!share.value && !!share.value.has_password && !authorized.value)

const errorTitle = computed(() => {
  switch (errorState.value) {
    case 'invalid':
      return t('share.invalid')
    case 'expired':
      return t('share.expired')
    case 'limit':
      return t('share.limitReached')
    case 'network':
      return t('errors.network')
    default:
      return t('errors.unknown')
  }
})

const errorIcon = computed(() => {
  const icons = { invalid: 'Link', expired: 'Timer', limit: 'Download', network: 'Connection' }
  return icons[errorState.value] || 'WarningFilled'
})

const downloadsText = computed(() => {
  const current = Number(share.value?.downloads || 0)
  const max = Number(share.value?.max_downloads || 0)
  return max > 0 ? `${current} / ${max}` : `${current} · ${t('share.unlimited')}`
})

const expiresText = computed(() => (share.value?.expires_at ? formatDate(share.value.expires_at) : t('share.unlimited')))

const websiteHref = computed(() => {
  const site = brand.value.website || ''
  if (!site) return '#'
  return /^https?:\/\//i.test(site) ? site : `https://${site}`
})

function goHome() {
  router.push('/')
}

function download() {
  window.location.assign(api.shares.downloadURL(token))
}

async function load() {
  loading.value = true
  errorState.value = ''
  wrongPassword.value = false
  try {
    const payload = await api.shares.publicInfo(token)
    share.value = payload
    authorized.value = !!payload.authorized
    document.title = `${payload.name} · ${t('share.title')}`
  } catch (error) {
    share.value = null
    const status = error?.response?.status
    const code = error?.response?.data?.error?.code
    if (code === 'limit_reached') {
      errorState.value = 'limit'
    } else if (code === 'expired' || status === 410) {
      errorState.value = 'expired'
    } else if (status === 404) {
      errorState.value = 'invalid'
    } else if (!error?.response) {
      errorState.value = 'network'
      ElMessage.error(errorMessage(error))
    } else {
      errorState.value = 'error'
      ElMessage.error(errorMessage(error))
    }
    document.title = `${t('share.title')} · ${t('app.name')}`
  } finally {
    loading.value = false
  }
}

async function loadBrand() {
  try {
    const info = await api.public.info()
    brand.value = {
      name: info.name || 'PrivacyOS',
      version: info.version || '1.2.1',
      website: info.website || 'privacyos.local'
    }
  } catch (error) {
    // Branding information is optional for the public share page.
  }
}

async function verify() {
  if (verifying.value) return
  if (!password.value) {
    ElMessage.warning(t('share.passwordRequired'))
    return
  }
  verifying.value = true
  wrongPassword.value = false
  try {
    const payload = await api.shares.verify(token, password.value)
    authorized.value = true
    password.value = ''
    ElMessage.success(t('common.operationSuccess'))
    window.location.assign(payload?.download_url || api.shares.downloadURL(token))
  } catch (error) {
    const status = error?.response?.status
    const code = error?.response?.data?.error?.code
    if (status === 401 || code === 'invalid_password') {
      wrongPassword.value = true
      ElMessage.error(t('share.wrongPassword'))
    } else if (code === 'limit_reached') {
      errorState.value = 'limit'
      ElMessage.error(errorMessage(error))
    } else if (code === 'expired' || status === 410) {
      errorState.value = 'expired'
      ElMessage.error(errorMessage(error))
    } else {
      ElMessage.error(errorMessage(error))
    }
  } finally {
    verifying.value = false
  }
}

onMounted(() => {
  ui.applyTheme()
  loadBrand()
  load()
})
</script>

<style scoped>
.share-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 36px 20px 28px;
  overflow: hidden;
  background:
    radial-gradient(900px 520px at 50% -10%, rgba(45, 212, 191, 0.14), transparent 62%),
    radial-gradient(760px 520px at 100% 100%, rgba(96, 165, 250, 0.1), transparent 60%),
    var(--pos-bg);
}
.orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.5;
  pointer-events: none;
}
.orb-a {
  width: 420px;
  height: 420px;
  background: rgba(45, 212, 191, 0.16);
  top: -160px;
  right: -100px;
}
.orb-b {
  width: 360px;
  height: 360px;
  background: rgba(96, 165, 250, 0.12);
  bottom: -170px;
  left: -110px;
}
.share-shell {
  position: relative;
  z-index: 2;
  width: 100%;
  max-width: 520px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 20px;
}
.share-brand {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}
.share-brand img {
  width: 42px;
  height: 42px;
}
.brand-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.brand-text strong {
  font-size: 18px;
  letter-spacing: 0.8px;
}
.brand-text span {
  font-size: 11.5px;
  color: var(--pos-muted);
}
.share-card {
  position: relative;
  padding: 32px 30px;
  min-height: 300px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  overflow: hidden;
}
.share-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--pos-accent), transparent);
  opacity: 0.7;
}
.share-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 240px;
  color: var(--pos-muted);
}
.share-empty {
  padding: 24px 4px;
}
.share-empty :deep(.empty-state) {
  padding: 22px 4px;
}
.gate {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  text-align: center;
}
.gate-icon {
  width: 64px;
  height: 64px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
  margin-bottom: 4px;
}
.gate h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 650;
}
.gate-hint {
  max-width: 320px;
  margin-bottom: 6px;
}
.gate :deep(.el-input) {
  margin-top: 4px;
}
.gate :deep(.el-input__wrapper) {
  background: var(--pos-panel-2);
}
.gate-btn {
  width: 100%;
  height: 44px;
  margin-top: 8px;
}
.gate-error {
  margin: 0;
  color: var(--pos-danger);
  font-size: 12px;
}
.file {
  display: flex;
  flex-direction: column;
  align-items: stretch;
}
.file-icon {
  width: 76px;
  height: 76px;
  border-radius: 22px;
  margin: 0 auto 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--pos-accent);
  background: var(--pos-accent-soft);
  border: 1px solid rgba(45, 212, 191, 0.22);
}
.file-icon.folder {
  color: #60a5fa;
  background: rgba(96, 165, 250, 0.12);
  border-color: rgba(96, 165, 250, 0.22);
}
.file-name {
  margin: 0;
  text-align: center;
  font-size: 19px;
  font-weight: 650;
  line-height: 1.45;
  word-break: break-word;
}
.file-tags {
  display: flex;
  justify-content: center;
  gap: 8px;
  margin-top: 10px;
}
.share-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin: 22px 0;
}
.stat {
  padding: 11px 12px;
  border-radius: 11px;
  border: 1px solid var(--pos-border);
  background: var(--pos-panel-2);
  display: flex;
  flex-direction: column;
  gap: 4px;
  text-align: center;
}
.stat dt {
  font-size: 10.5px;
  letter-spacing: 0.3px;
  color: var(--pos-muted);
}
.stat dd {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  word-break: break-word;
}
.download-btn {
  width: 100%;
  height: 46px;
  font-size: 15px;
  letter-spacing: 0.5px;
}
.share-foot {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  flex-wrap: wrap;
  color: var(--pos-muted);
  font-size: 11.5px;
  text-align: center;
}
.share-foot a {
  color: var(--pos-muted);
  transition: color 0.16s ease;
}
.share-foot a:hover {
  color: var(--pos-accent);
}
.sep {
  opacity: 0.5;
}

@media (max-width: 560px) {
  .share-page {
    align-items: flex-start;
    padding: 26px 16px 20px;
  }
  .share-card {
    padding: 24px 18px;
  }
  .file-name {
    font-size: 17px;
  }
  .share-stats {
    grid-template-columns: 1fr;
  }
}
</style>
