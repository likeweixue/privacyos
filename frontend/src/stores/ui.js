import { defineStore } from 'pinia'
import i18n from '../locales'

const THEME_KEY = 'pos_theme_v2'
const LANG_KEY = 'pos_lang'
const SIDEBAR_KEY = 'pos_sidebar'

export const useUiStore = defineStore('ui', {
  state: () => ({
    theme: localStorage.getItem(THEME_KEY) || 'light',
    lang: localStorage.getItem(LANG_KEY) || 'zh-CN',
    sidebarCollapsed: localStorage.getItem(SIDEBAR_KEY) === '1',
    inboxOpen: false
  }),
  actions: {
    applyTheme() {
      const html = document.documentElement
      html.classList.toggle('dark', this.theme === 'dark')
      html.classList.toggle('light', this.theme === 'light')
    },
    setTheme(theme) {
      this.theme = theme === 'light' ? 'light' : 'dark'
      localStorage.setItem(THEME_KEY, this.theme)
      this.applyTheme()
    },
    toggleTheme() {
      this.setTheme(this.theme === 'dark' ? 'light' : 'dark')
    },
    setLang(lang) {
      this.lang = lang === 'en-US' ? 'en-US' : 'zh-CN'
      localStorage.setItem(LANG_KEY, this.lang)
      i18n.global.locale.value = this.lang
      document.documentElement.setAttribute('lang', this.lang === 'en-US' ? 'en' : 'zh-CN')
    },
    toggleLang() {
      this.setLang(this.lang === 'zh-CN' ? 'en-US' : 'zh-CN')
    },
    toggleSidebar() {
      this.sidebarCollapsed = !this.sidebarCollapsed
      localStorage.setItem(SIDEBAR_KEY, this.sidebarCollapsed ? '1' : '0')
    }
  }
})
