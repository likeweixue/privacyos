<template>
  <footer class="Taskbar" @click.stop>
    <div class="Taskbar-left">
      <button type="button" class="start-button" :class="{ active: store.startOpen }" :title="t('desktop.start')" @click="toggleStart">
        <img src="/logo.svg" alt="" />
        <span>{{ t('desktop.start') }}</span>
      </button>
      <button type="button" class="task-search" :title="t('search.open')" :aria-label="t('search.open')" @click="openSearch">
        <el-icon :size="15"><Search /></el-icon>
      </button>
      <span class="task-divider" />
      <div class="window-list">
        <button
          v-for="win in store.windows"
          :key="win.id"
          type="button"
          class="window-task"
          :class="{ active: store.activeId === win.id && !win.minimized, minimized: win.minimized }"
          :title="windowLabel(win)"
          @click="onTaskClick(win)"
          @contextmenu.prevent="onTaskContext($event, win)"
        >
          <span class="window-task-icon" :style="{ color: apps[win.app].color }">
            <el-icon :size="16"><component :is="apps[win.app].icon" /></el-icon>
          </span>
          <span class="window-task-label">{{ windowLabel(win) }}</span>
        </button>
      </div>
    </div>

    <div class="Taskbar-right">
      <el-popover v-model:visible="inboxOpen" placement="top-end" :width="380" trigger="click" @show="loadNotifications">
        <template #reference>
          <button type="button" class="tray-button" :title="t('notifications.title')">
            <el-badge :value="unread" :hidden="!unread" :max="99">
              <el-icon :size="16"><Bell /></el-icon>
            </el-badge>
          </button>
        </template>
        <div class="inbox">
          <div class="inbox-head">
            <strong>{{ t('notifications.title') }}</strong>
            <el-button link size="small" @click="readAll">{{ t('notifications.markAllRead') }}</el-button>
          </div>
          <div v-if="!notifications.length" class="inbox-empty">{{ t('notifications.empty') }}</div>
          <div v-for="n in notifications" :key="n.id" class="inbox-item" :class="{ unread: !n.read }">
            <span class="level" :class="n.level" />
            <div class="inbox-body">
              <div class="inbox-title">{{ n.title }}</div>
              <div class="inbox-msg">{{ n.message }}</div>
              <div class="inbox-time">{{ formatRelative(n.created_at) }}</div>
            </div>
            <button type="button" class="inbox-del" @click="removeNotification(n.id)">
              <el-icon><Close /></el-icon>
            </button>
          </div>
        </div>
      </el-popover>

      <button type="button" class="tray-button" :title="t('nav.theme')" @click="ui.toggleTheme">
        <el-icon :size="16"><component :is="ui.theme === 'dark' ? 'Sunny' : 'Moon'" /></el-icon>
      </button>
      <el-dropdown trigger="click" @command="ui.setLang">
        <button type="button" class="tray-button lang">{{ ui.lang === 'zh-CN' ? '中' : 'EN' }}</button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="zh-CN" :disabled="ui.lang === 'zh-CN'">简体中文</el-dropdown-item>
            <el-dropdown-item command="en-US" :disabled="ui.lang === 'en-US'">English</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <div class="clock" :title="clockTitle">
        <strong>{{ timeText }}</strong>
        <span>{{ dateText }}</span>
      </div>

      <el-dropdown trigger="click" @command="onSessionCommand">
        <button type="button" class="session-button" :title="auth.displayName" aria-haspopup="menu">
          <span class="session-avatar">{{ initial }}</span>
          <span class="session-name">{{ auth.displayName }}</span>
          <el-icon :size="11"><ArrowDown /></el-icon>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item disabled>{{ auth.displayName }} · {{ auth.editionName }}</el-dropdown-item>
            <el-dropdown-item command="settings" divided>
              <el-icon><Setting /></el-icon>{{ t('nav.settings') }}
            </el-dropdown-item>
            <el-dropdown-item command="password">
              <el-icon><Lock /></el-icon>{{ t('settings.account.changePassword') }}
            </el-dropdown-item>
            <el-dropdown-item command="logout" divided>
              <el-icon><SwitchButton /></el-icon>{{ t('common.logout') }}
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <button type="button" class="show-desktop" :title="t('desktop.showDesktop')" @click="showDesktop" />
    </div>

    <StartMenu
      v-if="store.startOpen"
      :groups="groups"
      @launch="(key) => emit('launch', key)"
      @logout="emit('logout')"
    />

    <div v-if="taskMenu.open" class="task-context" :style="{ left: `${taskMenu.x}px`, top: `${taskMenu.y}px` }" @click.stop>
      <button type="button" @click="taskAction('new')">
        <el-icon><CopyDocument /></el-icon>{{ t('desktop.newWindow') }}
      </button>
      <button type="button" @click="taskAction('close')">
        <el-icon><Close /></el-icon>{{ t('desktop.closeWindow') }}
      </button>
    </div>
  </footer>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { ArrowDown, Bell, Close, CopyDocument, Lock, Search, Setting, SwitchButton } from '@element-plus/icons-vue'
import { api, errorMessage } from '../api'
import { apps } from '../apps'
import { useAuthStore } from '../stores/auth'
import { useDesktopStore } from '../stores/desktop'
import { useUiStore } from '../stores/ui'
import { formatRelative } from '../utils/format'
import StartMenu from './StartMenu.vue'

defineProps({ groups: { type: Array, default: () => [] } })
const emit = defineEmits(['launch', 'logout', 'password', 'close-window'])

const { t } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const store = useDesktopStore()
const ui = useUiStore()
const inboxOpen = ref(false)
const notifications = ref([])
const unread = ref(0)
const now = ref(new Date())
const taskMenu = reactive({ open: false, x: 0, y: 0, win: null })
let timer = null

const initial = computed(() => (auth.displayName || 'P').slice(0, 1).toUpperCase())
const locale = computed(() => (ui.lang === 'en-US' ? 'en-US' : 'zh-CN'))
const timeText = computed(() => now.value.toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' }))
const dateText = computed(() => now.value.toLocaleDateString(locale.value, { month: '2-digit', day: '2-digit' }))
const clockTitle = computed(() => now.value.toLocaleDateString(locale.value, { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long' }))

function onDocumentClick() {
  taskMenu.open = false
}
onMounted(() => {
  timer = setInterval(() => { now.value = new Date() }, 15000)
  loadNotifications()
  document.addEventListener('click', onDocumentClick)
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  document.removeEventListener('click', onDocumentClick)
})

function windowLabel(win) {
  const base = t(apps[win.app]?.titleKey || 'common.window')
  return store.instanceCount(win.app) > 1 ? `${base} · ${store.instanceNo(win)}` : base
}
function toggleStart() {
  store.startOpen = !store.startOpen
  taskMenu.open = false
}
function openSearch() {
  store.startOpen = false
  store.openSearch()
}
function onTaskClick(win) {
  taskMenu.open = false
  store.startOpen = false
  if (win.minimized) {
    store.focus(win.id)
  } else if (store.activeId === win.id) {
    store.minimize(win.id)
    return
  } else {
    store.focus(win.id)
  }
  router.replace({ name: win.app }).catch(() => {})
}
function onTaskContext(event, win) {
  taskMenu.open = true
  taskMenu.win = win
  taskMenu.x = Math.min(event.clientX, window.innerWidth - 180)
  taskMenu.y = Math.max(8, event.clientY - 88)
}
function taskAction(action) {
  const win = taskMenu.win
  taskMenu.open = false
  if (!win) return
  if (action === 'new') {
    store.open(win.app, { forceNew: true })
  } else if (action === 'close') {
    emit('close-window', win)
  }
}
function showDesktop() {
  taskMenu.open = false
  store.startOpen = false
  store.minimizeAll()
  router.replace({ name: 'desktop' }).catch(() => {})
}
function onSessionCommand(command) {
  if (command === 'settings') emit('launch', 'settings')
  else if (command === 'password') emit('password')
  else if (command === 'logout') emit('logout')
}
async function loadNotifications() {
  try {
    const payload = await api.notifications.list({ limit: 30 })
    notifications.value = payload.items || []
    unread.value = payload.unread || 0
  } catch (error) { /* non-critical */ }
}
async function readAll() {
  try {
    await api.notifications.read(0)
    notifications.value = notifications.value.map((n) => ({ ...n, read: true }))
    unread.value = 0
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
async function removeNotification(id) {
  try {
    await api.notifications.remove(id)
    notifications.value = notifications.value.filter((n) => n.id !== id)
    unread.value = notifications.value.filter((n) => !n.read).length
  } catch (error) { ElMessage.error(errorMessage(error)) }
}
</script>

<style scoped>
.Taskbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  height: 52px;
  z-index: 5000;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 6px;
  background: var(--pos-panel);
  border-top: 1px solid var(--pos-border);
}
.Taskbar-left { display: flex; align-items: center; gap: 3px; min-width: 0; flex: 1; height: 100%; }
.start-button {
  height: 38px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 10px 0 6px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  font-size: 12px;
  font-weight: 550;
  cursor: pointer;
  flex-shrink: 0;
}
.start-button img { width: 25px; height: 25px; }
.start-button:hover, .start-button.active { background: var(--pos-panel-2); border-color: var(--pos-border); }
.task-search {
  width: 32px;
  height: 34px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.task-search:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.task-divider { width: 1px; height: 20px; background: var(--pos-border); margin: 0 3px; flex-shrink: 0; }
.window-list { display: flex; align-items: center; gap: 2px; overflow-x: auto; min-width: 0; height: 100%; scrollbar-width: none; }
.window-list::-webkit-scrollbar { display: none; }
.window-task {
  position: relative;
  height: 38px;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  max-width: 200px;
  flex-shrink: 0;
}
.window-task:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.window-task.active { background: var(--pos-accent-soft); border-color: color-mix(in srgb, var(--pos-accent) 25%, transparent); color: var(--pos-text); }
.window-task.active::after {
  content: '';
  position: absolute;
  left: 18%;
  right: 18%;
  bottom: 1px;
  height: 2px;
  background: var(--pos-accent);
}
.window-task.minimized { opacity: 0.62; }
.window-task-icon { display: inline-flex; }
.window-task-label { font-size: 11px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.Taskbar-right { display: flex; align-items: center; gap: 1px; height: 100%; flex-shrink: 0; }
.tray-button {
  width: 32px;
  height: 36px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.tray-button:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.tray-button.lang { font-size: 11px; font-weight: 600; }
.clock {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  padding: 0 10px;
  line-height: 1.3;
  min-width: 66px;
  border-radius: 2px;
}
.clock:hover { background: var(--pos-panel-2); }
.clock strong { font-size: 11px; font-weight: 550; font-variant-numeric: tabular-nums; }
.clock span { font-size: 9.5px; color: var(--pos-muted); }
.session-button {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 6px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  cursor: pointer;
}
.session-button:hover { background: var(--pos-panel-2); }
.session-avatar {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: var(--pos-accent);
  color: #fff;
  font-size: 10.5px;
  font-weight: 650;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.session-name { font-size: 11px; max-width: 110px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.show-desktop {
  width: 8px;
  height: 32px;
  border: none;
  border-left: 1px solid var(--pos-border);
  background: transparent;
  cursor: pointer;
  margin-left: 2px;
}
.show-desktop:hover { background: var(--pos-accent-soft); border-left-color: var(--pos-accent); }
.inbox { max-height: 420px; overflow: auto; }
.inbox-head { display: flex; justify-content: space-between; align-items: center; padding-bottom: 8px; border-bottom: 1px solid var(--pos-border); margin-bottom: 6px; }
.inbox-empty { padding: 24px 0; text-align: center; color: var(--pos-muted); font-size: 12px; }
.inbox-item { display: flex; gap: 8px; padding: 9px 3px; border-bottom: 1px solid var(--pos-border); }
.inbox-item:last-child { border-bottom: none; }
.level { width: 6px; height: 6px; border-radius: 50%; margin-top: 5px; background: var(--pos-blue); flex-shrink: 0; }
.level.warning { background: var(--pos-warning); }
.level.critical, .level.error { background: var(--pos-danger); }
.level.success { background: var(--pos-success); }
.inbox-body { flex: 1; min-width: 0; }
.inbox-title { font-size: 12px; font-weight: 600; }
.inbox-msg { font-size: 11px; color: var(--pos-muted); line-height: 1.5; }
.inbox-time { font-size: 10px; color: var(--pos-muted); margin-top: 3px; opacity: 0.8; }
.inbox-del { background: transparent; border: none; color: var(--pos-muted); cursor: pointer; opacity: 0.5; }
.inbox-del:hover { opacity: 1; color: var(--pos-danger); }
.task-context {
  position: fixed;
  z-index: 6500;
  min-width: 160px;
  padding: 4px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border-strong);
  box-shadow: 0 12px 30px rgba(20, 32, 44, 0.18);
  display: flex;
  flex-direction: column;
}
.task-context button {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 7px 8px;
  border: none;
  background: transparent;
  border-radius: 2px;
  color: var(--pos-text);
  font-size: 11.5px;
  text-align: left;
  cursor: pointer;
}
.task-context button:hover { background: var(--pos-panel-2); }
@media (max-width: 900px) {
  .start-button span, .start-button { padding-right: 6px; }
  .start-button span { display: none; }
  .window-task-label, .session-name, .clock span { display: none; }
  .window-task { padding: 0 9px; }
  .clock { min-width: 48px; padding: 0 7px; }
  .task-search { display: none; }
}
</style>
