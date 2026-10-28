import { h } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import { me } from './api'
import AdminLogin from './views/admin/AdminLogin.vue'
import Login from './views/Login.vue'

const placeholder = {
  render() {
    return h('main', { class: 'min-h-screen bg-page' })
  },
}

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    { path: '/', component: placeholder },
    { path: '/admin/login', component: AdminLogin },
    { path: '/admin', component: placeholder },
  ],
})

router.beforeEach(async (to) => {
  if (to.path !== '/') {
    return true
  }
  try {
    await me()
    return true
  } catch {
    return '/login'
  }
})
