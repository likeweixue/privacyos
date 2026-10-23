<template>
  <transition name="start-fade">
    <div v-if="store.startOpen" class="start-menu" @click.stop>
      <div class="start-head">
        <span class="avatar">{{ initial }}</span>
        <div class="who">
          <strong>{{ auth.displayName }}</strong>
          <span>{{ auth.isAdmin ? t('users.admin') : t('users.user') }} · {{ auth.editionName }}</span>
        </div>
        <div class="spacer" />
        <el-tag size="small" effect="plain" :type="auth.pro ? 'warning' : 'info'">
          v{{ auth.version }}
        </el-tag>
      </div>

      <div class="search-row">
        <el-input
          v-model="query"
          :placeholder="t('desktop.searchApps')"
          :prefix-icon="Search"
          clearable
          size="default"
        />
      </div>

      <div class="app-section-title">
        <span>{{ t('desktop.allApps') }}</span>
        <span class="pos-hint">{{ filtered.length }}</span>
      </div>

      <div class="app-grid">
        <button
          v-for="app in filtered"
          :key="app.key"
          type="button"
          class="app-tile"
          @click="launch(app)"
        >
          <span class="tile-icon" :style="{ background: `${app.color}20`, color: app.color }">
            <el-icon :size="23"><component :is="app.icon" /></el-icon>
          </span>
          <span class="tile-label">{{ t(app.titleKey) }}</span>
        </button>
        <div v-if="!filtered.length" class="empty">{{ t('common.empty') }}</div>
      </div>

      <div class="start-foot">
        <button type="button" class="foot-action" @click="$emit('password')">
          <el-icon><Lock /></el-icon>{{ t('settings.account.changePassword') }}
        </button>
        <button v-if="auth.isAdmin" type="button" class="foot-action" @click="launchApp('settings')">
          <el-icon><Setting /></el-icon>{{ t('nav.settings') }}
        </button>
        <div class="spacer" />
        <button type="button" class="power" :title="t('common.logout')" @click="logout">
          <el-icon><SwitchButton /></el-icon>
        </button>
      </div>
    </div>
  </transition>
</template>

<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessageBox } from 'element-plus'
import { Search, Lock, Setting, SwitchButton } from '@element-plus/icons-vue'
import { useDesktopStore } from '../stores/desktop'
import { useAuthStore } from '../stores/auth'

const props = defineProps({
  apps: { type: Array, default: () => [] }
})
const emit = defineEmits(['launch', 'password', 'logout'])

const { t } = useI18n()
const store = useDesktopStore()
const auth = useAuthStore()
const query = ref('')

const filtered = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.apps
  return props.apps.filter((app) => {
    const label = t(app.titleKey).toLowerCase()
    return label.includes(q) || app.key.includes(q)
  })
})

const initial = computed(() => (auth.displayName || 'P').slice(0, 1).toUpperCase())

function launch(app) {
  store.startOpen = false
  emit('launch', app.key)
}

function launchApp(key) {
  store.startOpen = false
  emit('launch', key)
}

async function logout() {
  store.startOpen = false
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
</script>

<style scoped>
.start-menu {
  position: fixed;
  left: 12px;
  bottom: 64px;
  width: 590px;
  max-width: calc(100vw - 24px);
  max-height: min(640px, calc(100vh - 90px));
  z-index: 6000;
  display: flex;
  flex-direction: column;
  padding: 16px;
  border-radius: 16px;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  box-shadow: 0 30px 100px rgba(0, 0, 0, 0.72), 0 0 0 1px rgba(255, 255, 255, 0.03);
}
.start-head {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 2px 4px 12px;
  border-bottom: 1px solid var(--pos-border);
}
.avatar {
  width: 38px;
  height: 38px;
  border-radius: 11px;
  background: linear-gradient(135deg, #2dd4bf, #0ea5e9);
  color: #06202a;
  font-weight: 700;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.who { display: flex; flex-direction: column; gap: 2px; }
.who strong { font-size: 13px; }
.who span { font-size: 11px; color: var(--pos-muted); }
.spacer { flex: 1; }
.search-row { padding: 12px 2px 8px; }
.app-section-title {
  display: flex;
  justify-content: space-between;
  padding: 4px 6px 8px;
  font-size: 11.5px;
  color: var(--pos-muted);
  letter-spacing: 0.4px;
}
.app-grid {
  flex: 1;
  overflow: auto;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 6px;
  padding: 2px;
  min-height: 150px;
}
.app-tile {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 14px 6px 12px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: transparent;
  color: var(--pos-text);
  cursor: pointer;
  transition: background 0.14s ease, border-color 0.14s ease, transform 0.14s ease;
}
.app-tile:hover {
  background: rgba(139, 161, 180, 0.09);
  border-color: var(--pos-border);
  transform: translateY(-1px);
}
.tile-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.tile-label { font-size: 11.5px; text-align: center; line-height: 1.3; }
.empty { grid-column: 1 / -1; text-align: center; color: var(--pos-muted); padding: 30px; }
.start-foot {
  display: flex;
  align-items: center;
  gap: 4px;
  border-top: 1px solid var(--pos-border);
  margin-top: 10px;
  padding-top: 10px;
}
.foot-action {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  border: none;
  background: transparent;
  color: var(--pos-muted);
  font-size: 12px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
}
.foot-action:hover { background: rgba(139, 161, 180, 0.1); color: var(--pos-text); }
.power {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  border: 1px solid var(--pos-border);
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.power:hover { background: rgba(248, 113, 113, 0.12); color: var(--pos-danger); border-color: rgba(248,113,113,.35); }
.start-fade-enter-active, .start-fade-leave-active { transition: opacity 0.14s ease, transform 0.14s ease; }
.start-fade-enter-from, .start-fade-leave-to { opacity: 0; transform: translateY(8px); }
@media (max-width: 900px) {
  .start-menu { left: 8px; width: calc(100vw - 16px); bottom: 62px; padding: 12px; }
  .app-grid { grid-template-columns: repeat(4, 1fr); }
  .tile-icon { width: 38px; height: 38px; }
}
</style>
