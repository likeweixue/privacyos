import { defineAsyncComponent } from 'vue'

// Every desktop application maps to a router route and a lazily loaded view.
// `desktop` controls whether it appears as a desktop icon; all permitted apps
// are available from the Start menu.
export const apps = {
  dashboard: {
    key: 'dashboard',
    routeName: 'dashboard',
    titleKey: 'nav.dashboard',
    icon: 'Odometer',
    color: '#2dd4bf',
    width: 1200,
    height: 800,
    desktop: true,
    group: 'main',
    component: defineAsyncComponent(() => import('../views/Dashboard.vue'))
  },
  files: {
    key: 'files',
    routeName: 'files',
    titleKey: 'nav.files',
    icon: 'FolderOpened',
    color: '#fbbf24',
    width: 1240,
    height: 790,
    desktop: true,
    feature: 'files',
    group: 'main',
    component: defineAsyncComponent(() => import('../views/Files.vue'))
  },
  photos: {
    key: 'photos',
    routeName: 'photos',
    titleKey: 'nav.photos',
    icon: 'Picture',
    color: '#2dd4bf',
    width: 1200,
    height: 790,
    desktop: true,
    feature: 'photos',
    group: 'media',
    component: defineAsyncComponent(() => import('../views/Photos.vue'))
  },
  movies: {
    key: 'movies',
    routeName: 'movies',
    titleKey: 'nav.movies',
    icon: 'VideoCamera',
    color: '#60a5fa',
    width: 1200,
    height: 790,
    desktop: true,
    feature: 'movies',
    group: 'media',
    component: defineAsyncComponent(() => import('../views/Movies.vue'))
  },
  docker: {
    key: 'docker',
    routeName: 'docker',
    titleKey: 'nav.docker',
    icon: 'Box',
    color: '#38bdf8',
    width: 1140,
    height: 740,
    desktop: true,
    feature: 'docker',
    group: 'system',
    component: defineAsyncComponent(() => import('../views/Docker.vue'))
  },
  monitor: {
    key: 'monitor',
    routeName: 'monitor',
    titleKey: 'nav.monitor',
    icon: 'Monitor',
    color: '#34d399',
    width: 1180,
    height: 790,
    desktop: true,
    group: 'system',
    component: defineAsyncComponent(() => import('../views/Monitor.vue'))
  },
  apps: {
    key: 'apps',
    routeName: 'apps',
    titleKey: 'nav.apps',
    icon: 'Grid',
    color: '#f472b6',
    width: 1140,
    height: 740,
    desktop: true,
    admin: true,
    group: 'system',
    component: defineAsyncComponent(() => import('../views/Apps.vue'))
  },
  users: {
    key: 'users',
    routeName: 'users',
    titleKey: 'nav.users',
    icon: 'User',
    color: '#c084fc',
    width: 1140,
    height: 740,
    desktop: true,
    admin: true,
    group: 'admin',
    component: defineAsyncComponent(() => import('../views/Users.vue'))
  },
  storage: {
    key: 'storage',
    routeName: 'storage',
    titleKey: 'nav.storage',
    icon: 'Coin',
    color: '#a78bfa',
    width: 1180,
    height: 760,
    desktop: true,
    admin: true,
    group: 'admin',
    component: defineAsyncComponent(() => import('../views/Storage.vue'))
  },
  network: {
    key: 'network',
    routeName: 'network',
    titleKey: 'nav.network',
    icon: 'Connection',
    color: '#34d399',
    width: 1160,
    height: 760,
    desktop: true,
    admin: true,
    group: 'admin',
    component: defineAsyncComponent(() => import('../views/Network.vue'))
  },
  logs: {
    key: 'logs',
    routeName: 'logs',
    titleKey: 'nav.logs',
    icon: 'Document',
    color: '#94a3b8',
    width: 1160,
    height: 740,
    desktop: true,
    admin: true,
    group: 'admin',
    component: defineAsyncComponent(() => import('../views/Logs.vue'))
  },
  settings: {
    key: 'settings',
    routeName: 'settings',
    titleKey: 'nav.settings',
    icon: 'Setting',
    color: '#94a3b8',
    width: 1200,
    height: 800,
    desktop: true,
    admin: true,
    group: 'admin',
    component: defineAsyncComponent(() => import('../views/Settings.vue'))
  }
}

export const appList = Object.values(apps)

export const desktopOrder = [
  'files', 'photos', 'movies', 'docker', 'monitor', 'dashboard',
  'storage', 'network', 'users', 'apps', 'logs', 'settings'
]

export function allowedApps(auth) {
  return appList.filter((app) => {
    if (app.admin && !auth.isAdmin) return false
    if (app.feature && !auth.isAdmin && auth.features[app.feature] === false) return false
    return true
  })
}

export function appByRouteName(name) {
  return appList.find((app) => app.routeName === name) || null
}
