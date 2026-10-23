import { defineStore } from 'pinia'
import { api, errorMessage, tokenStore } from '../api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    edition: 'normal',
    editionName: '普通版',
    pro: false,
    version: '',
    features: {},
    adminSeeAll: false,
    brand: { name: 'PrivacyOS', website: 'privacyos.local' },
    dev: false,
    loading: false
  }),
  getters: {
    isLoggedIn: () => !!tokenStore.access,
    isAdmin: (state) => state.user?.role === 'admin',
    username: (state) => state.user?.username || '',
    displayName: (state) => state.user?.display_name || state.user?.username || '',
    permissions: (state) => state.features || {}
  },
  actions: {
    applyMe(payload) {
      this.user = payload.user
      this.edition = payload.edition
      this.editionName = payload.edition_name
      this.pro = payload.pro
      this.version = payload.version
      this.features = payload.features || {}
      this.adminSeeAll = payload.admin_see_all
      this.brand = payload.brand || this.brand
      this.dev = payload.dev
      localStorage.setItem('pos_edition', payload.edition)
      localStorage.setItem('pos_version', payload.version)
    },
    async login(credentials) {
      this.loading = true
      try {
        const payload = await api.auth.login(credentials)
        tokenStore.set(payload.access_token, payload.refresh_token)
        this.user = payload.user
        await this.fetchMe()
        return payload
      } finally {
        this.loading = false
      }
    },
    async fetchMe() {
      const payload = await api.auth.me()
      this.applyMe(payload)
      return payload
    },
    async logout() {
      try {
        await api.auth.logout()
      } catch (error) {
        // Session may already be gone; local cleanup still applies.
      }
      tokenStore.clear()
      this.user = null
      this.features = {}
    },
    async updatePreferences(payload) {
      const result = await api.auth.updatePreferences(payload)
      if (this.user) this.user.preferences = result.preferences
      return result
    },
    async refresh() {
      await this.fetchMe()
    },
    errorMessage
  }
})
