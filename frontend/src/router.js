import { createRouter, createWebHistory } from 'vue-router'
import { getToken, userState } from './api'

import PublicOrderView from './views/PublicOrderView.vue'
import LoginView from './views/LoginView.vue'
import ClientCabinetView from './views/ClientCabinetView.vue'
import ManagerView from './views/ManagerView.vue'
import AdminView from './views/AdminView.vue'

const routes = [
  { path: '/', name: 'public', component: PublicOrderView },
  { path: '/login', name: 'login', component: LoginView },
  { path: '/cabinet', name: 'cabinet', component: ClientCabinetView, meta: { role: 'client' } },
  { path: '/manager', name: 'manager', component: ManagerView, meta: { role: 'manager' } },
  { path: '/admin', name: 'admin', component: AdminView, meta: { role: 'superadmin' } },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Защита маршрутов по роли.
router.beforeEach((to) => {
  const required = to.meta.role
  if (!required) return true

  if (!getToken()) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  const user = userState.value
  if (!user || user.role !== required) {
    return user?.role === 'client' ? { name: 'cabinet' } : { name: 'login' }
  }
  return true
})

export default router
