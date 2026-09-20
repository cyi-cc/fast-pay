import { createApp } from 'vue'
import { createPinia } from 'pinia'
import naive from 'naive-ui'
import './style.css'
import App from './App.vue'
import router from './router'

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(naive) // 全局注册全部 Naive UI 组件（模板中直接使用 <n-xxx>）
app.mount('#app')
