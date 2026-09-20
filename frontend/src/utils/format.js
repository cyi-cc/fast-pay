// 展示格式化与字典映射

export function fen2yuan(fen) {
  return (Number(fen || 0) / 100).toFixed(2)
}

export function yuan2fen(str) {
  const n = Math.round(parseFloat(String(str).trim()) * 100)
  if (!Number.isFinite(n)) return 0
  return n
}

export function fmtTime(ts) {
  if (!ts) return '-'
  return new Date(Number(ts) * 1000).toLocaleString('zh-CN', { hour12: false })
}

export const orderStatusMap = {
  0: { label: '待支付', type: 'warning' },
  1: { label: '已支付', type: 'success' },
  2: { label: '已退款', type: 'info' },
  3: { label: '已关闭', type: 'error' },
}

export const channelMap = {
  alipay: '支付宝',
  wxpay: '微信支付',
  qqpay: 'QQ钱包',
}

export const settleStatusMap = {
  0: { label: '待审核', type: 'warning' },
  1: { label: '已打款', type: 'success' },
  2: { label: '已驳回', type: 'error' },
}

export const balanceLogTypeMap = {
  0: '订单收入',
  1: '提现扣款',
  2: '驳回返还',
  3: '管理员调整',
  4: '订单退款',
}

export function copyText(text) {
  if (navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(text)
  }
  const ta = document.createElement('textarea')
  ta.value = text
  document.body.appendChild(ta)
  ta.select()
  document.execCommand('copy')
  ta.remove()
  return Promise.resolve()
}
