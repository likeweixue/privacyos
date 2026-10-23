<template>
  <div class="App">
    <a class="sr-only-focusable sr-only-focusable-custom" href="#content">{{ t('nav.skip') }}</a>

    <header id="header" class="App-header navbar-fixed-top">
      <div class="Header-navigation">
        <div id="app-navigation" class="App-navigation">
          <button
            type="button"
            class="Button Button--icon NavToggle"
            :aria-label="t('nav.openMenu')"
            aria-controls="admin-navigation"
            :aria-expanded="drawerOpen"
            @click="drawerOpen = !drawerOpen"
          >
            <span class="burger" aria-hidden="true"><i /><i /><i /></span>
          </button>
        </div>
        <button
          v-if="route.name !== 'dashboard'"
          type="button"
          class="Button Button--icon BackButton"
          :aria-label="t('nav.back')"
          @click="goBack"
        >
          <el-icon :size="16"><ArrowLeft /></el-icon>
        </button>
        <router-link class="Header-title" :to="{ name: 'dashboard' }">
          <img src="/logo.svg" alt="" />
          <span>PrivacyOS</span>
          <em>Admin</em>
        </router-link>
      </div>

      <ul id="header-primary" class="Header-primary"></ul>

      <ul id="header-secondary" class="Header-secondary">
        <li class="item-search">
          <button
            type="button"
            class="Button Button--icon"
            :aria-label="t('search.open')"
            aria-haspopup="dialog"
            @click="openSearch"
          >
            <el-icon :size="16"><Search /></el-icon>
          </button>
        </li>
        <li class="item-help">
          <a
            class="Button Button--icon"
            href="https://github.com/likeweixue/privacyos"
            target="_blank"
            rel="noopener noreferrer"
            :aria-label="t('nav.help')"
            :title="t('nav.help')"
          >
            <el-icon :size="16"><QuestionFilled /></el-icon>
          </a>
        </li>
        <li class="item-session">
          <el-dropdown trigger="click" @command="onSessionCommand">
            <button type="button" class="Session SessionDropdown" aria-haspopup="menu">
              <span class="Session-avatar">{{ initial }}</span>
              <span class="Session-name">{{ auth.displayName }}</span>
              <span class="Session-role">{{ auth.isAdmin ? t('users.admin') : t('users.user') }}</span>
              <el-icon :size="12"><ArrowDown /></el-icon>
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  {{ auth.displayName }} · {{ auth.editionName }}
                </el-dropdown-item>
                <el-dropdown-item v-if="auth.isAdmin" command="settings" divided>
                  <el-icon><Setting /></el-icon>{{ t('nav.settings') }}
                </el-dropdown-item>
                <el-dropdown-item command="password" :divided="!auth.isAdmin">
                  <el-icon><Lock /></el-icon>{{ t('settings.account.changePassword') }}
                </el-dropdown-item>
                <el-dropdown-item command="logout" divided>
                  <el-icon><SwitchButton /></el-icon>{{ t('common.logout') }}
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </li>
      </ul>
    </header>

    <div v-if="drawerOpen" class="drawer-backdrop" @click="drawerOpen = false" />

    <div id="drawer" class="App-drawer">
      <aside
        id="admin-navigation"
        class="App-nav sideNav"
        :class="{ open: drawerOpen }"
        :aria-label="t('nav.admin')"
      >
        <div class="AdminNav Dropdown--select" :class="{ 'selector-open': navOpen }">
          <div class="nav-head">
            <div class="nav-head-text">
              <strong>{{ t('nav.admin') }}</strong>
              <span>{{ auth.editionName }} · v{{ auth.version }}</span>
            </div>
            <span class="nav-edition">{{ auth.pro ? 'PRO' : 'FREE' }}</span>
          </div>

          <div class="NavSelector-wrap" @click.stop>
            <button
              type="button"
              class="NavSelector Dropdown-toggle"
              aria-haspopup="listbox"
              :aria-expanded="navOpen"
              @click="navOpen = !navOpen"
            >
              <el-icon :size="15"><Grid /></el-icon>
              <span>{{ t('nav.select') }}</span>
              <el-icon :size="12" class="NavSelector-arrow"><ArrowDown /></el-icon>
            </button>
            <transition name="dropdown">
              <div v-if="navOpen" class="Dropdown-menu NavSelector-panel">
                <div class="NavSelector-search" role="search">
                  <el-input
                    ref="navSearchRef"
                    v-model="navQuery"
                    :placeholder="t('nav.searchPlaceholder')"
                    :prefix-icon="Search"
                    clearable
                    size="small"
                  />
                </div>
                <div class="NavSelector-list" role="listbox" :aria-label="t('nav.select')">
                  <button
                    v-for="app in filteredNavApps"
                    :key="app.key"
                    type="button"
                    class="NavSelector-item"
                    role="option"
                    :aria-selected="route.name === app.key"
                    @click="openApp(app)"
                  >
                    <span class="nav-icon"><el-icon :size="15"><component :is="app.icon" /></el-icon></span>
                    <span class="NavSelector-label">{{ t(app.titleKey) }}</span>
                    <span class="NavSelector-group">{{ t(groupLabel(app.group)) }}</span>
                  </button>
                  <div v-if="!filteredNavApps.length" class="NavSelector-empty">{{ t('search.empty') }}</div>
                </div>
              </div>
            </transition>
          </div>

          <nav class="nav-groups" :aria-label="t('nav.admin')">
            <div v-for="group in groups" :key="group.key" class="nav-group">
              <div class="nav-group-title">{{ t(group.labelKey) }}</div>
              <router-link
                v-for="app in group.apps"
                :key="app.key"
                :to="{ name: app.key }"
                class="nav-item"
                :class="{ active: route.name === app.key }"
                :aria-current="route.name === app.key ? 'page' : undefined"
              >
                <span class="nav-icon"><el-icon :size="16"><component :is="app.icon" /></el-icon></span>
                <span class="nav-item-label">{{ t(app.titleKey) }}</span>
                <span
                  v-if="app.feature"
                  class="StatusDot"
                  :class="auth.features[app.feature] === false ? 'disabled' : 'enabled'"
                  :title="auth.features[app.feature] === false ? t('common.disabled') : t('common.enabled')"
                  aria-hidden="true"
                />
              </router-link>
            </div>
          </nav>

          <div class="nav-foot">
            <button type="button" class="nav-foot-button" :title="t('nav.theme')" @click="ui.toggleTheme">
              <el-icon :size="14"><component :is="ui.theme === 'dark' ? 'Sunny' : 'Moon'" /></el-icon>
            </button>
            <button type="button" class="nav-foot-button" :title="t('nav.language')" @click="ui.toggleLang">
              {{ ui.lang === 'zh-CN' ? '中' : 'EN' }}
            </button>
            <span class="nav-foot-version">PrivacyOS {{ auth.version }}</span>
          </div>
        </div>
      </aside>
    </div>

    <main id="content" class="App-content sideNavOffset" tabindex="-1">
      <div class="container" :class="{ 'container-wide': route.name === 'files' }">
        <router-view v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <component :is="Component" :key="route.name" />
          </transition>
        </router-view>
      </div>
    </main>

    <transition name="fade">
      <div
        v-if="searchOpen"
        class="CommandPalette"
        role="dialog"
        aria-modal="true"
        :aria-label="t('search.title')"
        @click.self="searchOpen = false"
      >
        <div class="CommandPalette-panel">
          <div class="CommandPalette-input" role="search">
            <el-icon :size="16"><Search /></el-icon>
            <input
              ref="searchInput"
              v-model="searchQuery"
              type="search"
              :placeholder="t('search.placeholder')"
              :aria-label="t('search.placeholder')"
              @keydown.down.prevent="moveSearch(1)"
              @keydown.up.prevent="moveSearch(-1)"
              @keydown.enter.prevent="openActiveSearch"
              @keydown.esc.prevent="searchOpen = false"
            />
            <kbd>esc</kbd>
          </div>
          <div class="CommandPalette-list">
            <button
              v-for="(app, index) in searchResults"
              :key="app.key"
              type="button"
              class="CommandPalette-item"
              :class="{ active: index === searchIndex }"
              @mouseenter="searchIndex = index"
              @click="openApp(app)"
            >
              <span class="nav-icon"><el-icon :size="16"><component :is="app.icon" /></el-icon></span>
              <span>{{ t(app.titleKey) }}</span>
              <span class="CommandPalette-group">{{ t(groupLabel(app.group)) }}</span>
              <el-icon class="CommandPalette-enter"><Right /></el-icon>
            </button>
            <div v-if="!searchResults.length" class="CommandPalette-empty">{{ t('search.empty') }}</div>
          </div>
          <div class="CommandPalette-foot">
            <span>↑↓ {{ t('search.navigate') }}</span>
            <span>↵ {{ t('search.open') }}</span>
            <span class="spacer" />
            <span>{{ t('search.hint') }}</span>
          </div>
        </div>
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
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowDown, ArrowLeft, Grid, Lock, QuestionFilled, Right, Search,
  Setting, SwitchButton
} from '@element-plus/icons-vue'
import { visibleApps, visibleNavGroups } from '../apps'
import { api, errorMessage } from '../api'
import { useAuthStore } from '../stores/auth'
import { useUiStore } from '../stores/ui'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const ui = useUiStore()

const drawerOpen = ref(false)
const navOpen = ref(false)
const navQuery = ref('')
const navSearchRef = ref(null)
const searchOpen = ref(false)
const searchQuery = ref('')
const searchIndex = ref(0)
const searchInput = ref(null)
const passwordOpen = ref(false)
const busy = ref(false)
const passwordForm = reactive({ old: '', next: '', confirm: '' })

const allowed = computed(() => visibleApps(auth))
const groups = computed(() => visibleNavGroups(auth))
const initial = computed(() => (auth.displayName || 'P').slice(0, 1).toUpperCase())

const filteredNavApps = computed(() => {
  const q = navQuery.value.trim().toLowerCase()
  if (!q) return allowed.value
  return allowed.value.filter((app) => {
    return t(app.titleKey).toLowerCase().includes(q) || app.key.includes(q)
  })
})

const searchResults = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  const list = allowed.value
  if (!q) return list
  return list.filter((app) => t(app.titleKey).toLowerCase().includes(q) || app.key.includes(q))
})

function groupLabel(key) {
  return { core: 'nav.groups.core', features: 'nav.groups.features', system: 'nav.groups.system' }[key] || 'nav.admin'
}

function openApp(app) {
  navOpen.value = false
  drawerOpen.value = false
  searchOpen.value = false
  navQuery.value = ''
  searchQuery.value = ''
  if (route.name !== app.key) router.push({ name: app.key })
}

function goBack() {
  if (window.history.length > 1) router.back()
  else router.push({ name: 'dashboard' })
}

function openSearch() {
  searchOpen.value = true
  searchQuery.value = ''
  searchIndex.value = 0
  nextTick(() => searchInput.value?.focus())
}

function moveSearch(delta) {
  if (!searchResults.value.length) return
  searchIndex.value = (searchIndex.value + delta + searchResults.value.length) % searchResults.value.length
}

function openActiveSearch() {
  const app = searchResults.value[searchIndex.value]
  if (app) openApp(app)
}

function onSessionCommand(command) {
  if (command === 'settings') router.push({ name: 'settings' })
  else if (command === 'password') passwordOpen.value = true
  else if (command === 'logout') logout()
}

async function logout() {
  try {
    await ElMessageBox.confirm(t('nav.logoutConfirm'), t('common.logout'), {
      type: 'warning',
      confirmButtonText: t('common.confirm'),
      cancelButtonText: t('common.cancel')
    })
  } catch (error) {
    return
  }
  try {
    await auth.logout()
  } finally {
    router.replace({ name: 'login' })
  }
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

function onKeydown(event) {
  if (event.key === 'Escape') {
    navOpen.value = false
    drawerOpen.value = false
    searchOpen.value = false
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    openSearch()
  }
}

function onDocumentClick(event) {
  if (!event.target.closest('.NavSelector-wrap')) navOpen.value = false
}

watch(navOpen, (open) => {
  if (open) nextTick(() => navSearchRef.value?.focus())
})

watch(() => route.fullPath, () => {
  drawerOpen.value = false
  navOpen.value = false
  searchOpen.value = false
  navQuery.value = ''
  searchQuery.value = ''
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  document.addEventListener('click', onDocumentClick)
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.removeEventListener('click', onDocumentClick)
})
</script>

<style scoped>
.App {
  min-height: 100vh;
  background: var(--pos-bg);
}
.sr-only-focusable {
  position: absolute;
  left: -9999px;
  top: 0;
  z-index: 3000;
  padding: 10px 14px;
  background: var(--pos-panel);
  color: var(--pos-text);
  border: 1px solid var(--pos-border);
  font-size: 13px;
}
.sr-only-focusable:focus {
  left: 10px;
  top: 10px;
}

.App-header {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 56px;
  z-index: 1000;
  display: flex;
  align-items: center;
  background: var(--pos-header);
  border-bottom: 1px solid var(--pos-border);
}
.Header-navigation {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 100%;
  padding-left: 10px;
}
.Header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 100%;
  padding: 0 8px;
  color: var(--pos-text);
  font-size: 14.5px;
  font-weight: 650;
  letter-spacing: 0.2px;
}
.Header-title img { width: 26px; height: 26px; }
.Header-title em {
  font-style: normal;
  font-size: 10px;
  font-weight: 500;
  letter-spacing: 0.8px;
  text-transform: uppercase;
  color: var(--pos-muted);
  border-left: 1px solid var(--pos-border);
  padding-left: 8px;
  margin-left: 2px;
}
.Header-primary { flex: 1; margin: 0; padding: 0; list-style: none; }
.Header-secondary {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 100%;
  margin: 0;
  padding: 0 8px;
  list-style: none;
}
.Header-secondary li { display: flex; align-items: center; height: 100%; }

.Button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 32px;
  padding: 0 10px;
  border: 1px solid var(--pos-border);
  border-radius: 3px;
  background: var(--pos-panel);
  color: var(--pos-text);
  font-size: 12.5px;
  cursor: pointer;
  transition: background 0.12s ease, border-color 0.12s ease, color 0.12s ease;
}
.Button:hover { background: var(--pos-panel-2); border-color: var(--pos-border-strong); }
.Button--icon { width: 32px; padding: 0; border-color: transparent; background: transparent; color: var(--pos-muted); }
.Button--icon:hover { color: var(--pos-text); background: var(--pos-panel-2); border-color: transparent; }
.NavToggle { display: none; }
.burger {
  width: 15px;
  display: inline-flex;
  flex-direction: column;
  gap: 3px;
}
.burger i {
  display: block;
  height: 1.5px;
  border-radius: 1px;
  background: currentColor;
}
.BackButton { border-color: transparent; background: transparent; color: var(--pos-muted); }

.Session {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 8px;
  border: 1px solid transparent;
  border-radius: 3px;
  background: transparent;
  color: var(--pos-text);
  cursor: pointer;
}
.Session:hover { background: var(--pos-panel-2); }
.Session-avatar {
  width: 26px;
  height: 26px;
  border-radius: 50%;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--pos-accent);
  color: #fff;
  font-size: 11px;
  font-weight: 650;
}
.Session-name { font-size: 12.5px; font-weight: 550; }
.Session-role { font-size: 10.5px; color: var(--pos-muted); }

.App-nav {
  position: fixed;
  top: 56px;
  bottom: 0;
  left: 0;
  width: 240px;
  z-index: 900;
  background: var(--pos-nav);
  border-right: 1px solid var(--pos-border);
}
.AdminNav {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.nav-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 14px 10px;
}
.nav-head-text { display: flex; flex-direction: column; gap: 2px; }
.nav-head-text strong { font-size: 13px; font-weight: 650; }
.nav-head-text span { font-size: 10.5px; color: var(--pos-muted); }
.nav-edition {
  font-size: 9.5px;
  letter-spacing: 0.7px;
  color: var(--pos-muted);
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  padding: 2px 5px;
}
.NavSelector-wrap { position: relative; padding: 0 12px 12px; }
.NavSelector {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 34px;
  padding: 0 9px;
  border: 1px solid var(--pos-border);
  border-radius: 3px;
  background: var(--pos-panel);
  color: var(--pos-text);
  font-size: 12.5px;
  cursor: pointer;
  text-align: left;
}
.NavSelector:hover { border-color: var(--pos-border-strong); }
.NavSelector-arrow { margin-left: auto; color: var(--pos-muted); }
.NavSelector-panel {
  position: absolute;
  left: 12px;
  right: 12px;
  top: 46px;
  z-index: 50;
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  box-shadow: 0 14px 34px rgba(25, 39, 52, 0.14);
  display: flex;
  flex-direction: column;
  max-height: min(540px, calc(100vh - 220px));
  overflow: hidden;
}
.NavSelector-search { padding: 8px; border-bottom: 1px solid var(--pos-border); }
.NavSelector-list { overflow: auto; min-height: 0; padding: 5px; }
.NavSelector-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 9px;
  border: none;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  font-size: 12.5px;
  cursor: pointer;
  text-align: left;
}
.NavSelector-item:hover, .NavSelector-item[aria-selected='true'] { background: var(--pos-accent-soft); color: var(--pos-accent); }
.NavSelector-label { flex: 1; }
.NavSelector-group { font-size: 10px; color: var(--pos-muted); }
.NavSelector-empty { padding: 18px 8px; text-align: center; color: var(--pos-muted); font-size: 12px; }
.nav-groups { flex: 1; overflow: auto; padding: 0 8px 12px; transition: visibility 0.1s ease; }
.AdminNav.selector-open .nav-groups { visibility: hidden; }
.nav-group { margin-bottom: 12px; }
.nav-group-title {
  padding: 6px 10px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.8px;
  text-transform: uppercase;
  color: var(--pos-muted);
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 8px 10px;
  border-left: 2px solid transparent;
  color: var(--pos-text-2);
  font-size: 12.5px;
  text-decoration: none;
}
.nav-item:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.nav-item.active {
  background: var(--pos-accent-soft);
  border-left-color: var(--pos-accent);
  color: var(--pos-accent);
  font-weight: 600;
}
.nav-icon { display: inline-flex; width: 18px; color: var(--pos-muted); }
.nav-item.active .nav-icon { color: var(--pos-accent); }
.nav-item-label { flex: 1; }
.StatusDot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--pos-success);
}
.StatusDot.disabled { background: var(--pos-muted); opacity: 0.55; }
.nav-foot {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 9px 12px;
  border-top: 1px solid var(--pos-border);
}
.nav-foot-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 26px;
  padding: 0 6px;
  border: 1px solid transparent;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-muted);
  font-size: 11px;
  cursor: pointer;
}
.nav-foot-button:hover { background: var(--pos-panel-2); color: var(--pos-text); }
.nav-foot-version { margin-left: auto; font-size: 10px; color: var(--pos-muted); }

.App-content {
  margin-left: 240px;
  padding-top: 56px;
  min-height: 100vh;
  background: var(--pos-bg);
}
.container {
  max-width: 1280px;
  margin: 0 auto;
  padding: 22px 26px 28px;
}
.container-wide { max-width: 1560px; }
.drawer-backdrop { display: none; }

.CommandPalette {
  position: fixed;
  inset: 0;
  z-index: 2500;
  background: rgba(15, 23, 32, 0.38);
  display: flex;
  justify-content: center;
  align-items: flex-start;
  padding-top: 12vh;
}
.CommandPalette-panel {
  width: 590px;
  max-width: calc(100vw - 32px);
  background: var(--pos-panel);
  border: 1px solid var(--pos-border);
  box-shadow: 0 24px 70px rgba(15, 23, 32, 0.28);
  display: flex;
  flex-direction: column;
  max-height: 66vh;
}
.CommandPalette-input {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--pos-border);
  color: var(--pos-muted);
}
.CommandPalette-input input {
  flex: 1;
  border: none;
  outline: none;
  background: transparent;
  color: var(--pos-text);
  font-size: 14px;
  font-family: inherit;
}
.CommandPalette-input kbd {
  font-family: inherit;
  font-size: 10px;
  color: var(--pos-muted);
  border: 1px solid var(--pos-border);
  border-radius: 2px;
  padding: 2px 5px;
}
.CommandPalette-list { overflow: auto; padding: 5px; }
.CommandPalette-item {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 9px 10px;
  border: none;
  border-radius: 2px;
  background: transparent;
  color: var(--pos-text);
  font-size: 13px;
  cursor: pointer;
  text-align: left;
}
.CommandPalette-item.active { background: var(--pos-accent-soft); color: var(--pos-accent); }
.CommandPalette-group { margin-left: auto; font-size: 10.5px; color: var(--pos-muted); }
.CommandPalette-enter { color: var(--pos-muted); }
.CommandPalette-empty { padding: 28px 10px; text-align: center; color: var(--pos-muted); font-size: 12.5px; }
.CommandPalette-foot {
  display: flex;
  gap: 16px;
  padding: 8px 12px;
  border-top: 1px solid var(--pos-border);
  color: var(--pos-muted);
  font-size: 10.5px;
}
.CommandPalette-foot .spacer { flex: 1; }
.fade-enter-active, .fade-leave-active { transition: opacity 0.12s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
.dropdown-enter-active, .dropdown-leave-active { transition: opacity 0.12s ease, transform 0.12s ease; }
.dropdown-enter-from, .dropdown-leave-to { opacity: 0; transform: translateY(-3px); }
.page-enter-active, .page-leave-active { transition: opacity 0.12s ease; }
.page-enter-from, .page-leave-to { opacity: 0; }

@media (max-width: 900px) {
  .NavToggle { display: inline-flex; }
  .Header-primary { display: none; }
  .Header-title { padding-left: 2px; }
  .Header-title em { display: none; }
  .Session-name, .Session-role { display: none; }
  .App-nav {
    width: 280px;
    transform: translateX(-100%);
    transition: transform 0.18s ease;
    z-index: 950;
  }
  .App-nav.open { transform: translateX(0); box-shadow: 8px 0 30px rgba(15, 23, 32, 0.18); }
  .drawer-backdrop {
    display: block;
    position: fixed;
    inset: 56px 0 0 0;
    z-index: 940;
    background: rgba(15, 23, 32, 0.32);
  }
  .App-content { margin-left: 0; }
  .container { padding: 16px 14px 34px; }
  .BackButton { display: none; }
}
@media (max-width: 560px) {
  .Header-secondary { padding-right: 2px; }
  .CommandPalette { padding-top: 8vh; }
}
</style>
