import { createRouter, createWebHistory } from 'vue-router'
import { tokenStore } from '../api'
import { useAuthStore } from '../stores/auth'
import Desktop from '../desktop/Desktop.vue'

// The desktop shell renders windows itself; child route components are only
// used as metadata containers and are mounted by the window manager.
const EmptyView = { name: 'EmptyView', render: () => null }

const routes = [
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
        component: EmptyView,
        meta: { title: 'desktop.title' }
      },
      {
        path: 'dashboard',
        name: 'dashboard',
        component: () => import('../views/Dashboard.vue'),
        meta: { title: 'nav.dashboard' }
      },
      {
        path: 'files',
        name: 'files',
        component: () => import('../views/Files.vue'),
        meta: { title: 'nav.files', feature: 'files' }
      },
      {
        path: 'photos',
        name: 'photos',
        component: () => import('../views/Photos.vue'),
        meta: { title: 'nav.photos', feature: 'photos' }
      },
      {
        path: 'movies',
        name: 'movies',
        component: () => import('../views/Movies.vue'),
        meta: { title: 'nav.movies', feature: 'movies' }
      },
      {
        path: 'docker',
        name: 'docker',
        component: () => import('../views/Docker.vue'),
        meta: { title: 'nav.docker', feature: 'docker' }
      },
      {
        path: 'users',
        name: 'users',
        component: () => import('../views/Users.vue'),
        meta: { title: 'nav.users', admin: true }
      },
      {
        path: 'storage',
        name: 'storage',
        component: () => import('../views/Storage.vue'),
        meta: { title: 'nav.storage', admin: true }
      },
      {
        path: 'network',
        name: 'network',
        component: () => import('../views/Network.vue'),
        meta: { title: 'nav.network', admin: true }
      },
      {
        path: 'apps',
        name: 'apps',
        component: () => import('../views/Apps.vue'),
        meta: { title: 'nav.apps', admin: true }
      },
      {
        path: 'monitor',
        name: 'monitor',
        component: () => import('../views/Monitor.vue'),
        meta: { title: 'nav.monitor' }
      },
      {
        path: 'logs',
        name: 'logs',
        component: () => import('../views/Logs.vue'),
        meta: { title: 'nav.logs', admin: true }
      },
      {
        path: 'settings',
        name: 'settings',
        component: () => import('../views/Settings.vue'),
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
