// fun RPC 客户端封装：token 注入 + 签发满 2 小时后台轮换 + 401 处理 + 结果解包
import api from './fun'

export const TOKEN_KEY = 'funpay_token'
export const REDIRECT_KEY = 'funpay_redirect'

const client = api.create('/api')

// 会话令牌有效期 24 小时；签发满 2 小时由请求拦截器后台触发显式轮换，
// 轮换失败 30 秒内不重试，避免风暴
const TOKEN_REFRESH_AFTER = 2 * 3600 * 1000
const REFRESH_RETRY_COOLDOWN = 30 * 1000

function safeParse(s) {
  try { return s ? JSON.parse(s) : null } catch { return null }
}

// 每次请求现读 localStorage（而非内存缓存），多标签页轮换后天然同步
export function readToken() {
  const raw = localStorage.getItem(TOKEN_KEY)
  const stored = safeParse(raw)
  if (stored && typeof stored === 'object' && stored.token) return stored.token
  if (typeof stored === 'string') return stored
  return raw || ''
}

export function readTokenAt() {
  const stored = safeParse(localStorage.getItem(TOKEN_KEY))
  if (stored && typeof stored === 'object' && stored.token) return stored.at || 0
  return 0
}

export function persistToken(token) {
  localStorage.setItem(TOKEN_KEY, JSON.stringify({ token, at: Date.now() }))
}

export function clearToken() {
  localStorage.removeItem(TOKEN_KEY)
}

let refreshInFlight = null
let lastRefreshFailedAt = 0

// 后台轮换：成功即更新 localStorage（后续请求自动带新令牌），失败仅记冷却时间。
// 服务端为旧令牌保留 5 分钟宽限期，在途请求与重复刷新不受影响
function triggerTokenRefresh() {
  refreshInFlight = (async () => {
    try {
      const r = await client.authSvc.refresh()
      if (r.status === 0 && r.data?.token) {
        persistToken(r.data.token)
        return
      }
    } catch { /* 网络异常：冷却后随下一次请求重试 */ }
    lastRefreshFailedAt = Date.now()
  })().finally(() => { refreshInFlight = null })
  return refreshInFlight
}

// 请求拦截：把 localStorage 中的会话令牌写入 state；签发满 2 小时后台轮换
client.addRequestInterceptor((svc, m, state) => {
  const token = readToken()
  if (token) state.token = token
  if (!token) return
  // 轮换中/冷却中/refresh 自身都不再进轮换链路；触发轮换的本次请求照常带旧令牌
  if (svc.toLowerCase() === 'authsvc' && m.toLowerCase() === 'refresh') return
  if (refreshInFlight || Date.now() - lastRefreshFailedAt < REFRESH_RETRY_COOLDOWN) return
  if (Date.now() - readTokenAt() > TOKEN_REFRESH_AFTER) void triggerTokenRefresh()
})

// 响应拦截：登录失效统一跳转登录页
client.addResponseInterceptor((_svc, _m, result) => {
  if (result.status === 2 && (result.code === 4010 || result.code === 4011)) {
    clearToken()
    if (!location.pathname.startsWith('/login')) {
      location.href = '/login'
    }
  }
  return result
})

// call：解包 result<T>，业务失败抛 Error(msg)
export async function call(promise) {
  const r = await promise
  if (r.status !== 0) {
    throw new Error(r.msg || '请求失败')
  }
  return r.data
}

export default client
