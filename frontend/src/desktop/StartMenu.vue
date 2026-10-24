<template>
  <transition name="start-pop">
    <div class="StartMenu" @click.stop>
      <header class="StartMenu-header">
        <span class="Session-avatar">{{ initial }}</span>
        <div class="who">
          <strong>{{ auth.displayName }}</strong>
          <span>{{ auth.isAdmin ? t('users.admin') : t('users.user') }} · {{ auth.editionName }}</span>
        </div>
        <div class="spacer" />
        <span class="edition">{{ auth.pro ? 'PRO' : 'FREE' }}</span>
      </header>

      <div class="StartMenu-search" role="search">
        <el-input
          ref="searchRef"
          v-model="query"
          :placeholder="t('desktop.searchApps')"
          :prefix-icon="Search"
          clearable
          size="default"
        />
      </div>

      <div class="StartMenu-body">
        <section v-for="group in filteredGroups" :key="group.key" class="StartMenu-group">
          <div class="StartMenu-group-title">{{ t(group.labelKey) }}</div>
          <div class="StartMenu-grid">
            <button
              v-for="app in group.apps"
              :key="app.key"
              type="button"
              class="StartMenu-tile"
              @click="emitLaunch(app)"
            >
              <span class="StartMenu-icon" :style="{ color: app.color }">
                <el-icon :size="20"><component :is="app.icon" /></el-icon>
              </span>
              <span class="StartMenu-label">{{ t(app.titleKey) }}</span>
              <span
                v-if="auth.features[app.feature] === false"
                class="StatusDot disabled"
                :title="t('common.disabled')"
              />
            </button>
          </div>
        </section>
        <div v-if="!filteredGroups.length" class="StartMenu-empty">{{ t('search.empty') }}</div>
      </div>

      <footer class="StartMenu-footer">
        <button type="button" class="foot-btn" :title="t('nav.theme')" @click="ui.toggleTheme">
          <el-icon :size="14"><component :is="ui.theme === 'dark' ? 'Sunny' : 'Moon'" /></el-icon>
          <span>{{ ui.theme === 'dark' ? t('settings.general.light') : t('settings.general.dark') }}</span>
        </button>
        <button type="button" class="foot-btn" @click="ui.toggleLang">
          <span class="lang-glyph">文</span>{{ ui.lang === 'zh-CN' ? 'English' : '中文' }}
        </button>
        <a class="foot-btn" href="https://github.com/likeweixue/privacyos" target="_blank" rel="noopener noreferrer">
          <el-icon :size="14"><QuestionFilled /></el-icon>
          <span>{{ t('nav.help') }}</span>
        </a>
        <div class="spacer" />
        <span class="hint">{{ t('desktop.newWindowHint') }}</span>
        <button type="button" class="foot-btn power" :title="t('common.logout')" @click="$emit('logout')">
          <el-icon :size="15"><SwitchButton /></el-icon>
        </button>
      </footer>

      <button type="button" class="StartMenu-close" :title="t('common.close')" @click="store.startOpen = false">
        <el-icon :size="13"><Close /></el-icon>
      </button>
    </div>
  </transition>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Close, QuestionFilled, Search, SwitchButton } from '@element-plus/icons-vue'
import { useAuthStore } from '../stores/auth'
import { useDesktopStore } from '../stores/desktop'
import { useUiStore } from '../stores/ui'

const props = defineProps({
  groups: { type: Array, default: () => [] }
})
const emit = defineEmits(['launch', 'logout'])
const { t } = useI18n()
const auth = useAuthStore()
const store = useDesktopStore()
const ui = useUiStore()
const query = ref('')
const searchRef = ref(null)

const initial = computed(() => (auth.displayName || 'P').slice(0, 1).toUpperCase())
const filteredGroups = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return props.groups
  return props.groups.map((group) => ({
    ...group,
    apps: group.apps.filter((app) => t(app.titleKey).toLowerCase().includes(q) || app.key.includes(q))
  })).filter((group) => group.apps.length)
})

function emitLaunch(app) {
  emit('launch', app.key)
}

function focusSearch() {
  nextTick(() => searchRef.value?.focus())
}

watch(() => store.startSearchNonce, focusSearch)
onMounted(focusSearch)
</script>

<style scoped>
.StartMenu {
  position: fixed;
  left: 10px;
  bottom: 58px;
  width: 640px;
  max-width: calc(100vw - 20px);
  max-height: min(650px, calc(100vh - 78px));
  z-index: 6000;
  display: flex;
  flex-direction: column;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border-strong);
  border-radius: var(--pos-window-radius, 0px);
  box-shadow: 0 22px 60px rgba(20, 32, 44, 0.22);
  overflow: hidden;
}
.StartMenu-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--pos-border);
}
.Session-avatar {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--pos-accent);
  color: #fff;
  font-size: 12px;
  font-weight: 650;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.who { display: flex; flex-direction: column; gap: 2px; }
.who strong { font-size: 13px; }
.who span { font-size: 10.5px; color: var(--pos-muted); }
.spacer { flex: 1; }
.edition {
  font-size: 9.5px;
  letter-spacing: 0.7px;
  color: var(--pos-muted);
  border: 1px solid var(--pos-border);
  padding: 2px 6px;
}
.StartMenu-search { padding: 12px 14px 8px; }
.StartMenu-body { flex: 1; overflow: auto; padding: 2px 14px 12px; min-height: 0; }
.StartMenu-group { margin-bottom: 10px; }
.StartMenu-group-title {
  padding: 5px 4px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.7px;
  text-transform: uppercase;
  color: var(--pos-muted);
}
.StartMenu-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 4px;
}
.StartMenu-tile {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  padding: 12px 4px 10px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  cursor: pointer;
  text-align: center;
}
.StartMenu-tile:hover {
  background: var(--pos-panel-2);
  border-color: var(--pos-border);
}
.StartMenu-icon {
  width: 38px;
  height: 38px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  background: var(--pos-panel);
}
.StartMenu-label { font-size: 11px; line-height: 1.3; }
.StatusDot {
  position: absolute;
  top: 10px;
  right: 10px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--pos-success);
}
.StatusDot.disabled { background: var(--pos-muted); opacity: 0.55; }
.StartMenu-empty { padding: 28px 0; text-align: center; color: var(--pos-muted); font-size: 12px; }
.StartMenu-footer {
  display: flex;
  align-items: center;
  gap: 3px;
  padding: 8px 10px;
  border-top: 1px solid var(--pos-border);
  background: var(--pos-panel);
}
.foot-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 27px;
  padding: 0 7px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-muted);
  font-size: 10.5px;
  cursor: pointer;
  text-decoration: none;
}
.foot-btn:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.lang-glyph { font-size: 11px; }
.hint { font-size: 10px; color: var(--pos-muted); margin-right: 4px; }
.power:hover { color: var(--pos-danger); background: color-mix(in srgb, var(--pos-danger) 8%, transparent); }
.StartMenu-close {
  position: absolute;
  right: 8px;
  top: 8px;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-muted);
  cursor: pointer;
}
.StartMenu-close:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.start-pop-enter-active, .start-pop-leave-active { transition: opacity 0.12s ease, transform 0.12s ease; }
.start-pop-enter-from, .start-pop-leave-to { opacity: 0; transform: translateY(6px); }
@media (max-width: 900px) {
  .StartMenu { left: 6px; width: calc(100vw - 12px); bottom: 56px; }
  .StartMenu-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .hint { display: none; }
}
@media (max-width: 460px) {
  .StartMenu-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
</style>
