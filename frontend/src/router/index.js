import { createRouter, createWebHistory } from 'vue-router'
import { tokenStore } from '../api'
import { useAuthStore } from '../stores/auth'
import Desktop from '../desktop/Desktop.vue'

// App views are mounted inside desktop windows by the registry in src/apps.js.
const AppStub = { name: 'AppStub', render: () => null }

const routes = [
  {
    path: '/install',
    name: 'install',
    component: () => import('../views/Install.vue'),
    meta: { public: true, title: 'installer.title' }
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/Login.vue'),
    meta: { public: true, title: 'login.title' }
  },
  {
    path: '/',
    component: Desktop,
    children: [
      {
        path: '',
        name: 'desktop',
        component: AppStub,
        meta: { title: 'desktop.title' }
      },
      {
        path: 'dashboard',
        name: 'dashboard',
        component: AppStub,
        meta: { title: 'nav.dashboard' }
      },
      {
        path: 'files',
        name: 'files',
        component: AppStub,
        meta: { title: 'nav.files', feature: 'files' }
      },
      {
        path: 'photos',
        name: 'photos',
        component: AppStub,
        meta: { title: 'nav.photos', feature: 'photos' }
      },
      {
        path: 'movies',
        name: 'movies',
        component: AppStub,
        meta: { title: 'nav.movies', feature: 'movies' }
      },
      {
        path: 'docker',
        name: 'docker',
        component: AppStub,
        meta: { title: 'nav.docker', feature: 'docker' }
      },
      {
        path: 'users',
        name: 'users',
        component: AppStub,
        meta: { title: 'nav.users', admin: true }
      },
      {
        path: 'storage',
        name: 'storage',
        component: AppStub,
        meta: { title: 'nav.storage', admin: true }
      },
      {
        path: 'network',
        name: 'network',
        component: AppStub,
        meta: { title: 'nav.network', admin: true }
      },
      {
        path: 'apps',
        name: 'apps',
        component: AppStub,
        meta: { title: 'nav.apps', admin: true }
      },
      {
        path: 'monitor',
        name: 'monitor',
        component: AppStub,
        meta: { title: 'nav.monitor' }
      },
      {
        path: 'logs',
        name: 'logs',
        component: AppStub,
        meta: { title: 'nav.logs', admin: true }
      },
      {
        path: 'settings',
        name: 'settings',
        component: AppStub,
        meta: { title: 'nav.settings', admin: true }
      }
    ]
  },
  {
    path: '/s/:token',
    name: 'share',
    component: () => import('../views/PublicShare.vue'),
    meta: { public: true, title: 'share.title' }
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../views/NotFound.vue'),
    meta: { public: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 })
})

router.beforeEach(async (to) => {
  if (to.name === 'login') {
    return tokenStore.access ? { name: 'dashboard' } : true
  }
  if (to.meta.public) return true
  if (!tokenStore.access) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  const auth = useAuthStore()
  if (!auth.user) {
    try {
      await auth.fetchMe()
    } catch (error) {
      tokenStore.clear()
      return { name: 'login', query: { redirect: to.fullPath } }
    }
  }
  if (to.meta.admin && !auth.isAdmin) return { name: 'dashboard' }
  if (to.meta.feature && !auth.isAdmin && auth.features[to.meta.feature] === false) {
    return { name: 'dashboard' }
  }
  return true
})

export default router
