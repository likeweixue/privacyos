<template>
  <footer class="taskbar" @click.stop>
    <div class="taskbar-left">
      <button
        type="button"
        class="start-btn"
        :class="{ active: store.startOpen }"
        :title="t('desktop.start')"
        @click="toggleStart"
      >
        <img src="/logo.svg" alt="PrivacyOS" />
        <span class="start-label">{{ t('desktop.start') }}</span>
      </button>

      <div class="divider" />

      <div class="window-list">
        <button
          v-for="win in store.windows"
          :key="win.id"
          type="button"
          class="task-btn"
          :class="{ active: store.activeId === win.id && !win.minimized, minimized: win.minimized }"
          :title="t(apps[win.app]?.titleKey || 'desktop.window')"
          @click="onTaskClick(win)"
        >
          <span class="task-icon" :style="{ color: apps[win.app]?.color }">
            <el-icon :size="18"><component :is="apps[win.app]?.icon" /></el-icon>
          </span>
          <span class="task-label">{{ t(apps[win.app]?.titleKey || '') }}</span>
        </button>
      </div>
    </div>

    <div class="taskbar-right">
      <el-popover v-model:visible="inboxOpen" placement="top-end" :width="390" trigger="click" @show="loadNotifications">
        <template #reference>
          <button type="button" class="tray-btn" :title="t('notifications.title')">
            <el-badge :value="unread" :hidden="!unread" :max="99">
              <el-icon :size="17"><Bell /></el-icon>
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

      <button type="button" class="tray-btn" :title="t('desktop.theme')" @click="ui.toggleTheme">
        <el-icon :size="17"><component :is="ui.theme === 'dark' ? 'Sunny' : 'Moon'" /></el-icon>
      </button>

      <el-dropdown trigger="click" @command="ui.setLang">
        <button type="button" class="tray-btn lang">{{ ui.lang === 'zh-CN' ? '中' : 'EN' }}</button>
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

      <el-dropdown trigger="click" @command="onUserCommand">
        <button type="button" class="user-tray" :title="auth.displayName">
          <span class="avatar">{{ initial }}</span>
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item disabled>
              {{ auth.displayName }} · {{ auth.editionName }}
            </el-dropdown-item>
            <el-dropdown-item command="password" divided>
              <el-icon><Lock /></el-icon>{{ t('settings.account.changePassword') }}
            </el-dropdown-item>
            <el-dropdown-item v-if="auth.isAdmin" command="settings">
              <el-icon><Setting /></el-icon>{{ t('nav.settings') }}
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
      :apps="allowed"
      @launch="launch"
      @password="emit('password')"
      @logout="emit('logout')"
    />
  </footer>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Bell, Close, Lock, Setting, SwitchButton
} from '@element-plus/icons-vue'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useDesktopStore } from '../stores/desktop'
import { useUiStore } from '../stores/ui'
import { apps } from '../desktop/apps'
import { formatRelative } from '../utils/format'
import StartMenu from './StartMenu.vue'

const props = defineProps({
  allowed: { type: Array, default: () => [] }
})
const emit = defineEmits(['launch', 'password', 'logout'])

const { t } = useI18n()
const auth = useAuthStore()
const store = useDesktopStore()
const ui = useUiStore()
const router = useRouter()

const inboxOpen = ref(false)
const notifications = ref([])
const unread = ref(0)
const now = ref(new Date())
let timer = null

const initial = computed(() => (auth.displayName || 'P').slice(0, 1).toUpperCase())
const locale = computed(() => (ui.lang === 'en-US' ? 'en-US' : 'zh-CN'))
const timeText = computed(() => now.value.toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' }))
const dateText = computed(() => now.value.toLocaleDateString(locale.value, { month: '2-digit', day: '2-digit' }))
const clockTitle = computed(() =>
  now.value.toLocaleDateString(locale.value, { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long' })
)

onMounted(() => {
  timer = setInterval(() => { now.value = new Date() }, 15000)
  loadNotifications()
})
onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

function toggleStart() {
  store.startOpen = !store.startOpen
}

function launch(key) {
  store.startOpen = false
  emit('launch', key)
}

function onTaskClick(win) {
  store.startOpen = false
  const meta = apps[win.app]
  if (win.minimized) {
    store.focus(win.id)
  } else if (store.activeId === win.id) {
    store.minimize(win.id)
  } else {
    store.focus(win.id)
  }
  if (meta && !win.minimized) {
    router.replace({ name: meta.routeName }).catch(() => {})
  }
}

function showDesktop() {
  store.startOpen = false
  store.minimizeAll()
  router.replace({ name: 'desktop' }).catch(() => {})
}

async function loadNotifications() {
  try {
    const payload = await api.notifications.list({ limit: 30 })
    notifications.value = payload.items || []
    unread.value = payload.unread || 0
  } catch (error) {
    // Non-critical.
  }
}

async function readAll() {
  try {
    await api.notifications.read(0)
    notifications.value = notifications.value.map((n) => ({ ...n, read: true }))
    unread.value = 0
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function removeNotification(id) {
  try {
    await api.notifications.remove(id)
    notifications.value = notifications.value.filter((n) => n.id !== id)
    unread.value = notifications.value.filter((n) => !n.read).length
  } catch (error) {
    ElMessage.error(errorMessage(error))
  }
}

async function onUserCommand(command) {
  if (command === 'password') {
    emit('password')
  } else if (command === 'settings') {
    launch('settings')
  } else if (command === 'logout') {
    try {
      await ElMessageBox.confirm(t('desktop.logoutConfirm'), t('common.logout'), {
        type: 'warning',
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel')
      })
    } catch (error) {
      return
    }
    emit('logout')
  }
}
</script>

<style scoped>
.taskbar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  height: 56px;
  z-index: 5000;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 0 8px;
  background: color-mix(in srgb, var(--pos-panel) 78%, transparent);
  border-top: 1px solid var(--pos-border);
  backdrop-filter: blur(26px) saturate(1.35);
  box-shadow: 0 -10px 30px rgba(0, 0, 0, 0.25);
}
.taskbar-left {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 1;
  height: 100%;
}
.start-btn {
  height: 42px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px 0 7px;
  border: 1px solid transparent;
  border-radius: 9px;
  background: transparent;
  color: var(--pos-text);
  cursor: pointer;
  flex-shrink: 0;
}
.start-btn:hover, .start-btn.active {
  background: rgba(139, 161, 180, 0.12);
  border-color: var(--pos-border);
}
.start-btn img { width: 28px; height: 28px; }
.start-label { font-size: 12.5px; font-weight: 550; }
.divider {
  width: 1px;
  height: 24px;
  background: var(--pos-border);
  margin: 0 4px;
  flex-shrink: 0;
}
.window-list {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow-x: auto;
  min-width: 0;
  height: 100%;
  scrollbar-width: none;
}
.window-list::-webkit-scrollbar { display: none; }
.task-btn {
  position: relative;
  height: 42px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  max-width: 190px;
  flex-shrink: 0;
}
.task-btn:hover { background: rgba(139, 161, 180, 0.1); color: var(--pos-text); }
.task-btn.active {
  background: rgba(45, 212, 191, 0.1);
  border-color: rgba(45, 212, 191, 0.22);
  color: var(--pos-text);
}
.task-btn.active::after {
  content: '';
  position: absolute;
  left: 20%;
  right: 20%;
  bottom: 2px;
  height: 2px;
  border-radius: 2px;
  background: var(--pos-accent);
}
.task-btn.minimized { opacity: 0.62; }
.task-icon { display: inline-flex; }
.task-label {
  font-size: 11.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.taskbar-right {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 100%;
  flex-shrink: 0;
}
.tray-btn {
  width: 34px;
  height: 38px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.tray-btn:hover { background: rgba(139, 161, 180, 0.12); color: var(--pos-text); }
.tray-btn.lang { font-size: 12px; font-weight: 600; }
.clock {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  padding: 0 12px;
  border-radius: 7px;
  line-height: 1.3;
  min-width: 72px;
  cursor: default;
}
.clock:hover { background: rgba(139, 161, 180, 0.1); }
.clock strong { font-size: 11.5px; font-weight: 550; font-variant-numeric: tabular-nums; }
.clock span { font-size: 10px; color: var(--pos-muted); }
.user-tray {
  width: 38px;
  height: 38px;
  padding: 0;
  border: none;
  background: transparent;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
}
.user-tray:hover { background: rgba(139, 161, 180, 0.12); }
.avatar {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(135deg, #2dd4bf, #0ea5e9);
  color: #06202a;
  font-weight: 700;
  font-size: 11px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.show-desktop {
  width: 10px;
  height: 34px;
  margin-left: 3px;
  border: 1px solid transparent;
  border-left-color: var(--pos-border);
  background: transparent;
  cursor: pointer;
}
.show-desktop:hover { background: rgba(139, 161, 180, 0.15); border-left-color: var(--pos-accent); }
.inbox { max-height: 430px; overflow: auto; }
.inbox-head {
  display: flex; justify-content: space-between; align-items: center;
  padding-bottom: 8px; border-bottom: 1px solid var(--pos-border); margin-bottom: 6px;
}
.inbox-empty { padding: 26px 0; text-align: center; color: var(--pos-muted); font-size: 13px; }
.inbox-item { display: flex; gap: 10px; padding: 10px 4px; border-bottom: 1px dashed var(--pos-border); }
.inbox-item:last-child { border-bottom: none; }
.inbox-item.unread .inbox-title { color: var(--pos-accent); }
.level { width: 7px; height: 7px; border-radius: 50%; margin-top: 5px; background: var(--pos-blue); flex-shrink: 0; }
.level.warning { background: var(--pos-warning); }
.level.critical, .level.error { background: var(--pos-danger); }
.level.success { background: var(--pos-success); }
.inbox-body { flex: 1; min-width: 0; }
.inbox-title { font-size: 13px; font-weight: 600; margin-bottom: 2px; }
.inbox-msg { font-size: 12px; color: var(--pos-muted); line-height: 1.55; }
.inbox-time { font-size: 10.5px; color: var(--pos-muted); margin-top: 4px; opacity: 0.7; }
.inbox-del { background: transparent; border: none; color: var(--pos-muted); cursor: pointer; opacity: 0.5; }
.inbox-del:hover { opacity: 1; color: var(--pos-danger); }
@media (max-width: 900px) {
  .start-label, .divider, .task-label, .clock span { display: none; }
  .task-btn { padding: 0 10px; }
  .clock { padding: 0 8px; min-width: 46px; }
}
</style>
