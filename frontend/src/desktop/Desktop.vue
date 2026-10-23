<template>
  <div
    class="desktop-root"
    :class="{ light: ui.theme === 'light' }"
    @click="onRootClick"
    @contextmenu.prevent="onRootContext"
  >
    <div class="wallpaper" />
    <div class="wallpaper-grid" />
    <div class="wallpaper-glow" />

    <div class="desktop-body">
      <div class="icon-grid" @click.stop>
        <button
          v-for="app in desktopApps"
          :key="app.key"
          type="button"
          class="desktop-icon"
          :class="{ selected: selectedIcon === app.key }"
          @click="onIconClick(app)"
          @dblclick="launch(app.key)"
          @contextmenu.stop.prevent="onIconContext($event, app)"
        >
          <span class="icon-tile" :style="{ '--icon-color': app.color }">
            <el-icon :size="30"><component :is="app.icon" /></el-icon>
          </span>
          <span class="icon-label">{{ t(app.titleKey) }}</span>
        </button>
      </div>

      <div class="session-card">
        <span class="session-avatar">{{ initial }}</span>
        <div class="session-meta">
          <strong>{{ auth.displayName }}</strong>
          <span>{{ auth.isAdmin ? t('users.admin') : t('users.user') }} · {{ auth.editionName }}</span>
        </div>
      </div>

      <div class="watermark">
        <img src="/logo.svg" alt="" />
        <div>
          <strong>PrivacyOS</strong>
          <span>v{{ auth.version }} · Debian 13.7</span>
        </div>
      </div>

      <div class="desktop-tip">{{ t('desktop.tip') }}</div>
    </div>

    <div class="windows-layer" @contextmenu.prevent.stop>
      <WindowFrame
        v-for="win in store.windows"
        :key="win.id"
        :win="win"
        @close="closeWindow"
      />
    </div>

    <Taskbar
      :allowed="allowed"
      @launch="launch"
      @password="passwordOpen = true"
      @logout="logout"
    />

    <transition name="ctx-fade">
      <div
        v-if="ctx.visible"
        class="desktop-menu"
        :style="{ left: `${ctx.x}px`, top: `${ctx.y}px` }"
        @click.stop
      >
        <button type="button" @click="refreshDesktop">
          <el-icon><Refresh /></el-icon>{{ t('desktop.contextRefresh') }}
        </button>
        <button v-if="auth.isAdmin" type="button" @click="launch('settings')">
          <el-icon><Setting /></el-icon>{{ t('desktop.contextSettings') }}
        </button>
        <button type="button" @click="minimizeAll">
          <el-icon><Grid /></el-icon>{{ t('desktop.showDesktop') }}
        </button>
        <div class="menu-sep" />
        <button type="button" @click="aboutOpen = true">
          <el-icon><InfoFilled /></el-icon>{{ t('desktop.contextAbout') }}
        </button>
      </div>
    </transition>

    <el-dialog v-model="passwordOpen" :title="t('settings.account.changePassword')" width="430px">
      <el-form label-position="top" @submit.prevent>
        <el-form-item :label="t('settings.account.oldPassword')">
          <el-input v-model="passwordForm.old" type="password" show-password autocomplete="current-password" />
        </el-form-item>
        <el-form-item :label="t('settings.account.newPassword')">
          <el-input v-model="passwordForm.next" type="password" show-password autocomplete="new-password" />
        </el-form-item>
        <el-form-item :label="t('settings.account.confirmPassword')">
          <el-input v-model="passwordForm.confirm" type="password" show-password autocomplete="new-password" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="passwordOpen = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="busy" @click="submitPassword">
          {{ t('common.save') }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="aboutOpen" :title="t('desktop.aboutTitle')" width="440px">
      <div class="about-box">
        <img src="/logo.svg" alt="PrivacyOS" />
        <h2>PrivacyOS</h2>
        <p>{{ t('desktop.aboutBody') }}</p>
        <el-descriptions :column="1" size="small" border>
          <el-descriptions-item :label="t('app.version')">{{ auth.version }}</el-descriptions-item>
          <el-descriptions-item :label="t('app.edition')">{{ auth.editionName }}</el-descriptions-item>
          <el-descriptions-item :label="t('settings.about.builtOn')">Debian 13.7 Trixie</el-descriptions-item>
          <el-descriptions-item :label="t('settings.about.license')">MIT</el-descriptions-item>
        </el-descriptions>
        <p class="slogan">{{ t('settings.about.slogan') }}</p>
      </div>
      <template #footer>
        <el-button type="primary" @click="aboutOpen = false">{{ t('common.close') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Refresh, Setting, Grid, InfoFilled } from '@element-plus/icons-vue'
import WindowFrame from './WindowFrame.vue'
import Taskbar from './Taskbar.vue'
import { apps, allowedApps, desktopOrder } from './apps'
import { useDesktopStore } from '../stores/desktop'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'
import { api, errorMessage } from '../api'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useDesktopStore()
const auth = useAuthStore()
const ui = useUiStore()

const selectedIcon = ref('')
const ctx = reactive({ visible: false, x: 0, y: 0 })
const passwordOpen = ref(false)
const aboutOpen = ref(false)
const busy = ref(false)
const passwordForm = reactive({ old: '', next: '', confirm: '' })

const initial = computed(() => (auth.displayName || 'P').slice(0, 1).toUpperCase())
const allowed = computed(() => allowedApps(auth))
const desktopApps = computed(() => {
  const map = new Map(allowed.value.map((app) => [app.key, app]))
  return desktopOrder.map((key) => map.get(key)).filter(Boolean)
})

let stopAfterEach = null

onMounted(() => {
  const current = apps[route.name]
  if (current) store.open(current.key)
  window.addEventListener('resize', onResize)
  window.addEventListener('keydown', onKeydown)
  stopAfterEach = router.afterEach((to) => {
    const meta = apps[to.name]
    if (meta) store.open(meta.key)
  })
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  window.removeEventListener('keydown', onKeydown)
  if (stopAfterEach) stopAfterEach()
})

function onKeydown(event) {
  if (event.key === 'Escape') {
    store.startOpen = false
    ctx.visible = false
    selectedIcon.value = ''
  }
}

function onResize() {
  store.clampAll()
}

function onRootClick(event) {
  if (event.target.closest('.taskbar') || event.target.closest('.desktop-menu')) {
    return
  }
  store.startOpen = false
  ctx.visible = false
  if (!event.target.closest('.desktop-icon')) selectedIcon.value = ''
}

function onRootContext(event) {
  if (event.target.closest('.os-window') || event.target.closest('.taskbar')) return
  ctx.visible = true
  ctx.x = Math.min(event.clientX, window.innerWidth - 220)
  ctx.y = Math.min(event.clientY, window.innerHeight - 230)
}

function onIconClick(app) {
  selectedIcon.value = app.key
  if (window.innerWidth < 900) launch(app.key)
}

function onIconContext(event, app) {
  selectedIcon.value = app.key
  ctx.visible = true
  ctx.x = Math.min(event.clientX, window.innerWidth - 220)
  ctx.y = Math.min(event.clientY, window.innerHeight - 230)
}

function canLaunch(key) {
  const meta = apps[key]
  if (!meta) return false
  if (meta.admin && !auth.isAdmin) return false
  if (meta.feature && !auth.isAdmin && auth.features[meta.feature] === false) return false
  return true
}

function launch(key) {
  if (!canLaunch(key)) {
    ElMessage.warning(t('errors.forbidden'))
    return
  }
  const meta = apps[key]
  store.startOpen = false
  ctx.visible = false
  store.open(key)
  if (route.name !== meta.routeName) {
    router.push({ name: meta.routeName }).catch(() => {})
  }
}

function closeWindow(win) {
  const closed = store.close(win.id)
  if (closed && route.name === closed.routeName) {
    router.replace({ name: 'desktop' }).catch(() => {})
  }
}

function minimizeAll() {
  store.minimizeAll()
  ctx.visible = false
  router.replace({ name: 'desktop' }).catch(() => {})
}

function refreshDesktop() {
  ctx.visible = false
  window.location.reload()
}

async function submitPassword() {
  if (!passwordForm.old || !passwordForm.next) return
  if (passwordForm.next !== passwordForm.confirm) {
    ElMessage.error(t('settings.account.passwordMismatch'))
    return
  }
  if (passwordForm.next.length < 8) {
    ElMessage.error(t('settings.account.weakPassword'))
    return
  }
  busy.value = true
  try {
    await api.auth.changePassword({ old_password: passwordForm.old, new_password: passwordForm.next })
    ElMessage.success(t('settings.account.passwordChanged'))
    passwordOpen.value = false
    passwordForm.old = ''
    passwordForm.next = ''
    passwordForm.confirm = ''
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    busy.value = false
  }
}

async function logout() {
  try {
    await auth.logout()
  } finally {
    store.closeAll()
    router.replace({ name: 'login' })
  }
}
</script>

<style scoped>
.desktop-root {
  position: fixed;
  inset: 0;
  overflow: hidden;
  color: #edf6fb;
  background: #061019;
  user-select: none;
}
.wallpaper {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(1100px 620px at 12% 8%, rgba(13, 148, 136, 0.3), transparent 65%),
    radial-gradient(900px 620px at 88% 16%, rgba(37, 99, 235, 0.24), transparent 62%),
    radial-gradient(900px 700px at 62% 104%, rgba(14, 116, 144, 0.26), transparent 68%),
    linear-gradient(155deg, #050b11 0%, #071521 46%, #061019 100%);
}
.wallpaper-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(139, 161, 180, 0.055) 1px, transparent 1px),
    linear-gradient(90deg, rgba(139, 161, 180, 0.055) 1px, transparent 1px);
  background-size: 46px 46px;
  mask-image: radial-gradient(circle at 50% 45%, #000 20%, transparent 78%);
}
.wallpaper-glow {
  position: absolute;
  width: 720px;
  height: 720px;
  right: -180px;
  top: -260px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(45, 212, 191, 0.18), transparent 66%);
  filter: blur(30px);
}
.desktop-root.light .wallpaper {
  background:
    radial-gradient(1100px 620px at 12% 8%, rgba(13, 148, 136, 0.18), transparent 65%),
    radial-gradient(900px 620px at 88% 16%, rgba(37, 99, 235, 0.14), transparent 62%),
    linear-gradient(155deg, #e8f2f6 0%, #dcebf1 48%, #eef6f9 100%);
}
.desktop-root.light .wallpaper-grid {
  background-image:
    linear-gradient(rgba(16, 27, 38, 0.05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(16, 27, 38, 0.05) 1px, transparent 1px);
}
.desktop-root.light { color: #0f1d28; }

.desktop-body {
  position: absolute;
  inset: 0 0 56px 0;
  padding: 22px 24px;
  display: flex;
  gap: 22px;
}
.icon-grid {
  display: grid;
  grid-template-columns: repeat(2, 88px);
  grid-auto-rows: 96px;
  gap: 8px 6px;
  align-content: start;
  width: 194px;
}
.desktop-icon {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  padding: 6px 2px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  cursor: default;
}
.desktop-icon.selected {
  background: rgba(45, 212, 191, 0.14);
  border-color: rgba(45, 212, 191, 0.32);
}
.desktop-icon:hover .icon-tile {
  transform: translateY(-2px);
  border-color: rgba(255, 255, 255, 0.2);
}
.icon-tile {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--icon-color);
  background: color-mix(in srgb, var(--icon-color) 16%, rgba(9, 18, 26, 0.6));
  border: 1px solid color-mix(in srgb, var(--icon-color) 30%, transparent);
  box-shadow: 0 10px 22px rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(10px);
  transition: transform 0.15s ease, border-color 0.15s ease;
}
.desktop-root.light .icon-tile {
  background: color-mix(in srgb, var(--icon-color) 16%, rgba(255, 255, 255, 0.72));
  box-shadow: 0 8px 18px rgba(15, 34, 51, 0.12);
}
.icon-label {
  font-size: 11.5px;
  text-align: center;
  line-height: 1.25;
  max-width: 86px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.55);
}
.desktop-root.light .icon-label { text-shadow: none; }

.session-card {
  position: absolute;
  top: 22px;
  right: 26px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px 8px 8px;
  border-radius: 12px;
  background: rgba(8, 17, 25, 0.42);
  border: 1px solid rgba(139, 161, 180, 0.16);
  backdrop-filter: blur(16px);
}
.desktop-root.light .session-card {
  background: rgba(255, 255, 255, 0.62);
  border-color: rgba(16, 34, 51, 0.1);
}
.session-avatar {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: linear-gradient(135deg, #2dd4bf, #0ea5e9);
  color: #06202a;
  font-weight: 700;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.session-meta { display: flex; flex-direction: column; gap: 2px; }
.session-meta strong { font-size: 12px; }
.session-meta span { font-size: 10.5px; opacity: 0.66; }

.watermark {
  position: absolute;
  right: 30px;
  bottom: 22px;
  display: flex;
  align-items: center;
  gap: 12px;
  opacity: 0.55;
  pointer-events: none;
}
.watermark img { width: 46px; height: 46px; }
.watermark div { display: flex; flex-direction: column; gap: 2px; }
.watermark strong { font-size: 20px; letter-spacing: 1px; }
.watermark span { font-size: 10.5px; letter-spacing: 0.6px; }
.desktop-tip {
  position: absolute;
  left: 26px;
  bottom: 18px;
  font-size: 10.5px;
  opacity: 0.45;
  pointer-events: none;
}
.windows-layer {
  position: absolute;
  inset: 0 0 56px 0;
  pointer-events: none;
  z-index: 100;
}

.desktop-menu {
  position: fixed;
  z-index: 7000;
  min-width: 200px;
  padding: 6px;
  border-radius: 11px;
  background: color-mix(in srgb, var(--pos-panel) 94%, transparent);
  border: 1px solid var(--pos-border);
  box-shadow: 0 22px 60px rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(20px);
  display: flex;
  flex-direction: column;
}
.desktop-menu button {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 9px 10px;
  border: none;
  background: transparent;
  border-radius: 7px;
  color: var(--pos-text);
  font-size: 12.5px;
  cursor: pointer;
  text-align: left;
}
.desktop-menu button:hover { background: var(--pos-accent-soft); color: var(--pos-accent); }
.menu-sep { height: 1px; background: var(--pos-border); margin: 5px 4px; }
.ctx-fade-enter-active, .ctx-fade-leave-active { transition: opacity 0.12s ease, transform 0.12s ease; }
.ctx-fade-enter-from, .ctx-fade-leave-to { opacity: 0; transform: translateY(-4px); }
.about-box { display: flex; flex-direction: column; align-items: center; gap: 8px; text-align: center; }
.about-box img { width: 56px; height: 56px; }
.about-box h2 { margin: 0; letter-spacing: 1px; }
.about-box p { margin: 0 0 8px; color: var(--pos-muted); font-size: 12.5px; }
.about-box :deep(.el-descriptions) { width: 100%; text-align: left; }
.slogan { color: var(--pos-accent) !important; font-size: 12px; margin-top: 4px; }

@media (max-width: 900px) {
  .desktop-body { padding: 14px; gap: 10px; }
  .icon-grid {
    grid-template-columns: repeat(4, 72px);
    grid-auto-rows: 88px;
    width: auto;
    gap: 4px;
  }
  .icon-tile { width: 42px; height: 42px; }
  .icon-label { font-size: 10.5px; max-width: 68px; }
  .session-card, .watermark, .desktop-tip { display: none; }
}
</style>
