import { defineStore } from 'pinia'
import { apps } from '../apps'

export const TASKBAR_HEIGHT = 52
const MIN_WIDTH = 440
const MIN_HEIGHT = 300

export const useDesktopStore = defineStore('desktop', {
  state: () => ({
    windows: [],
    activeId: null,
    zTop: 100,
    seq: 0,
    startOpen: false,
    startSearchNonce: 0
  }),
  getters: {
    activeWindow: (state) => state.windows.find((w) => w.id === state.activeId) || null,
    visibleWindows: (state) => state.windows.filter((w) => !w.minimized)
  },
  actions: {
    viewport() {
      return {
        width: window.innerWidth,
        height: Math.max(320, window.innerHeight - TASKBAR_HEIGHT)
      }
    },
    open(appKey, options = {}) {
      const meta = apps[appKey]
      if (!meta) return null
      const existing = this.windows.filter((w) => w.app === appKey)
      if (existing.length && !options.forceNew) {
        const top = existing.slice().sort((a, b) => b.z - a.z)[0]
        this.focus(top.id)
        return top
      }

      const vp = this.viewport()
      const mobile = vp.width < 900
      const w = mobile ? vp.width : Math.min(meta.width || 1100, vp.width - 28)
      const h = mobile ? vp.height : Math.min(meta.height || 720, vp.height - 24)
      const count = this.windows.length
      const offset = (count % 7) * 24
      const x = mobile ? 0 : Math.max(12, Math.min((vp.width - w) / 2 + offset - 70, vp.width - w - 12))
      const y = mobile ? 0 : Math.max(10, Math.min((vp.height - h) / 2 + offset - 40, vp.height - h - 10))
      const win = {
        id: `win-${++this.seq}`,
        app: appKey,
        x,
        y,
        w,
        h,
        z: ++this.zTop,
        minimized: false,
        maximized: mobile || !!options.maximized,
        restore: { x, y, w, h }
      }
      this.windows.push(win)
      this.activeId = win.id
      return win
    },
    focus(id) {
      const win = this.windows.find((w) => w.id === id)
      if (!win) return
      win.minimized = false
      win.z = ++this.zTop
      this.activeId = id
    },
    minimize(id) {
      const win = this.windows.find((w) => w.id === id)
      if (!win) return
      win.minimized = true
      if (this.activeId === id) this.activeId = this.topVisible()?.id || null
    },
    toggleMinimize(id) {
      const win = this.windows.find((w) => w.id === id)
      if (!win) return
      if (win.minimized) this.focus(id)
      else if (this.activeId === id) this.minimize(id)
      else this.focus(id)
    },
    toggleMaximize(id) {
      const win = this.windows.find((w) => w.id === id)
      if (!win) return
      if (win.maximized) {
        win.maximized = false
        Object.assign(win, win.restore)
      } else {
        win.restore = { x: win.x, y: win.y, w: win.w, h: win.h }
        win.maximized = true
      }
      win.z = ++this.zTop
      this.activeId = id
    },
    move(id, x, y) {
      const win = this.windows.find((w) => w.id === id)
      if (!win || win.maximized) return
      const vp = this.viewport()
      win.x = Math.min(Math.max(x, -win.w + 150), vp.width - 110)
      win.y = Math.min(Math.max(y, 0), vp.height - 42)
    },
    resize(id, w, h) {
      const win = this.windows.find((w) => w.id === id)
      if (!win || win.maximized) return
      const vp = this.viewport()
      win.w = Math.min(Math.max(w, MIN_WIDTH), vp.width - 6)
      win.h = Math.min(Math.max(h, MIN_HEIGHT), vp.height - 6)
    },
    close(id) {
      const idx = this.windows.findIndex((w) => w.id === id)
      if (idx < 0) return null
      const [closed] = this.windows.splice(idx, 1)
      if (this.activeId === id) this.activeId = this.topVisible()?.id || null
      return closed
    },
    closeAll() {
      this.windows = []
      this.activeId = null
      this.startOpen = false
    },
    minimizeAll() {
      this.windows.forEach((w) => { w.minimized = true })
      this.activeId = null
    },
    clampAll() {
      const vp = this.viewport()
      this.windows.forEach((win) => {
        if (win.maximized) return
        win.w = Math.min(win.w, vp.width - 6)
        win.h = Math.min(win.h, vp.height - 6)
        win.x = Math.min(Math.max(win.x, -win.w + 150), vp.width - 110)
        win.y = Math.min(Math.max(win.y, 0), Math.max(0, vp.height - 42))
      })
    },
    topVisible() {
      return this.windows.filter((w) => !w.minimized).sort((a, b) => b.z - a.z)[0] || null
    },
    topOfApp(appKey, excludeId = '') {
      return this.windows
        .filter((w) => w.app === appKey && w.id !== excludeId)
        .sort((a, b) => b.z - a.z)[0] || null
    },
    instanceNo(win) {
      const same = this.windows.filter((w) => w.app === win.app)
      return same.findIndex((w) => w.id === win.id) + 1
    },
    instanceCount(appKey) {
      return this.windows.filter((w) => w.app === appKey).length
    },
    openSearch() {
      this.startOpen = true
      this.startSearchNonce += 1
    }
  }
})
