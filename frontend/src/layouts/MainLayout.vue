<template>
  <div class="app-layout">
    <!-- ===== 左侧导航栏 ===== -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <div class="brand">
          <div class="brand-logo">
            <n-icon :component="Flash" :size="17" />
          </div>
          <div class="brand-text">
            <span class="brand-name">Fast Pay</span>
            <span class="brand-sub">支付管理控制台</span>
          </div>
        </div>
      </div>

      <div class="nav-group">
        <span class="nav-group-title">业务管理</span>
        <nav class="nav-list" aria-label="主要功能">
          <router-link
            v-for="item in navItems"
            :key="item.key"
            :to="{ name: item.key }"
            class="nav-link"
            :class="{ active: route.name === item.key }"
          >
            <span class="nav-icon-wrap" :class="item.tone">
              <n-icon :component="item.icon" :size="15" />
            </span>
            <span class="nav-title">{{ item.label }}</span>
            <n-icon v-if="route.name === item.key" :component="ChevronForwardOutline" :size="13" class="nav-arrow" />
          </router-link>
        </nav>
      </div>

      <div class="sidebar-spacer"></div>

      <!-- 底部退出登录 -->
      <button class="logout-link" @click="logout">
        <span class="nav-icon-wrap tone-red">
          <n-icon :component="LogOutOutline" :size="15" />
        </span>
        <span class="nav-title">退出登录</span>
      </button>
    </aside>

    <!-- ===== 右侧工作区 ===== -->
    <div class="main-viewport">
      <main class="page-container">
        <div class="page-content">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { useRoute, useRouter } from 'vue-router'
import { NIcon } from 'naive-ui'
import { ReceiptOutline, PersonOutline, LogOutOutline, ChevronForwardOutline, Flash } from '@vicons/ionicons5'
import { useUserStore } from '../stores/user'

const route = useRoute()
const router = useRouter()
const store = useUserStore()

const navItems = [
  { key: 'profile', label: '系统配置', icon: PersonOutline, tone: 'tone-violet' },
  { key: 'orders', label: '订单管理', icon: ReceiptOutline, tone: 'tone-blue' },
]

async function logout() {
  await store.logout()
  router.push('/login')
}
</script>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background-color: var(--bg);
}

/* ===== 侧栏规范 ===== */
.sidebar {
  width: 244px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: #FFFFFF;
  border-right: 1px solid var(--line);
  padding: 16px 12px;
}

.sidebar-header {
  padding: 4px 8px 20px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.brand-logo {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: linear-gradient(160deg, #3B82F6 0%, #1D4ED8 100%);
  box-shadow: 0 5px 12px rgba(37, 99, 235, 0.32), inset 0 1px 0 rgba(255, 255, 255, 0.25);
  color: #FFFFFF;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.brand-text {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.brand-name {
  font-size: 15px;
  font-weight: 700;
  color: #0F172A;
  letter-spacing: -0.01em;
  line-height: 1.25;
}

.brand-sub {
  font-size: 10px;
  color: #94A3B8;
  font-weight: 500;
  letter-spacing: 0.02em;
}

.nav-group {
  margin-top: 4px;
}

.nav-group-title {
  display: block;
  font-size: 11px;
  font-weight: 500;
  color: #94A3B8;
  padding: 0 8px 8px;
}

.nav-list {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.nav-link {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  height: 42px;
  padding: 0 12px 0 8px;
  border-radius: 10px;
  color: #475569;
  text-decoration: none;
  font-size: 13px;
  font-weight: 500;
  transition: all 0.18s ease;
}

.nav-link:hover {
  background: #F4F5F7;
  color: #0F172A;
}

.nav-link.active {
  background: var(--brand);
  color: #FFFFFF;
  font-weight: 600;
  box-shadow: 0 6px 14px -6px rgba(37, 99, 235, 0.45), inset 0 1px 0 rgba(255, 255, 255, 0.18);
}

.nav-icon-wrap {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.18s ease;
}

.nav-icon-wrap.tone-blue {
  background: #EFF6FF;
  color: #2563EB;
}

.nav-icon-wrap.tone-violet {
  background: #F5F3FF;
  color: #7C3AED;
}

.nav-link.active .nav-icon-wrap {
  background: rgba(255, 255, 255, 0.2);
  color: #FFFFFF;
}

.nav-title {
  flex: 1;
}

.nav-arrow {
  color: rgba(255, 255, 255, 0.75);
}

.sidebar-spacer {
  flex: 1;
}

/* 退出登录 */
.logout-link {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 42px;
  padding: 0 12px 0 8px;
  border-radius: 10px;
  border: none;
  background: transparent;
  color: #475569;
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  cursor: pointer;
  transition: all 0.18s ease;
}

.logout-link:hover {
  background: #FEF2F2;
  color: #DC2626;
}

.nav-icon-wrap.tone-red {
  background: #FEF2F2;
  color: #EF4444;
}

.logout-link:hover .nav-icon-wrap.tone-red {
  background: #FEE2E2;
  color: #DC2626;
}

/* ===== 右侧视窗：极光渐变底 ===== */
.main-viewport {
  position: relative;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}

.main-viewport::before {
  content: '';
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(640px 340px at 12% -8%, rgba(99, 102, 241, 0.11), transparent 70%),
    radial-gradient(560px 320px at 88% -4%, rgba(56, 189, 248, 0.10), transparent 70%),
    radial-gradient(720px 460px at 55% 115%, rgba(167, 139, 250, 0.09), transparent 70%);
}

/* 页面滚动区域 */
.page-container {
  position: relative;
  z-index: 1;
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 26px 28px 32px;
}

.page-content {
  max-width: 1200px;
  margin: 0 auto;
}

/* 切换动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.fade-enter-from {
  opacity: 0;
  transform: translateY(4px);
}

.fade-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
