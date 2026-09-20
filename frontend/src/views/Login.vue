<template>
  <div class="login-page">
    <!-- ===== 左侧：品牌展示区 ===== -->
    <section class="showcase">
      <div class="showcase-bg" aria-hidden="true">
        <div class="mesh mesh-a"></div>
        <div class="mesh mesh-b"></div>
        <div class="grid-overlay"></div>
      </div>

      <div class="showcase-top">
        <div class="brand-mark">
          <div class="brand-logo"><n-icon :component="Flash" :size="16" /></div>
          <span>Fast Pay</span>
        </div>
      </div>

      <div class="showcase-body">
        <span class="eyebrow">聚合支付网关</span>
        <h1>让收款和对账<br /><span class="grad-text">变得简单可靠</span></h1>
        <p>统一管理商户订单、资金结算与渠道数据，为你的业务提供稳定的支付基础设施。</p>

        <ul class="feature-list">
          <li>
            <span class="feature-icon"><n-icon :component="FlashOutline" :size="16" /></span>
            <div><b>实时到账</b><span>订单状态毫秒级同步，异步通知自动重试</span></div>
          </li>
          <li>
            <span class="feature-icon"><n-icon :component="ShieldCheckmarkOutline" :size="16" /></span>
            <div><b>安全合规</b><span>完整签名校验体系，会话与权限严格隔离</span></div>
          </li>
          <li>
            <span class="feature-icon"><n-icon :component="StatsChartOutline" :size="16" /></span>
            <div><b>数据透明</b><span>交易流水与结算记录清晰可追溯</span></div>
          </li>
        </ul>
      </div>
    </section>

    <!-- ===== 右侧：登录表单 ===== -->
    <section class="form-side">
      <div class="form-side-bg" aria-hidden="true">
        <div class="mesh mesh-c"></div>
        <div class="dot-overlay"></div>
      </div>

      <div class="mobile-brand">
        <div class="brand-logo"><n-icon :component="Flash" :size="16" /></div>
        <span>Fast Pay</span>
      </div>

      <div class="form-card">
        <div class="form-logo">
          <div class="brand-logo lg"><n-icon :component="Flash" :size="22" /></div>
        </div>

        <div class="form-header">
          <span class="welcome-tag">欢迎回来</span>
          <h2>登录控制台</h2>
          <p>请输入管理员密码以继续访问</p>
        </div>

        <n-form ref="formRef" :model="form" :rules="rules" :show-label="false">
          <label class="field-label" for="password">管理员密码</label>
          <n-form-item path="password">
            <n-input
              id="password"
              v-model:value="form.password"
              type="password"
              show-password-on="click"
              placeholder="请输入管理员密码"
              size="large"
              :input-props="{ autocomplete: 'current-password' }"
              @keyup.enter="submit"
            >
              <template #prefix>
                <n-icon :component="LockClosedOutline" :size="16" class="field-icon" />
              </template>
            </n-input>
          </n-form-item>

          <button type="button" class="btn-primary" :disabled="loading" @click="submit">
            <span v-if="loading" class="spinner"></span>
            <span>{{ loading ? '正在验证身份...' : '登录控制台' }}</span>
            <n-icon v-if="!loading" :component="ArrowForwardOutline" :size="16" />
          </button>
        </n-form>

        <div class="trust-row">
          <div class="trust-item">
            <span class="trust-icon"><n-icon :component="ShieldCheckmarkOutline" :size="15" /></span>
            <span>加密传输</span>
          </div>
          <div class="trust-item">
            <span class="trust-icon"><n-icon :component="LockClosedOutline" :size="15" /></span>
            <span>会话隔离</span>
          </div>
          <div class="trust-item">
            <span class="trust-icon"><n-icon :component="CheckmarkCircleOutline" :size="15" /></span>
            <span>安全审计</span>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import {
  LockClosedOutline, ArrowForwardOutline, ShieldCheckmarkOutline,
  Flash, FlashOutline, StatsChartOutline, CheckmarkCircleOutline,
} from '@vicons/ionicons5'
import client, { call, REDIRECT_KEY } from '../api'
import { useUserStore } from '../stores/user'
import { message } from '../ui'

const ADMIN_USER = 'admin'

const router = useRouter()
const store = useUserStore()
const formRef = ref(null)
const loading = ref(false)

const form = reactive({ password: '' })
const rules = {
  password: [
    { required: true, message: '请输入管理员密码', trigger: 'blur' },
    { min: 6, message: '密码至少 6 位', trigger: 'blur' },
  ],
}

async function submit() {
  await formRef.value?.validate()
  loading.value = true
  try {
    const data = await call(client.authSvc.login({ username: ADMIN_USER, password: form.password }))
    store.setLogin(data.token, data.user)
    const redirect = localStorage.getItem(REDIRECT_KEY) || '/orders'
    localStorage.removeItem(REDIRECT_KEY)
    router.push(redirect)
  } catch (e) {
    message.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  min-height: 100dvh;
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
  background: #FFFFFF;
}

/* ===== 左侧展示区 ===== */
.showcase {
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  padding: 40px 56px;
  background: linear-gradient(160deg, #2563EB 0%, #1D4ED8 60%, #1E3A8A 100%);
}

.showcase-bg { position: absolute; inset: 0; overflow: hidden; }
.mesh { position: absolute; border-radius: 50%; filter: blur(90px); }
.mesh-a { width: 520px; height: 520px; top: -180px; right: -120px; background: radial-gradient(circle, rgba(125, 211, 252, 0.35), transparent 68%); }
.mesh-b { width: 460px; height: 460px; bottom: -160px; left: -120px; background: radial-gradient(circle, rgba(59, 130, 246, 0.4), transparent 68%); }
.grid-overlay {
  position: absolute;
  inset: 0;
  background-image: linear-gradient(rgba(255, 255, 255, 0.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255, 255, 255, 0.1) 1px, transparent 1px);
  background-size: 42px 42px;
  mask-image: radial-gradient(circle at 30% 30%, black 0%, transparent 75%);
  opacity: 0.9;
}

.showcase-top, .showcase-body { position: relative; z-index: 1; }

.brand-mark { display: flex; align-items: center; gap: 10px; font-size: 16px; font-weight: 700; color: #FFFFFF; }
.brand-logo {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  display: grid;
  place-items: center;
  color: #fff;
  background: linear-gradient(160deg, #3B82F6, #1D4ED8);
  box-shadow: 0 6px 16px rgba(37, 99, 235, 0.32), inset 0 1px 0 rgba(255, 255, 255, 0.28);
}

.showcase-body { max-width: 480px; margin: auto 0; padding: 40px 0; }
.eyebrow { display: inline-block; margin-bottom: 16px; padding: 5px 11px; border-radius: 999px; background: rgba(255, 255, 255, 0.14); border: 1px solid rgba(255, 255, 255, 0.25); color: #DBEAFE; font-size: 11px; font-weight: 700; letter-spacing: 0.04em; }
.showcase-body h1 { margin: 0; font-size: clamp(32px, 3.4vw, 44px); line-height: 1.22; letter-spacing: -0.03em; color: #FFFFFF; }
.grad-text { color: #BAE6FD; }
.showcase-body > p { margin: 20px 0 30px; color: rgba(219, 234, 254, 0.75); font-size: 14px; line-height: 1.75; }

.feature-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 18px; }
.feature-list li { display: flex; align-items: center; gap: 12px; }
.feature-icon {
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  border-radius: 9px;
  color: #DBEAFE;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.2);
}
.feature-icon :deep(svg) { display: block; }
.feature-list li > div { display: flex; flex-direction: column; gap: 2px; }
.feature-list b { font-size: 13px; font-weight: 600; color: #FFFFFF; line-height: 1.3; }
.feature-list li > div > span { color: rgba(219, 234, 254, 0.7); font-size: 12px; line-height: 1.4; }

/* ===== 右侧表单区 ===== */
.form-side {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 48px;
  background: #FFFFFF;
}

.form-side-bg { position: absolute; inset: 0; overflow: hidden; pointer-events: none; }
.mesh-c {
  position: absolute;
  width: 640px;
  height: 640px;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  border-radius: 50%;
  filter: blur(80px);
  background: radial-gradient(circle, rgba(37, 99, 235, 0.07), transparent 65%);
}
.dot-overlay {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(#E2E8F0 1.4px, transparent 1.4px);
  background-size: 26px 26px;
  mask-image: radial-gradient(circle at 50% 42%, black 0%, transparent 68%);
  opacity: 0.9;
}

.mobile-brand { display: none; }

.form-card {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 380px;
  padding: 40px 36px 32px;
  background: #FFFFFF;
  border: 1px solid #E2E8F0;
  border-radius: 20px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.04), 0 24px 48px -16px rgba(15, 23, 42, 0.12);
  text-align: center;
}

.form-logo { display: flex; justify-content: center; margin-bottom: 18px; }
.brand-logo.lg {
  width: 46px;
  height: 46px;
  border-radius: 13px;
}

.welcome-tag { display: inline-flex; padding: 5px 10px; border-radius: 999px; color: #1D4ED8; background: #EFF6FF; font-size: 11px; font-weight: 700; }
.form-header h2 { margin: 16px 0 6px; font-size: 24px; color: #0F172A; letter-spacing: -0.03em; }
.form-header p { margin: 0 0 26px; color: #64748B; font-size: 13px; }

.field-label { display: block; margin-bottom: 8px; color: #334155; font-size: 13px; font-weight: 500; text-align: left; }
.field-icon { display: flex; align-items: center; color: #94A3B8; }

.btn-primary {
  width: 100%;
  height: 44px;
  border-radius: 10px;
  background: #2563EB;
  color: #fff;
  border: none;
  font-size: 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  transition: all 0.18s ease;
  box-shadow: 0 4px 14px rgba(37, 99, 235, 0.28);
  margin-top: 6px;
}
.btn-primary:hover:not(:disabled) { background: #1D4ED8; box-shadow: 0 6px 18px rgba(37, 99, 235, 0.36); transform: translateY(-1px); }
.btn-primary:active:not(:disabled) { transform: translateY(0) scale(0.99); }
.btn-primary:disabled { opacity: 0.7; cursor: not-allowed; }

.spinner { width: 14px; height: 14px; border: 2px solid rgba(255, 255, 255, 0.35); border-top-color: #fff; border-radius: 50%; animation: spin 0.6s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.trust-row { display: flex; align-items: stretch; justify-content: space-between; gap: 8px; margin-top: 26px; padding-top: 20px; border-top: 1px solid #F1F5F9; }
.trust-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 12px 4px;
  border-radius: 12px;
  background: #F8FAFC;
  border: 1px solid #F1F5F9;
  color: #64748B;
  font-size: 10px;
}
.trust-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 8px;
  background: #EFF6FF;
  color: #2563EB;
  line-height: 0;
}
.trust-icon :deep(svg) { display: block; }

@media (max-width: 980px) {
  .login-page { grid-template-columns: 1fr; }
  .showcase { display: none; }
  .form-side { min-height: 100vh; align-items: center; justify-content: flex-start; padding-top: 110px; background: linear-gradient(160deg, #2563EB 0%, #1E3A8A 100%); }
  .mobile-brand { position: absolute; top: 28px; left: 28px; z-index: 1; display: flex; align-items: center; gap: 10px; font-weight: 700; color: #FFFFFF; }
}

@media (max-width: 460px) {
  .form-side { padding: 96px 20px 40px; }
  .form-card { padding: 32px 24px 26px; }
  .form-header h2 { font-size: 22px; }
}
</style>
