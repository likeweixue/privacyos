import { defineStore } from 'pinia'
import { apps } from '../desktop/apps'

const TASKBAR_HEIGHT = 56
const MIN_WIDTH = 460
const MIN_HEIGHT = 320

export const useDesktopStore = defineStore('desktop', {
  state: () => ({
    windows: [],
    activeId: null,
    zTop: 100,
    seq: 0,
    startOpen: false,
    showDesktopAt: 0
  }),
  getters: {
    activeWindow: (state) => state.windows.find((w) => w.id === state.activeId) || null,
    visibleWindows: (state) => state.windows.filter((w) => !w.minimized),
    taskbarHeight: () => TASKBAR_HEIGHT
  },
  actions: {
    viewport() {
      return {
        width: window.innerWidth,
        height: Math.max(360, window.innerHeight - TASKBAR_HEIGHT)
      }
    },
    open(appKey, options = {}) {
      const meta = apps[appKey]
      if (!meta) return null
      const existing = this.windows.find((w) => w.app === appKey)
      if (existing) {
        existing.minimized = false
        this.focus(existing.id)
        return existing
      }
      const vp = this.viewport()
      const mobile = vp.width < 900
      const w = mobile ? vp.width : Math.min(meta.width, vp.width - 48)
      const h = mobile ? vp.height : Math.min(meta.height, vp.height - 40)
      const offset = (this.windows.length % 6) * 26
      const x = mobile ? 0 : Math.max(16, Math.min((vp.width - w) / 2 + offset - 70, vp.width - w - 16))
      const y = mobile ? 0 : Math.max(12, Math.min((vp.height - h) / 2 + offset - 50, vp.height - h - 12))
      const win = {
        id: `win-${++this.seq}`,
        app: appKey,
        routeName: meta.routeName,
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
      win.z = ++this.zTop
      win.minimized = false
      this.activeId = id
    },
    minimize(id) {
      const win = this.windows.find((w) => w.id === id)
      if (!win) return
      win.minimized = true
      if (this.activeId === id) {
        this.activeId = this.topVisible()?.id || null
      }
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
        win.z = ++this.zTop
      } else {
        win.restore = { x: win.x, y: win.y, w: win.w, h: win.h }
        win.maximized = true
        win.z = ++this.zTop
      }
      this.activeId = id
    },
    move(id, x, y) {
      const win = this.windows.find((w) => w.id === id)
      if (!win || win.maximized) return
      const vp = this.viewport()
      win.x = Math.min(Math.max(x, -win.w + 160), vp.width - 120)
      win.y = Math.min(Math.max(y, 0), vp.height - 48)
    },
    resize(id, w, h) {
      const win = this.windows.find((w) => w.id === id)
      if (!win || win.maximized) return
      const vp = this.viewport()
      win.w = Math.min(Math.max(w, MIN_WIDTH), vp.width - 8)
      win.h = Math.min(Math.max(h, MIN_HEIGHT), vp.height - 8)
    },
    close(id) {
      const idx = this.windows.findIndex((w) => w.id === id)
      if (idx < 0) return null
      const [closed] = this.windows.splice(idx, 1)
      if (this.activeId === id) {
        this.activeId = this.topVisible()?.id || null
      }
      return closed
    },
    closeAll() {
      this.windows = []
      this.activeId = null
    },
    minimizeAll() {
      this.windows.forEach((w) => { w.minimized = true })
      this.activeId = null
      this.showDesktopAt = Date.now()
    },
    clampAll() {
      const vp = this.viewport()
      this.windows.forEach((win) => {
        if (win.maximized) return
        win.w = Math.min(win.w, vp.width - 8)
        win.h = Math.min(win.h, vp.height - 8)
        win.x = Math.min(Math.max(win.x, -win.w + 160), vp.width - 120)
        win.y = Math.min(Math.max(win.y, 0), Math.max(0, vp.height - 48))
      })
    },
    topVisible() {
      return this.windows.filter((w) => !w.minimized).sort((a, b) => b.z - a.z)[0] || null
    }
  }
})
