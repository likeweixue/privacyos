// Central registry for the admin navigation and dashboard extension list.
// Views are lazy-loaded by the router; this file only holds presentation data.
export const apps = {
  dashboard: {
    key: 'dashboard', titleKey: 'nav.dashboard', icon: 'Odometer', color: '#0d9488',
    group: 'core', admin: false
  },
  files: {
    key: 'files', titleKey: 'nav.files', icon: 'FolderOpened', color: '#b7791f',
    group: 'core', feature: 'files'
  },
  photos: {
    key: 'photos', titleKey: 'nav.photos', icon: 'Picture', color: '#0d9488',
    group: 'core', feature: 'photos'
  },
  movies: {
    key: 'movies', titleKey: 'nav.movies', icon: 'VideoCamera', color: '#2563eb',
    group: 'core', feature: 'movies'
  },
  monitor: {
    key: 'monitor', titleKey: 'nav.monitor', icon: 'Monitor', color: '#0d9488',
    group: 'core'
  },
  users: {
    key: 'users', titleKey: 'nav.users', icon: 'User', color: '#7c3aed',
    group: 'system', admin: true
  },
  storage: {
    key: 'storage', titleKey: 'nav.storage', icon: 'Coin', color: '#7c3aed',
    group: 'system', admin: true
  },
  network: {
    key: 'network', titleKey: 'nav.network', icon: 'Connection', color: '#0d9488',
    group: 'system', admin: true
  },
  docker: {
    key: 'docker', titleKey: 'nav.docker', icon: 'Box', color: '#0284c7',
    group: 'features', feature: 'docker'
  },
  apps: {
    key: 'apps', titleKey: 'nav.apps', icon: 'Grid', color: '#db2777',
    group: 'features', admin: true
  },
  logs: {
    key: 'logs', titleKey: 'nav.logs', icon: 'Document', color: '#64748b',
    group: 'system', admin: true
  },
  settings: {
    key: 'settings', titleKey: 'nav.settings', icon: 'Setting', color: '#64748b',
    group: 'system', admin: true
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
