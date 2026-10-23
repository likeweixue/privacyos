import axios from 'axios'

const ACCESS_KEY = 'pos_access_token'
const REFRESH_KEY = 'pos_refresh_token'

export const tokenStore = {
  get access() {
    return localStorage.getItem(ACCESS_KEY) || ''
  },
  get refresh() {
    return localStorage.getItem(REFRESH_KEY) || ''
  },
  set(access, refresh) {
    if (access) localStorage.setItem(ACCESS_KEY, access)
    localStorage.setItem(REFRESH_KEY, refresh || '')
  },
  clear() {
    localStorage.removeItem(ACCESS_KEY)
    localStorage.removeItem(REFRESH_KEY)
  }
}

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 60000
})

http.interceptors.request.use((config) => {
  const token = tokenStore.access
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

let refreshInFlight = null

async function refreshTokens() {
  if (!refreshInFlight) {
    refreshInFlight = axios
      .post('/api/v1/auth/refresh', { refresh_token: tokenStore.refresh })
      .then((res) => {
        const payload = res.data.data
        tokenStore.set(payload.access_token, payload.refresh_token)
        return payload
      })
      .finally(() => {
        refreshInFlight = null
      })
  }
  return refreshInFlight
}

http.interceptors.response.use(
  (res) => res,
  async (error) => {
    const { response, config } = error
    const url = config?.url || ''
    const canRetry =
      response?.status === 401 &&
      !config?._retry &&
      tokenStore.refresh &&
      !url.includes('/auth/login') &&
      !url.includes('/auth/refresh')
    if (canRetry) {
      config._retry = true
      try {
        const payload = await refreshTokens()
        config.headers = config.headers || {}
        config.headers.Authorization = `Bearer ${payload.access_token}`
        return http(config)
      } catch (refreshError) {
        tokenStore.clear()
        window.dispatchEvent(new CustomEvent('pos:logout'))
        return Promise.reject(refreshError)
      }
    }
    if (response?.status === 401 && !url.includes('/auth/login')) {
      tokenStore.clear()
      window.dispatchEvent(new CustomEvent('pos:logout'))
    }
    return Promise.reject(error)
  }
)

export function errorMessage(error, fallback = '操作失败，请稍后重试') {
  const data = error?.response?.data
  if (data?.error?.message) return data.error.message
  if (typeof data?.error === 'string') return data.error
  if (error?.message === 'Network Error') return '无法连接 PrivacyOS 服务'
  return error?.message || fallback
}

export function errorPayload(error) {
  return error?.response?.data || {}
}

const unwrap = (promise) => promise.then((res) => res.data?.data)
const envelope = (promise) => promise.then((res) => res.data)

export function apiURL(path) {
  const [base, query] = path.split('?')
  const params = new URLSearchParams(query || '')
  if (tokenStore.access) params.set('access_token', tokenStore.access)
  const qs = params.toString()
  return `/api/v1${base}${qs ? `?${qs}` : ''}`
}

export const api = {
  auth: {
    login: (payload) => unwrap(http.post('/auth/login', payload)),
    me: () => unwrap(http.get('/auth/me')),
    logout: () => unwrap(http.post('/auth/logout')),
    changePassword: (payload) => unwrap(http.post('/auth/password', payload)),
    updatePreferences: (payload) => unwrap(http.patch('/auth/preferences', payload)),
    sessions: () => unwrap(http.get('/auth/sessions')),
    revokeSession: (id) => unwrap(http.delete(`/auth/sessions/${id}`)),
    totpSetup: () => unwrap(http.post('/auth/2fa/setup')),
    totpEnable: (code) => unwrap(http.post('/auth/2fa/enable', { code })),
    totpDisable: (payload) => unwrap(http.post('/auth/2fa/disable', payload))
  },
  public: {
    info: () => unwrap(http.get('/system/public'))
  },
  dashboard: {
    summary: () => unwrap(http.get('/dashboard/summary')),
    activity: (params) => unwrap(http.get('/dashboard/activity', { params }))
  },
  users: {
    list: (params) => unwrap(http.get('/users', { params })),
    deleted: () => unwrap(http.get('/users/deleted')),
    get: (id) => unwrap(http.get(`/users/${id}`)),
    create: (payload) => unwrap(http.post('/users', payload)),
    update: (id, payload) => unwrap(http.patch(`/users/${id}`, payload)),
    remove: (id) => unwrap(http.delete(`/users/${id}`)),
    restore: (id) => unwrap(http.post(`/users/${id}/restore`)),
    resetPassword: (id, password) => unwrap(http.post(`/users/${id}/password`, { password })),
    unlock: (id) => unwrap(http.post(`/users/${id}/unlock`)),
    usage: (id) => unwrap(http.get(`/users/${id}/usage`))
  },
  files: {
    list: (params) => unwrap(http.get('/files', { params })),
    users: () => unwrap(http.get('/files/users')),
    stat: (params) => unwrap(http.get('/files/stat', { params })),
    mkdir: (payload) => unwrap(http.post('/files/mkdir', payload)),
    rename: (payload) => unwrap(http.post('/files/rename', payload)),
    move: (payload) => unwrap(http.post('/files/move', payload)),
    copy: (payload) => unwrap(http.post('/files/copy', payload)),
    remove: (payload) => unwrap(http.post('/files/delete', payload)),
    upload: (formData, onProgress) =>
      unwrap(
        http.post('/files/upload', formData, {
          timeout: 0,
          onUploadProgress: onProgress
        })
      ),
    extract: (payload) => unwrap(http.post('/files/extract', payload)),
    search: (params) => unwrap(http.get('/files/search', { params })),
    trash: () => unwrap(http.get('/files/trash')),
    restore: (ids) => unwrap(http.post('/files/trash/restore', { ids })),
    emptyTrash: (ids) => unwrap(http.post('/files/trash/empty', { ids })),
    usage: () => unwrap(http.get('/files/usage')),
    share: (payload) => unwrap(http.post('/files/share', payload))
  },
  shares: {
    list: (params) => unwrap(http.get('/shares', { params })),
    get: (id) => unwrap(http.get(`/shares/${id}`)),
    remove: (id) => unwrap(http.delete(`/shares/${id}`)),
    publicInfo: (token) => unwrap(http.get(`/s/${token}`)),
    verify: (token, password) => unwrap(http.post(`/s/${token}/verify`, { password })),
    downloadURL: (token) => `/api/v1/s/${token}/download`
  },
  photos: {
    list: (params) => envelope(http.get('/photos', { params })),
    timeline: () => unwrap(http.get('/photos/timeline')),
    map: () => unwrap(http.get('/photos/map')),
    get: (id) => unwrap(http.get(`/photos/${id}`)),
    remove: (id) => unwrap(http.delete(`/photos/${id}`)),
    share: (id, payload) => unwrap(http.post(`/photos/${id}/share`, payload)),
    upload: (formData, onProgress) =>
      unwrap(http.post('/photos/upload', formData, { timeout: 0, onUploadProgress: onProgress })),
    scan: () => unwrap(http.post('/photos/scan')),
    albums: () => unwrap(http.get('/photos/albums')),
    album: (id) => unwrap(http.get(`/photos/albums/${id}`)),
    createAlbum: (payload) => unwrap(http.post('/photos/albums', payload)),
    deleteAlbum: (id) => unwrap(http.delete(`/photos/albums/${id}`)),
    addToAlbum: (id, photoIds) => unwrap(http.post(`/photos/albums/${id}/photos`, { photo_ids: photoIds })),
    removeFromAlbum: (id, photoId) => unwrap(http.delete(`/photos/albums/${id}/photos/${photoId}`))
  },
  movies: {
    list: (params) => envelope(http.get('/movies', { params })),
    meta: () => unwrap(http.get('/movies/meta')),
    get: (id) => unwrap(http.get(`/movies/${id}`)),
    update: (id, payload) => unwrap(http.patch(`/movies/${id}`, payload)),
    remove: (id, deleteFile) => unwrap(http.delete(`/movies/${id}`, { params: { delete_file: deleteFile ? 1 : 0 } })),
    scan: () => unwrap(http.post('/movies/scan')),
    scrape: (ids) => unwrap(http.post('/movies/scrape', { ids })),
    subtitles: (id) => unwrap(http.get(`/movies/${id}/subtitles`)),
    tmdb: () => unwrap(http.get('/movies/tmdb')),
    setTmdb: (payload) => unwrap(http.put('/movies/tmdb', payload))
  },
  docker: {
    status: () => unwrap(http.get('/docker/status')),
    containers: (params) => unwrap(http.get('/docker/containers', { params })),
    action: (id, action) => unwrap(http.post(`/docker/containers/${id}/${action}`)),
    remove: (id, force) => unwrap(http.delete(`/docker/containers/${id}`, { params: { force: force ? 1 : 0 } })),
    logs: (id, tail = 300) => unwrap(http.get(`/docker/containers/${id}/logs`, { params: { tail } })),
    stats: (id) => unwrap(http.get(`/docker/containers/${id}/stats`)),
    images: () => unwrap(http.get('/docker/images')),
    pull: (image) => unwrap(http.post('/docker/images/pull', { image })),
    removeImage: (id) => unwrap(http.delete(`/docker/images/${id}`)),
    compose: () => unwrap(http.get('/docker/compose')),
    createCompose: (payload) => unwrap(http.post('/docker/compose', payload)),
    deleteCompose: (name) => unwrap(http.delete(`/docker/compose/${name}`)),
    composeUp: (name) => unwrap(http.post(`/docker/compose/${name}/up`)),
    composeDown: (name) => unwrap(http.post(`/docker/compose/${name}/down`)),
    composeLogs: (name, tail = 200) => unwrap(http.get(`/docker/compose/${name}/logs`, { params: { tail } }))
  },
  storage: {
    disks: () => unwrap(http.get('/storage/disks')),
    smart: (device) => unwrap(http.get('/storage/smart', { params: { device } })),
    smartTest: (payload) => unwrap(http.post('/storage/smart/test', payload)),
    smartTests: (device) => unwrap(http.get('/storage/smart/tests', { params: { device } })),
    pools: () => unwrap(http.get('/storage/pools')),
    createPool: (payload) => unwrap(http.post('/storage/pools', payload)),
    mountPool: (id) => unwrap(http.post(`/storage/pools/${id}/mount`)),
    unmountPool: (id) => unwrap(http.post(`/storage/pools/${id}/unmount`)),
    deletePool: (id, payload) => unwrap(http.delete(`/storage/pools/${id}`, { data: payload })),
    extendPool: (id, devices) => unwrap(http.post(`/storage/pools/${id}/extend`, { devices })),
    mounts: () => unwrap(http.get('/storage/mounts'))
  },
  network: {
    interfaces: () => unwrap(http.get('/network/interfaces')),
    applyInterface: (name, payload) => unwrap(http.post(`/network/interfaces/${name}`, payload)),
    ports: () => unwrap(http.get('/network/ports')),
    shares: () => unwrap(http.get('/network/shares')),
    createShare: (payload) => unwrap(http.post('/network/shares', payload)),
    updateShare: (id, payload) => unwrap(http.patch(`/network/shares/${id}`, payload)),
    deleteShare: (id) => unwrap(http.delete(`/network/shares/${id}`)),
    apply: () => unwrap(http.post('/network/apply')),
    ddns: () => unwrap(http.get('/network/ddns')),
    setDDNS: (payload) => unwrap(http.put('/network/ddns', payload)),
    updateDDNS: () => unwrap(http.post('/network/ddns/update')),
    dns: () => unwrap(http.get('/network/dns')),
    setDNS: (servers) => unwrap(http.put('/network/dns', { servers }))
  },
  apps: {
    list: () => unwrap(http.get('/apps')),
    install: (slug) => unwrap(http.post(`/apps/${slug}/install`)),
    uninstall: (slug) => unwrap(http.post(`/apps/${slug}/uninstall`)),
    start: (slug) => unwrap(http.post(`/apps/${slug}/start`)),
    stop: (slug) => unwrap(http.post(`/apps/${slug}/stop`)),
    status: (slug) => unwrap(http.get(`/apps/${slug}/status`))
  },
  monitor: {
    overview: () => unwrap(http.get('/monitor/overview')),
    history: (limit = 60) => unwrap(http.get('/monitor/history', { params: { limit } })),
    processes: () => unwrap(http.get('/monitor/processes')),
    kill: (pid, force) => unwrap(http.post(`/monitor/processes/${pid}/kill`, { force }))
  },
  jobs: {
    list: () => unwrap(http.get('/jobs')),
    cancel: (id) => unwrap(http.post(`/jobs/${id}/cancel`))
  },
  logs: {
    audit: (params) => envelope(http.get('/logs/audit', { params })),
    login: (params) => unwrap(http.get('/logs/login', { params })),
    system: () => unwrap(http.get('/logs/system')),
    destroy: (params) => unwrap(http.get('/logs/destroy', { params }))
  },
  settings: {
    all: () => unwrap(http.get('/settings')),
    put: (key, payload) => unwrap(http.put(`/settings/${key}`, payload)),
    info: () => unwrap(http.get('/system/info')),
    updates: () => unwrap(http.get('/system/updates')),
    applyUpdates: () => unwrap(http.post('/system/updates/apply')),
    ssl: () => unwrap(http.get('/system/ssl')),
    selfSigned: (domain) => unwrap(http.post('/system/ssl/selfsigned', { domain })),
    firewall: () => unwrap(http.get('/system/firewall')),
    setFirewall: (payload) => unwrap(http.put('/system/firewall', payload)),
    confirmFirewall: () => unwrap(http.post('/system/firewall/confirm')),
    encryption: () => unwrap(http.get('/system/encryption')),
    vaults: () => unwrap(http.get('/system/vaults')),
    createVault: (username, sizeGb) => unwrap(http.post(`/system/vaults/${username}/create`, { size_gb: sizeGb })),
    unlockVault: (username) => unwrap(http.post(`/system/vaults/${username}/unlock`)),
    lockVault: (username) => unwrap(http.post(`/system/vaults/${username}/lock`)),
    destroy: (payload) => unwrap(http.post('/system/destroy', payload)),
    destroyPolicy: () => unwrap(http.get('/system/destroy/policy')),
    setDestroyPolicy: (payload) => unwrap(http.put('/system/destroy/policy', payload)),
    testNotification: () => unwrap(http.post('/notifications/test')),
    restoreBackup: (formData) => unwrap(http.post('/system/restore', formData, { timeout: 0 }))
  },
  notifications: {
    list: (params) => unwrap(http.get('/notifications', { params })),
    read: (id) => unwrap(http.post('/notifications/read', { id })),
    remove: (id) => unwrap(http.delete(`/notifications/${id}`))
  }
}

export { http }
