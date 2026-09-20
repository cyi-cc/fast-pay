import { defineStore } from 'pinia'
import { readToken, persistToken, clearToken } from '../api'
import client from '../api'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: readToken(),
    profile: null,
  }),
  getters: {
    isLogin: s => !!s.token,
    isAdmin: s => s.profile?.role === 1,
  },
  actions: {
    setLogin(token, user) {
      this.token = token
      this.profile = user
      persistToken(token)
    },
    async fetchProfile() {
      const r = await client.authSvc.me()
      if (r.status === 0) {
        this.profile = r.data
      }
      return this.profile
    },
    async logout() {
      try {
        await client.authSvc.logout()
      } catch {
        /* 忽略登出接口错误 */
      }
      this.token = ''
      this.profile = null
      clearToken()
    },
  },
})
