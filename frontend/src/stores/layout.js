import { defineStore } from 'pinia'

const KEY = 'pos_sidebars_v1'
const defaults = {
  files: { visible: true, width: 248 },
  photos: { visible: true, width: 232 },
  settings: { visible: true, width: 206 }
}

function load() {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY) || '{}')
    return {
      files: { ...defaults.files, ...(saved.files || {}) },
      photos: { ...defaults.photos, ...(saved.photos || {}) },
      settings: { ...defaults.settings, ...(saved.settings || {}) }
    }
  } catch (error) {
    return { ...defaults }
  }
}

export const useLayoutStore = defineStore('layout', {
  state: () => ({ sidebars: load() }),
  actions: {
    persist() {
      localStorage.setItem(KEY, JSON.stringify(this.sidebars))
    },
    toggle(name) {
      if (!this.sidebars[name]) return
      this.sidebars[name].visible = !this.sidebars[name].visible
      this.persist()
    },
    setVisible(name, visible) {
      if (!this.sidebars[name]) return
      this.sidebars[name].visible = visible
      this.persist()
    },
    setWidth(name, width) {
      if (!this.sidebars[name]) return
      this.sidebars[name].width = Math.max(160, Math.min(420, Math.round(width)))
      this.persist()
    }
  }
})
