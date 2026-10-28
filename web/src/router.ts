import { createRouter, createWebHistory } from 'vue-router'
import { me } from './api'
import AdminLogin from './views/admin/AdminLogin.vue'
import Chat from './views/Chat.vue'
import Login from './views/Login.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login },
    { path: '/', component: Chat },
    { path: '/admin/login', component: AdminLogin },
    {
      path: '/admin',
      component: () => import('./views/admin/Layout.vue'),
      children: [
        { path: '', component: () => import('./views/admin/Overview.vue') },
        { path: 'users', component: () => import('./views/admin/Users.vue') },
        { path: 'groups', component: () => import('./views/admin/Groups.vue') },
      ],
    },
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
