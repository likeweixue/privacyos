<template>
  <div class="login-page">
    <section class="hero" :style="heroStyle">
      <div class="hero-inner">
        <div class="brand">
          <img src="/logo.svg" alt="PrivacyOS" />
          <div>
            <h1>PrivacyOS</h1>
            <p>{{ t('login.brandHint') }}</p>
          </div>
        </div>
        <ul class="features">
          <li><el-icon><Lock /></el-icon>{{ t('login.feature1') }}</li>
          <li><el-icon><Key /></el-icon>{{ t('login.feature2') }}</li>
          <li><el-icon><Share /></el-icon>{{ t('login.feature3') }}</li>
        </ul>
        <div class="hero-foot">
          <span class="chip">{{ info.dev ? 'DEV' : 'NAS' }}</span>
          <span>{{ t('login.footer') }}</span>
        </div>
      </div>
      <div class="orb orb-a" />
      <div class="orb orb-b" />
    </section>

    <section class="form-side">
      <div class="form-card">
        <div class="mobile-brand">
          <img src="/logo.svg" alt="PrivacyOS" />
          <strong>PrivacyOS</strong>
        </div>
        <h2>{{ t('login.title') }}</h2>
        <p class="sub">{{ info.name || 'PrivacyOS' }} · {{ editionLabel }}</p>

        <el-form ref="formRef" :model="form" label-position="top" @submit.prevent="submit">
          <el-form-item :label="t('login.username')">
            <el-input
              v-model="form.username"
              size="large"
              :placeholder="t('login.usernamePlaceholder')"
              autocomplete="username"
              :prefix-icon="User"
            />
          </el-form-item>
          <el-form-item :label="t('login.password')">
            <el-input
              v-model="form.password"
              size="large"
              type="password"
              show-password
              :placeholder="t('login.passwordPlaceholder')"
              autocomplete="current-password"
              :prefix-icon="Lock"
              @keyup.enter="submit"
            />
          </el-form-item>
          <el-form-item v-if="needTotp" :label="t('login.totp')">
            <el-input
              v-model="form.totp"
              size="large"
              maxlength="6"
              :placeholder="t('login.totpPlaceholder')"
              autocomplete="one-time-code"
              :prefix-icon="Key"
              @keyup.enter="submit"
            />
            <div class="hint">{{ t('login.totpHint') }}</div>
          </el-form-item>

          <div v-if="retryAfter > 0" class="lock-box">
            <el-icon><WarningFilled /></el-icon>
            <div>
              <strong>{{ t('login.locked') }}</strong>
              <span>{{ retryAfter }}s {{ t('login.retryIn') }}</span>
            </div>
          </div>

          <el-button
            type="primary"
            size="large"
            class="submit"
            :loading="auth.loading"
            :disabled="retryAfter > 0"
            native-type="submit"
          >
            {{ auth.loading ? t('login.submitting') : t('login.submit') }}
          </el-button>
        </el-form>

        <div v-if="info.dev" class="demo-tip">
          <span class="demo-badge">{{ t('login.demo') }}</span>
          {{ t('login.demoHint') }}
        </div>

        <footer class="login-foot">
          <router-link class="installer-link" :to="{ name: 'install' }">{{ t('installer.title') }}</router-link>
          <span class="dot">·</span>
          <span>v{{ info.version || '1.3.0' }}</span>
          <span class="dot">·</span>
          <a :href="info.website || '#'" target="_blank" rel="noreferrer">{{ info.website || 'privacyos' }}</a>
          <span class="spacer" />
          <span class="edition">{{ editionLabel }}</span>
        </footer>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Lock, Key, Share, User, WarningFilled } from '@element-plus/icons-vue'
import { api, errorMessage, errorPayload } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ui = useUiStore()

const info = ref({ name: 'PrivacyOS', version: '1.3.0', edition: 'normal', website: 'privacyos.local', dev: false })
const form = ref({ username: '', password: '', totp: '' })
const needTotp = ref(false)
const retryAfter = ref(0)
let timer = null

const heroStyle = computed(() => {
  const a = info.value.appearance || {}
  if (!a.login_wallpaper || !a.wallpaper_url) return {}
  return {
    backgroundImage: `linear-gradient(160deg, rgba(5,12,18,.72), rgba(5,12,18,.55)), url("${a.wallpaper_url}")`,
    backgroundSize: 'cover',
    backgroundPosition: 'center'
  }
})
const editionLabel = computed(() => {
  const normal = t('app.normal')
  const pro = t('app.pro')
  return info.value.edition === 'pro' ? pro : normal
})

onMounted(async () => {
  ui.applyTheme()
  try {
    info.value = await api.public.info()
    ui.applyAppearance(info.value.appearance || {})
    document.title = `${info.value.name || 'PrivacyOS'} · ${t('login.title')}`
  } catch (error) {
    // Keep defaults when the API is unreachable.
  }
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

function startCountdown(seconds) {
  retryAfter.value = Math.max(0, Math.ceil(seconds || 0))
  if (timer) clearInterval(timer)
  timer = setInterval(() => {
    retryAfter.value = Math.max(0, retryAfter.value - 1)
    if (retryAfter.value === 0) clearInterval(timer)
  }, 1000)
}

async function submit() {
  if (retryAfter.value > 0) return
  if (!form.value.username || !form.value.password) {
    ElMessage.warning(t('login.usernamePlaceholder') + ' / ' + t('login.passwordPlaceholder'))
    return
  }
  try {
    await auth.login({
      username: form.value.username.trim(),
      password: form.value.password,
      totp: form.value.totp || undefined
    })
    ElMessage.success(t('dashboard.welcomeBack'))
    const redirect = route.query.redirect
    const target = typeof redirect === 'string' && redirect.startsWith('/') && redirect !== '/' ? redirect : null
    router.replace(target || { name: 'dashboard' })
  } catch (error) {
    const payload = errorPayload(error)
    if (payload.error?.code === 'totp_required') {
      needTotp.value = true
      ElMessage.warning(payload.error.message)
      return
    }
    if (payload.retry_after) startCountdown(payload.retry_after)
    if (payload.error?.code === 'too_many_attempts') needTotp.value = needTotp.value
    ElMessage.error(errorMessage(error, t('errors.unknown')))
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 1.05fr 0.95fr;
  background: var(--pos-bg);
}
.hero {
  position: relative;
  overflow: hidden;
  padding: 56px;
  display: flex;
  align-items: center;
  background:
    radial-gradient(900px 500px at 20% 10%, rgba(45, 212, 191, 0.16), transparent 60%),
    radial-gradient(700px 500px at 90% 90%, rgba(96, 165, 250, 0.14), transparent 60%),
    linear-gradient(160deg, #0a1118, #0d1a23 55%, #0a141b);
  border-right: 1px solid var(--pos-border);
}
.hero-inner {
  position: relative;
  z-index: 2;
  max-width: 480px;
}
.brand {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 42px;
}
.brand img {
  width: 60px;
  height: 60px;
}
.brand h1 {
  margin: 0;
  font-size: 32px;
  letter-spacing: 1px;
  color: #eef7fb;
}
.brand p {
  margin: 6px 0 0;
  color: #9fb6c6;
  font-size: 14px;
}
.features {
  list-style: none;
  padding: 0;
  margin: 0 0 56px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.features li {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #c6d5e1;
  font-size: 14px;
}
.features .el-icon {
  color: #2dd4bf;
  background: rgba(45, 212, 191, 0.12);
  padding: 7px;
  border-radius: 9px;
}
.hero-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #7f97a9;
  font-size: 12px;
}
.chip {
  padding: 2px 9px;
  border-radius: 999px;
  border: 1px solid rgba(45, 212, 191, 0.35);
  color: #2dd4bf;
  font-size: 10px;
  letter-spacing: 1px;
}
.orb {
  position: absolute;
  border-radius: 50%;
  filter: blur(70px);
  opacity: 0.55;
}
.orb-a {
  width: 360px;
  height: 360px;
  background: rgba(45, 212, 191, 0.28);
  top: -120px;
  right: -80px;
}
.orb-b {
  width: 300px;
  height: 300px;
  background: rgba(96, 165, 250, 0.22);
  bottom: -100px;
  left: -60px;
}
.form-side {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 28px;
}
.form-card {
  width: 100%;
  max-width: 410px;
}
.mobile-brand {
  display: none;
  align-items: center;
  gap: 10px;
  margin-bottom: 22px;
}
.mobile-brand img {
  width: 34px;
  height: 34px;
}
h2 {
  margin: 0 0 6px;
  font-size: 24px;
  font-weight: 650;
}
.sub {
  color: var(--pos-muted);
  font-size: 13px;
  margin: 0 0 26px;
}
.hint {
  font-size: 11.5px;
  color: var(--pos-muted);
  margin-top: 6px;
}
.submit {
  width: 100%;
  height: 46px;
  margin-top: 8px;
  font-size: 15px;
  letter-spacing: 2px;
}
.lock-box {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(251, 191, 36, 0.08);
  border: 1px solid rgba(251, 191, 36, 0.28);
  color: #fcd34d;
  margin-bottom: 12px;
  font-size: 12.5px;
}
.lock-box div {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.lock-box span {
  opacity: 0.8;
}
.demo-tip {
  margin-top: 22px;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px dashed var(--pos-border);
  color: var(--pos-muted);
  font-size: 12px;
  line-height: 1.7;
}
.demo-badge {
  color: var(--pos-accent);
  font-weight: 600;
  margin-right: 6px;
}
.installer-link { color: var(--pos-accent); }
.login-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 34px;
  color: var(--pos-muted);
  font-size: 11.5px;
}
.login-foot .spacer {
  flex: 1;
}
.login-foot .edition {
  padding: 1px 8px;
  border-radius: 999px;
  background: var(--pos-accent-soft);
  color: var(--pos-accent);
}

@media (max-width: 900px) {
  .login-page {
    grid-template-columns: 1fr;
  }
  .hero {
    display: none;
  }
  .mobile-brand {
    display: flex;
  }
  .form-side {
    padding: 28px 20px;
    align-items: flex-start;
    padding-top: 12vh;
  }
}
</style>
