// Central registry for desktop icons, Start menu, windows and dashboard modules.
import { defineAsyncComponent } from 'vue'

export const apps = {
  dashboard: {
    key: 'dashboard', titleKey: 'nav.dashboard', icon: 'Odometer', color: '#0d9488',
    group: 'core', admin: false, desktop: true,
    width: 1200, height: 800,
    component: defineAsyncComponent(() => import('./views/Dashboard.vue'))
  },
  files: {
    key: 'files', titleKey: 'nav.files', icon: 'FolderOpened', color: '#b7791f',
    group: 'core', feature: 'files', desktop: true,
    width: 1240, height: 790,
    component: defineAsyncComponent(() => import('./views/Files.vue'))
  },
  photos: {
    key: 'photos', titleKey: 'nav.photos', icon: 'Picture', color: '#0d9488',
    group: 'core', feature: 'photos', desktop: true,
    width: 1200, height: 790,
    component: defineAsyncComponent(() => import('./views/Photos.vue'))
  },
  movies: {
    key: 'movies', titleKey: 'nav.movies', icon: 'VideoCamera', color: '#2563eb',
    group: 'core', feature: 'movies', desktop: true,
    width: 1200, height: 790,
    component: defineAsyncComponent(() => import('./views/Movies.vue'))
  },
  monitor: {
    key: 'monitor', titleKey: 'nav.monitor', icon: 'Monitor', color: '#0d9488',
    group: 'core', desktop: true,
    width: 1180, height: 790,
    component: defineAsyncComponent(() => import('./views/Monitor.vue'))
  },
  users: {
    key: 'users', titleKey: 'nav.users', icon: 'User', color: '#7c3aed',
    group: 'system', admin: true, desktop: true,
    width: 1140, height: 740,
    component: defineAsyncComponent(() => import('./views/Users.vue'))
  },
  storage: {
    key: 'storage', titleKey: 'nav.storage', icon: 'Coin', color: '#7c3aed',
    group: 'system', admin: true, desktop: true,
    width: 1180, height: 760,
    component: defineAsyncComponent(() => import('./views/Storage.vue'))
  },
  network: {
    key: 'network', titleKey: 'nav.network', icon: 'Connection', color: '#0d9488',
    group: 'system', admin: true, desktop: true,
    width: 1160, height: 760,
    component: defineAsyncComponent(() => import('./views/Network.vue'))
  },
  docker: {
    key: 'docker', titleKey: 'nav.docker', icon: 'Box', color: '#0284c7',
    group: 'features', feature: 'docker', desktop: true,
    width: 1140, height: 740,
    component: defineAsyncComponent(() => import('./views/Docker.vue'))
  },
  apps: {
    key: 'apps', titleKey: 'nav.apps', icon: 'Grid', color: '#db2777',
    group: 'features', admin: true, desktop: true,
    width: 1140, height: 740,
    component: defineAsyncComponent(() => import('./views/Apps.vue'))
  },
  logs: {
    key: 'logs', titleKey: 'nav.logs', icon: 'Document', color: '#64748b',
    group: 'system', admin: true, desktop: true,
    width: 1160, height: 740,
    component: defineAsyncComponent(() => import('./views/Logs.vue'))
  },
  settings: {
    key: 'settings', titleKey: 'nav.settings', icon: 'Setting', color: '#64748b',
    group: 'system', admin: true, desktop: true,
    width: 1200, height: 800,
    component: defineAsyncComponent(() => import('./views/Settings.vue'))
  }
}

export const navGroups = [
  { key: 'core', labelKey: 'nav.groups.core', apps: ['dashboard', 'files', 'photos', 'movies', 'monitor'] },
  { key: 'features', labelKey: 'nav.groups.features', apps: ['docker', 'apps', 'network'] },
  { key: 'system', labelKey: 'nav.groups.system', apps: ['users', 'storage', 'logs', 'settings'] }
]

export const appList = Object.values(apps)

export function visibleApps(auth) {
  return appList.filter((app) => {
    if (app.admin && !auth.isAdmin) return false
    if (app.feature && !auth.isAdmin && auth.features[app.feature] === false) return false
    return true
  })
}

export function visibleNavGroups(auth) {
  const allowed = new Set(visibleApps(auth).map((app) => app.key))
  return navGroups
    .map((group) => ({ ...group, apps: group.apps.map((key) => apps[key]).filter((app) => allowed.has(app.key)) }))
    .filter((group) => group.apps.length)
}

export const desktopApps = ['files', 'photos', 'movies', 'docker', 'monitor', 'dashboard', 'network', 'storage', 'users', 'apps', 'logs', 'settings']
