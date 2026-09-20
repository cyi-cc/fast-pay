import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '../stores/user'
import { REDIRECT_KEY } from '../api'

const routes = [
  { path: '/login', name: 'login', component: () => import('../views/Login.vue'), meta: { public: true } },
  {
    path: '/',
    component: () => import('../layouts/MainLayout.vue'),
    redirect: '/orders',
    children: [
      { path: 'profile', name: 'profile', component: () => import('../views/merchant/Profile.vue'), meta: { title: '系统配置' } },
      { path: 'orders', name: 'orders', component: () => import('../views/merchant/Orders.vue'), meta: { title: '订单管理' } },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/orders' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async to => {
  const store = useUserStore()
  if (to.meta.public) return true
  if (!store.token) {
    // 登录后的回跳目标存 localStorage，避免出现在 URL 中
    localStorage.setItem(REDIRECT_KEY, to.fullPath)
    return { name: 'login' }
  }
  if (!store.profile) {
    try {
      const profile = await store.fetchProfile()
      if (!profile) {
        localStorage.setItem(REDIRECT_KEY, to.fullPath)
        return { name: 'login' }
      }
    } catch {
      localStorage.setItem(REDIRECT_KEY, to.fullPath)
      return { name: 'login' }
    }
  }
  return true
})

export default router
