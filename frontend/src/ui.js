// 全局消息/确认框（离散 API，无需依赖 provider 组件树）
import { createDiscreteApi } from 'naive-ui'

const { message, dialog, notification } = createDiscreteApi(['message', 'dialog', 'notification'])

export { message, dialog, notification }

// confirm 简易封装：Promise 化
export function confirm(content, { title = '确认操作', type = 'warning', positiveText = '确定', negativeText = '取消' } = {}) {
  return new Promise(resolve => {
    dialog[type]({
      title,
      content,
      positiveText,
      negativeText,
      onPositiveClick: () => resolve(true),
      onNegativeClick: () => resolve(false),
      onClose: () => resolve(false),
    })
  })
}
