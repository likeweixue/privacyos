<template>
  <div
    class="desktop-root"
    :class="{ light: ui.theme === 'light' }"
    @click="onRootClick"
    @contextmenu.prevent="onRootContext"
  >
    <div class="wallpaper" />
    <div class="wallpaper-dots" />

    <div class="desktop-area">
      <div class="desktop-icons" @click.stop>
        <button
          v-for="app in desktopApps"
          :key="app.key"
          type="button"
          class="desktop-icon"
          :class="{ selected: selectedIcon === app.key }"
          @click="onIconClick(app)"
          @dblclick="onIconDoubleClick(app)"
          @contextmenu.stop.prevent="onIconContext($event, app)"
        >
          <span class="desktop-icon-tile" :style="{ color: app.color }">
            <el-icon :size="26"><component :is="app.icon" /></el-icon>
          </span>
          <span class="desktop-icon-label">{{ t(app.titleKey) }}</span>
          <span v-if="auth.features[app.feature] === false" class="desktop-icon-dot" />
        </button>
      </div>

      <div class="desktop-hint">{{ t('desktop.multiWindowHint') }}</div>

      <div class="desktop-watermark">
        <img src="/logo.svg" alt="" />
        <div>
          <strong>PrivacyOS</strong>
          <span>v{{ auth.version }} · {{ auth.editionName }} · Debian 13.7</span>
        </div>
      </div>
    </div>

    <div class="windows-layer" @contextmenu.prevent.stop>
      <WindowFrame v-for="win in store.windows" :key="win.id" :win="win" @close="closeWindow" />
    </div>

    <Taskbar
      :groups="groups"
      @launch="launchNew"
      @password="passwordOpen = true"
      @logout="logout"
      @close-window="closeWindow"
    />

    <transition name="ctx-pop">
      <div v-if="ctx.visible" class="desktop-menu" :style="{ left: `${ctx.x}px`, top: `${ctx.y}px` }" @click.stop>
        <button type="button" @click="refreshDesktop">
          <el-icon><Refresh /></el-icon>{{ t('desktop.refresh') }}
        </button>
        <button type="button" @click="launchNew('files')">
          <el-icon><FolderOpened /></el-icon>{{ t('desktop.newFileWindow') }}
        </button>
        <button type="button" @click="showDesktop">
          <el-icon><Grid /></el-icon>{{ t('desktop.showDesktop') }}
        </button>
        <div class="menu-sep" />
        <button type="button" @click="aboutOpen = true">
          <el-icon><InfoFilled /></el-icon>{{ t('desktop.about') }}
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
        <el-button type="primary" :loading="busy" @click="submitPassword">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="aboutOpen" :title="t('desktop.about')" width="430px">
      <div class="about-box">
        <img src="/logo.svg" alt="" />
        <h3>PrivacyOS</h3>
        <p>{{ t('desktop.aboutBody') }}</p>
        <el-descriptions :column="1" size="small" border>
          <el-descriptions-item :label="t('app.version')">{{ auth.version }}</el-descriptions-item>
          <el-descriptions-item :label="t('app.edition')">{{ auth.editionName }}</el-descriptions-item>
          <el-descriptions-item :label="t('dashboard.os')">Debian 13.7 Trixie</el-descriptions-item>
          <el-descriptions-item :label="t('settings.about.license')">MIT</el-descriptions-item>
        </el-descriptions>
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
import { ElMessage, ElMessageBox } from 'element-plus'
import { FolderOpened, Grid, InfoFilled, Refresh } from '@element-plus/icons-vue'
import WindowFrame from './WindowFrame.vue'
import Taskbar from './Taskbar.vue'
import { apps, visibleApps, visibleNavGroups, desktopApps as order } from '../apps'
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
const passwordOpen = ref(false)
const aboutOpen = ref(false)
const busy = ref(false)
const ctx = reactive({ visible: false, x: 0, y: 0 })
const passwordForm = reactive({ old: '', next: '', confirm: '' })

const allowed = computed(() => visibleApps(auth))
const groups = computed(() => visibleNavGroups(auth))
const desktopApps = computed(() => {
  const map = new Map(allowed.value.map((app) => [app.key, app]))
  return order.map((key) => map.get(key)).filter(Boolean)
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
    store.startOpen = false
  })
})
onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  window.removeEventListener('keydown', onKeydown)
  if (stopAfterEach) stopAfterEach()
})

function onResize() {
  store.clampAll()
}
function onKeydown(event) {
  if (event.key === 'Escape') {
    store.startOpen = false
    ctx.visible = false
    selectedIcon.value = ''
  }
}
function onRootClick(event) {
  if (event.target.closest('.Taskbar') || event.target.closest('.desktop-menu')) return
  store.startOpen = false
  ctx.visible = false
  if (!event.target.closest('.desktop-icon')) selectedIcon.value = ''
}
function onRootContext(event) {
  if (event.target.closest('.os-window') || event.target.closest('.Taskbar')) return
  store.startOpen = false
  ctx.visible = true
  ctx.x = Math.min(event.clientX, window.innerWidth - 210)
  ctx.y = Math.min(event.clientY, window.innerHeight - 180)
}

function canLaunch(key) {
  const meta = apps[key]
  if (!meta) return false
  if (meta.admin && !auth.isAdmin) return false
  if (meta.feature && !auth.isAdmin && auth.features[meta.feature] === false) return false
  return true
}
function launchNew(key) {
  if (!canLaunch(key)) {
    ElMessage.warning(t('errors.forbidden'))
    return
  }
  store.startOpen = false
  ctx.visible = false
  store.open(key, { forceNew: true })
  if (route.name !== key) router.push({ name: key }).catch(() => {})
}
function onIconClick(app) {
  selectedIcon.value = app.key
  if (window.innerWidth < 900) launchNew(app.key)
}
function onIconDoubleClick(app) {
  if (window.innerWidth >= 900) launchNew(app.key)
}
function onIconContext(event, app) {
  selectedIcon.value = app.key
  ctx.visible = true
  ctx.x = Math.min(event.clientX, window.innerWidth - 210)
  ctx.y = Math.min(event.clientY, window.innerHeight - 180)
}
function closeWindow(win) {
  const closed = store.close(win.id)
  if (!closed) return
  if (route.name === closed.app) {
    const top = store.topVisible()
    router.replace(top ? { name: top.app } : { name: 'desktop' }).catch(() => {})
  }
}
function showDesktop() {
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
    await ElMessageBox.confirm(t('nav.logoutConfirm'), t('common.logout'), {
      type: 'warning', confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
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
  background: var(--pos-bg);
  color: var(--pos-text);
  user-select: none;
}
.wallpaper {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(900px 600px at 85% 10%, rgba(15, 118, 110, 0.06), transparent 60%),
    linear-gradient(150deg, color-mix(in srgb, var(--pos-bg) 88%, #fff), var(--pos-bg));
}
.wallpaper-dots {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(color-mix(in srgb, var(--pos-muted) 30%, transparent) 1px, transparent 1px);
  background-size: 26px 26px;
  opacity: 0.35;
  mask-image: linear-gradient(120deg, #000 0%, transparent 72%);
}
html.dark .wallpaper {
  background:
    radial-gradient(900px 600px at 85% 10%, rgba(45, 212, 191, 0.07), transparent 60%),
    linear-gradient(150deg, #0b1016, var(--pos-bg));
}
.desktop-area {
  position: absolute;
  inset: 0 0 52px 0;
  padding: 18px 20px;
}
.desktop-icons {
  display: grid;
  grid-template-columns: repeat(2, 86px);
  grid-auto-rows: 90px;
  gap: 4px;
  align-content: start;
  width: 184px;
}
.desktop-icon {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  padding: 5px 2px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  cursor: default;
}
.desktop-icon:hover .desktop-icon-tile { border-color: var(--pos-border-strong); background: var(--pos-panel); }
.desktop-icon.selected { background: var(--pos-accent-soft); border-color: color-mix(in srgb, var(--pos-accent) 25%, transparent); }
.desktop-icon-tile {
  width: 42px;
  height: 42px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  background: color-mix(in srgb, var(--pos-panel) 86%, transparent);
  box-shadow: 0 2px 4px rgba(20, 32, 44, 0.05);
}
.desktop-icon-label {
  font-size: 10.5px;
  line-height: 1.25;
  max-width: 82px;
  text-align: center;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.desktop-icon-dot {
  position: absolute;
  top: 8px;
  right: 15px;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--pos-muted);
}
.desktop-hint {
  position: absolute;
  left: 22px;
  bottom: 12px;
  font-size: 10px;
  color: var(--pos-muted);
  opacity: 0.75;
}
.desktop-watermark {
  position: absolute;
  right: 24px;
  bottom: 16px;
  display: flex;
  align-items: center;
  gap: 10px;
  opacity: 0.6;
  pointer-events: none;
}
.desktop-watermark img { width: 38px; height: 38px; }
.desktop-watermark div { display: flex; flex-direction: column; gap: 1px; }
.desktop-watermark strong { font-size: 17px; letter-spacing: 0.8px; }
.desktop-watermark span { font-size: 9.5px; color: var(--pos-muted); letter-spacing: 0.3px; }
.windows-layer {
  position: absolute;
  inset: 0 0 52px 0;
  z-index: 100;
  pointer-events: none;
}
.desktop-menu {
  position: fixed;
  z-index: 7000;
  min-width: 196px;
  padding: 4px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border-strong);
  box-shadow: 0 14px 36px rgba(20, 32, 44, 0.18);
  display: flex;
  flex-direction: column;
}
.desktop-menu button {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 8px;
  border: none;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  font-size: 11.5px;
  text-align: left;
  cursor: pointer;
}
.desktop-menu button:hover { background: var(--pos-panel-2); }
.menu-sep { height: 1px; background: var(--pos-border); margin: 4px 3px; }
.about-box { display: flex; flex-direction: column; align-items: center; gap: 7px; text-align: center; }
.about-box img { width: 52px; height: 52px; }
.about-box h3 { margin: 0; }
.about-box p { margin: 0 0 6px; color: var(--pos-muted); font-size: 12px; }
.about-box :deep(.el-descriptions) { width: 100%; text-align: left; }
.ctx-pop-enter-active, .ctx-pop-leave-active { transition: opacity 0.1s ease; }
.ctx-pop-enter-from, .ctx-pop-leave-to { opacity: 0; }
@media (max-width: 900px) {
  .desktop-area { padding: 12px; }
  .desktop-icons { grid-template-columns: repeat(4, 76px); grid-auto-rows: 82px; width: auto; }
  .desktop-icon-tile { width: 38px; height: 38px; }
  .desktop-icon-label { max-width: 70px; font-size: 10px; }
  .desktop-watermark, .desktop-hint { display: none; }
  .windows-layer { inset: 0 0 52px 0; }
}
</style>
