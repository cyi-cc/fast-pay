<template>
  <div class="profile-page">
    <!-- ===== 商户接入凭证：品牌渐变卡，与登录页风格呼应 ===== -->
    <section class="merchant-card">
      <div class="merchant-dots" aria-hidden="true"></div>
      <div class="cred-block">
        <div class="cred-head">
          <span class="cred-icon"><n-icon :component="KeyOutline" :size="15" /></span>
          <h2 class="cred-title">商户接入凭证</h2>
        </div>
        <div class="cred-list">
          <div class="cred-row">
            <span class="cred-label">用户 ID</span>
            <span class="cred-value mono">{{ app?.pid ?? store.profile?.id ?? '—' }}</span>
            <button class="btn-copy" title="复制用户 ID" aria-label="复制用户 ID" @click="copy(app?.pid ?? store.profile?.id)">
              <n-icon :component="CopyOutline" :size="13" />
            </button>
          </div>
          <div class="cred-row">
            <span class="cred-label">易支付密钥</span>
            <span class="cred-value mono cred-key">{{ app?.appKey || '—' }}</span>
            <button v-if="app" class="btn-copy" title="复制密钥" aria-label="复制密钥" @click="copy(app.appKey)">
              <n-icon :component="CopyOutline" :size="13" />
            </button>
            <button v-else class="btn-gen" :disabled="creating" @click="genKey">
              {{ creating ? '生成中...' : '生成密钥' }}
            </button>
          </div>
        </div>
      </div>

      <div class="balance-pane">
        <template v-if="store.isAdmin && up.configured">
          <span class="balance-title">上游可提现余额</span>
          <div class="balance-num tnum">{{ up.walletReady ? `¥${fen2yuan(up.upAvailable)}` : '—' }}</div>
          <span class="balance-sub">冻结金额 {{ up.walletReady ? `¥${fen2yuan(up.upFrozen)}` : '—' }}</span>
        </template>
        <template v-else>
          <span class="balance-title">账户可用余额</span>
          <div class="balance-num tnum">¥{{ fen2yuan(store.profile?.balance) }}</div>
        </template>
      </div>
    </section>

    <!-- ===== 配置卡片：两列布局，左列=上游登录/测试支付，右列=密码/库存 ===== -->
    <div class="profile-grid">
      <div class="profile-col">
      <section v-if="store.isAdmin" class="card detail-card login-card">
        <div class="card-head">
          <div class="head-icon tone-blue">
            <n-icon :component="LogInOutline" :size="16" />
          </div>
          <div>
            <h3 class="head-title">上游账号登录器</h3>
            <p class="head-desc">登录上游商户，令牌失效后自动重登</p>
          </div>
        </div>

        <div v-if="externalResult" class="login-result">
          <div class="result-mark">
            <n-icon :component="CheckmarkCircleOutline" :size="22" />
          </div>
          <div class="result-title">已连接上游平台</div>
          <div class="result-list">
            <div class="result-row"><span>登录账号</span><b>{{ externalResult.account }}</b></div>
            <div class="result-row"><span>商户昵称</span><b>{{ externalResult.nickname || '—' }}</b></div>
            <div class="result-row"><span>店铺标识</span><b class="mono">{{ externalResult.shop || '—' }}</b></div>
            <div class="result-row"><span>令牌状态</span><b :class="{ 'success-text': up.tokenAge >= 0 }">{{ up.tokenAge >= 0 ? '已连接' : '待登录' }}</b></div>
          </div>
          <n-button secondary block @click="resetExternalLogin">重新登录</n-button>
        </div>

        <n-form v-else ref="externalFormRef" :model="externalForm" :rules="externalRules" label-placement="top" class="security-form">
          <n-form-item label="账号" path="account">
            <n-input v-model:value="externalForm.account" placeholder="请输入登录账号" autocomplete="username" />
          </n-form-item>
          <n-form-item label="密码" path="password">
            <n-input
              v-model:value="externalForm.password"
              type="password"
              show-password-on="click"
              :placeholder="up.configured ? '留空则复用已保存密码' : '请输入登录密码'"
              :input-props="{ autocomplete: 'current-password' }"
              @keyup.enter="externalLogin"
            />
          </n-form-item>
          <n-form-item label="代理 API（选填）" path="proxyApi">
            <n-input
              v-model:value="externalForm.proxyApi"
              placeholder="代理取号接口地址，留空则直连"
              @keyup.enter="externalLogin"
            />
          </n-form-item>
          <div v-if="externalError" class="login-error">{{ externalError }}</div>
          <n-button type="primary" block :loading="externalLoading" @click="externalLogin">登录</n-button>
        </n-form>
      </section>

      <!-- ===== 测试支付（管理员） ===== -->
      <section v-if="store.isAdmin" class="card detail-card">
        <div class="card-head">
          <div class="head-icon tone-amber">
            <n-icon :component="CardOutline" :size="16" />
          </div>
          <div class="head-main">
            <h3 class="head-title">测试支付</h3>
            <p class="head-desc">输入金额走真实下单链路，弹出微信扫码支付二维码</p>
          </div>
        </div>
        <div class="testpay-body">
          <div class="testpay-form">
            <n-input v-model:value="testMoney" class="money-input" placeholder="金额（元）" @keyup.enter="startTestPay">
              <template #prefix>¥</template>
            </n-input>
            <n-button type="primary" :loading="testPaying" @click="startTestPay">发起支付</n-button>
          </div>
          <p class="testpay-tip">微信扫码支付，订单有效期 10 分钟</p>
        </div>
      </section>
      </div>

      <div class="profile-col">
      <section class="card detail-card">
        <div class="card-head">
          <div class="head-icon tone-violet">
            <n-icon :component="ShieldCheckmarkOutline" :size="16" />
          </div>
          <div>
            <h3 class="head-title">修改密码</h3>
            <p class="head-desc">修改管理密码以确保账户安全</p>
          </div>
        </div>

        <n-form ref="formRef" :model="form" :rules="rules" label-placement="top" class="security-form">
          <n-form-item label="当前密码" path="oldPassword">
            <n-input
              v-model:value="form.oldPassword"
              type="password"
              show-password-on="click"
              placeholder="请输入当前密码"
              :input-props="{ autocomplete: 'current-password' }"
            />
          </n-form-item>

          <div class="form-cols">
            <n-form-item label="新密码" path="newPassword">
              <n-input
                v-model:value="form.newPassword"
                type="password"
                show-password-on="click"
                placeholder="至少 6 位字符"
                :input-props="{ autocomplete: 'new-password' }"
              />
            </n-form-item>
            <n-form-item label="确认新密码" path="confirm">
              <n-input
                v-model:value="form.confirm"
                type="password"
                show-password-on="click"
                placeholder="再次输入新密码"
                :input-props="{ autocomplete: 'new-password' }"
                @keyup.enter="submit"
              />
            </n-form-item>
          </div>

          <div class="form-actions">
            <span class="action-tip">修改成功后将退出并需重新登录</span>
            <n-button type="primary" :loading="saving" @click="submit">确认更新密码</n-button>
          </div>
        </n-form>
      </section>

    <!-- ===== 库存配置（管理员） ===== -->
    <section v-if="store.isAdmin" class="card detail-card upstream-card">
      <div class="card-head">
        <div class="head-icon tone-green">
          <n-icon :component="CubeOutline" :size="16" />
        </div>
        <div class="head-main">
          <h3 class="head-title">库存配置</h3>
          <p class="head-desc">配置上游兑换码库存对应的总金额</p>
        </div>
      </div>

      <div class="upstream-body">
        <div class="stock-row">
          <span class="stock-label">库存金额</span>
          <n-input v-model:value="stockAmount" class="stock-input" size="small" placeholder="1000" aria-label="库存金额">
            <template #prefix>¥</template>
          </n-input>
          <n-button size="small" secondary :loading="stockSaving" @click="saveStock">保存</n-button>
          <span class="stock-hint">库存不足时自动补充到该金额，默认 ¥1000</span>
        </div>
      </div>
    </section>
      </div>
    </div>

    <!-- 支付二维码弹窗 -->
    <n-modal v-model:show="qrVisible" :mask-closable="false" @update:show="onQrToggle">
      <div class="qr-dialog">
        <button class="qr-close" aria-label="关闭" @click="qrVisible = false">
          <n-icon :component="CloseOutline" :size="14" />
        </button>

        <div class="qr-brand">
          <span class="qr-wechat"><n-icon :component="LogoWechat" :size="20" /></span>
          <span class="qr-title">微信扫码支付</span>
        </div>
        <div class="qr-order mono">订单 {{ testResult?.tradeNo }}</div>

        <div class="qr-amount tnum"><em>¥</em>{{ fen2yuan(testResult?.money) }}</div>

        <div class="qr-box">
          <i class="qr-corner tl"></i><i class="qr-corner tr"></i>
          <i class="qr-corner bl"></i><i class="qr-corner br"></i>
          <n-qr-code v-if="testResult?.qrcode" :value="testResult.qrcode" :size="188" color="#0F172A" />
        </div>

        <div class="qr-tip">请使用微信「扫一扫」完成支付</div>
        <div class="qr-foot">
          <n-icon :component="TimeOutline" :size="12" />
          <span>二维码 10 分钟内有效</span>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ShieldCheckmarkOutline, KeyOutline, CopyOutline, LogInOutline, CheckmarkCircleOutline, CubeOutline, CardOutline, CloseOutline, LogoWechat, TimeOutline } from '@vicons/ionicons5'
import client, { call } from '../../api'
import { useUserStore } from '../../stores/user'
import { fen2yuan } from '../../utils/format'
import { message } from '../../ui'

const store = useUserStore()
const router = useRouter()
const formRef = ref(null)
const saving = ref(false)

const app = ref(null)
const creating = ref(false)
const externalFormRef = ref(null)
const externalLoading = ref(false)
const externalError = ref('')
const externalResult = ref(null)
const externalForm = reactive({ account: '', password: '', proxyApi: '' })
const externalRules = {
  account: { required: true, message: '请输入登录账号', trigger: ['blur', 'input'] },
  password: {
    validator: (_rule, value) => up.value.configured === 1 || !!value,
    message: '首次登录请输入密码',
    trigger: ['blur', 'input'],
  },
}

// 上游对接状态（仅管理员可用 upstreamSvc）
const up = ref({ configured: 0, username: '', nickname: '', shop: '', goodsKey: '', goodsID: 0, goodsName: '', unitPrice: 0, stockAmount: 100000, upAvailable: 0, upFrozen: 0, walletReady: 0, tokenAge: -1, proxyApi: '' })
const stockAmount = ref('')
const stockSaving = ref(false)

// 测试支付
const testMoney = ref('')
const testPaying = ref(false)
const qrVisible = ref(false)
const testResult = ref(null)
let statusTimer = null
let statusStartedAt = 0
let statusChecking = false

async function startTestPay() {
  const money = testMoney.value.trim()
  if (!money) {
    message.warning('请输入金额')
    return
  }
  testPaying.value = true
  try {
    testResult.value = await call(client.upstreamSvc.testPay({ money }))
    qrVisible.value = true
    startStatusPolling(testResult.value.tradeNo)
  } catch (e) {
    message.error(e.message)
  } finally {
    testPaying.value = false
  }
}

function startStatusPolling(tradeNo) {
  stopStatusPolling()
  if (!tradeNo) return
  statusStartedAt = Date.now()
  const check = async () => {
    if (statusChecking) return
    if (Date.now() - statusStartedAt >= 10 * 60 * 1000) {
      qrVisible.value = false
      stopStatusPolling()
      message.warning('二维码已过期')
      return
    }
    statusChecking = true
    try {
      const st = await call(client.cashierSvc.status({ tradeNo }))
      if (!qrVisible.value) return
      if (st.status === 1) {
        qrVisible.value = false
        stopStatusPolling()
        message.success('支付成功')
      } else if (st.status !== 0) {
        qrVisible.value = false
        stopStatusPolling()
        message.warning(st.status === 2 ? '订单已退款' : '订单已关闭')
      }
    } catch { /* 网络抖动继续轮询 */ }
    finally { statusChecking = false }
  }
  void check()
  statusTimer = setInterval(check, 3000)
}

function stopStatusPolling() {
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
}



function onQrToggle(v) {
  if (!v) stopStatusPolling()
}

onUnmounted(stopStatusPolling)

onMounted(async () => {
  try {
    const apps = await call(client.appSvc.list())
    app.value = apps?.[0] || null
  } catch {
    /* 应用列表加载失败时仅不展示密钥 */
  }
  if (store.isAdmin) {
    await loadUpstream()
  }
})

async function loadUpstream() {
  try {
    const st = await call(client.upstreamSvc.status())
    up.value = st
    stockAmount.value = fen2yuan(st.stockAmount) || ''
    if (st.configured) {
      externalForm.account = st.username
      externalForm.proxyApi = st.proxyApi || ''
      externalResult.value = { account: st.username, nickname: st.nickname, shop: st.shop }
    }
  } catch {
    /* 未配置上游时保持登录表单 */
  }
}

async function genKey() {
  creating.value = true
  try {
    app.value = await call(client.appSvc.create({ name: '默认应用' }))
    message.success('密钥已生成')
  } catch (e) {
    message.error(e.message)
  } finally {
    creating.value = false
  }
}

function copy(v) {
  if (v === null || v === undefined || v === '') return
  navigator.clipboard?.writeText(String(v))
    .then(() => message.success('已复制'))
    .catch(() => message.error('复制失败'))
}

async function externalLogin() {
  externalError.value = ''
  try {
    await externalFormRef.value?.validate()
  } catch {
    externalError.value = '请检查账号和密码'
    return
  }
  externalLoading.value = true
  try {
    const st = await call(client.upstreamSvc.saveAccount({
      username: externalForm.account.trim(),
      password: externalForm.password,
      proxyAPI: externalForm.proxyApi.trim(),
    }))
    up.value = st
    externalResult.value = { account: st.username, nickname: st.nickname, shop: st.shop }
    message.success('上游登录成功，正在检查兑换码商品')
    void refreshAutoGoods()
  } catch (e) {
    externalError.value = e.message
  } finally {
    externalLoading.value = false
  }
}

function resetExternalLogin() {
  externalResult.value = null
  externalError.value = ''
  externalForm.account = up.value.username || ''
  externalForm.password = ''
  externalForm.proxyApi = up.value.proxyApi || ''
}

async function refreshAutoGoods() {
  for (let i = 0; i < 10; i++) {
    await new Promise(resolve => setTimeout(resolve, 1500))
    try {
      const st = await call(client.upstreamSvc.status())
      up.value = st
      stockAmount.value = fen2yuan(st.stockAmount)
      if (st.goodsKey) return
    } catch { /* 自动建品仍在后台执行 */ }
  }
}

async function saveStock() {
  stockSaving.value = true
  try {
    up.value = await call(client.upstreamSvc.setStock({ stockAmount: stockAmount.value.trim() }))
    message.success(`库存目标已设为 ¥${fen2yuan(up.value.stockAmount)}`)
  } catch (e) {
    message.error(e.message)
  } finally {
    stockSaving.value = false
  }
}





const form = reactive({ oldPassword: '', newPassword: '', confirm: '' })
const rules = {
  oldPassword: { required: true, message: '请输入当前密码', trigger: 'blur' },
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '至少 6 位', trigger: 'blur' },
  ],
  confirm: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: _ => form.confirm === form.newPassword, message: '两次密码不一致', trigger: 'blur' },
  ],
}

async function submit() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  saving.value = true
  try {
    await call(client.userSvc.changePassword({ oldPassword: form.oldPassword, newPassword: form.newPassword }))
    message.success('密码已修改，请重新登录')
    await store.logout()
    router.push('/login')
  } catch (e) {
    message.error(e.message)
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.profile-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 1100px;
}

/* ===== 商户概览：企业蓝名片 ===== */
.merchant-card {
  position: relative;
  overflow: hidden;
  color: #FFFFFF;
  background:
    radial-gradient(420px 220px at 96% -40%, rgba(125, 211, 252, 0.32), transparent 65%),
    linear-gradient(160deg, #3B82F6 0%, #1D4ED8 100%);
  border-radius: 18px;
  padding: 26px 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  box-shadow: 0 14px 32px -12px rgba(29, 78, 216, 0.45), inset 0 1px 0 rgba(255, 255, 255, 0.18);
}

.merchant-dots {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(255, 255, 255, 0.45) 1.2px, transparent 1.2px);
  background-size: 18px 18px;
  mask-image: radial-gradient(circle at 95% 0%, black 0%, transparent 65%);
  opacity: 0.5;
  pointer-events: none;
}

/* 接入凭证 */
.cred-block {
  position: relative;
  z-index: 1;
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.cred-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cred-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.16);
  border: 1px solid rgba(255, 255, 255, 0.24);
  color: #FFFFFF;
  display: flex;
  align-items: center;
  justify-content: center;
}

.cred-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #FFFFFF;
  letter-spacing: -0.01em;
}

.cred-list {
  display: flex;
  flex-direction: column;
}

.cred-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.14);
}

.cred-row:last-child {
  border-bottom: none;
}

.cred-label {
  width: 72px;
  flex-shrink: 0;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.72);
}

.cred-value {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: #FFFFFF;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cred-key {
  letter-spacing: 0.02em;
}

.btn-copy {
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.14);
  border: 1px solid rgba(255, 255, 255, 0.22);
  color: #FFFFFF;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 0.15s ease;
}

.btn-copy:hover {
  background: rgba(255, 255, 255, 0.26);
}

.btn-gen {
  flex-shrink: 0;
  height: 26px;
  padding: 0 10px;
  border-radius: 7px;
  border: 1px solid rgba(255, 255, 255, 0.3);
  background: rgba(255, 255, 255, 0.16);
  color: #FFFFFF;
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease;
}

.btn-gen:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.26);
}

.btn-gen:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

.balance-pane {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  padding: 14px 20px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.14);
  border: 1px solid rgba(255, 255, 255, 0.24);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.balance-title {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.72);
  margin-bottom: 2px;
}

.balance-num {
  font-size: 27px;
  font-weight: 600;
  color: #FFFFFF;
  letter-spacing: -0.02em;
}

.balance-sub {
  margin-top: 2px;
  font-size: 11px;
  color: rgba(255, 255, 255, 0.7);
}

/* ===== 双列布局 ===== */
.profile-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.profile-col {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.detail-card {
  background: #FFFFFF;
  border: 1px solid var(--line);
  border-radius: 16px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.03);
  overflow: hidden;
}

.card-head {
  padding: 16px 20px;
  border-bottom: 1px solid #F1F5F9;
  display: flex;
  align-items: center;
  gap: 10px;
}

.head-icon {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.head-icon.tone-blue {
  background: #EFF6FF;
  border: 1px solid #DBEAFE;
  color: #2563EB;
}

.head-icon.tone-violet {
  background: #F5F3FF;
  border: 1px solid #E9E5FC;
  color: #7C3AED;
}

.head-icon.tone-green {
  background: #ECFDF5;
  border: 1px solid #D1FAE5;
  color: #059669;
}

.head-icon.tone-amber {
  background: #FFFBEB;
  border: 1px solid #FDE68A;
  color: #D97706;
}

/* ===== 测试支付 ===== */
.testpay-body {
  padding: 16px 20px 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.testpay-form {
  display: flex;
  gap: 8px;
}

.money-input {
  max-width: 220px;
}

.testpay-tip {
  margin: 0;
  font-size: 11px;
  color: #94A3B8;
}

/* ===== 二维码弹窗 ===== */
.qr-dialog {
  position: relative;
  width: 340px;
  background: #FFFFFF;
  border-radius: 20px;
  padding: 26px 28px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  box-shadow: 0 24px 64px -12px rgba(15, 23, 42, 0.3), 0 0 0 1px rgba(15, 23, 42, 0.05);
}

.qr-close {
  position: absolute;
  top: 14px;
  right: 14px;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 50%;
  background: #F1F5F9;
  color: #64748B;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
}

.qr-close:hover {
  background: #E2E8F0;
  color: #0F172A;
}

.qr-brand {
  display: flex;
  align-items: center;
  gap: 10px;
}

.qr-wechat {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: #07C160;
  color: #FFFFFF;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 18px -4px rgba(7, 193, 96, 0.5);
}

.qr-title {
  font-size: 16px;
  font-weight: 600;
  color: #0F172A;
}

.qr-order {
  margin-top: 8px;
  font-size: 11px;
  color: #94A3B8;
  background: #F8FAFC;
  border: 1px solid #F1F5F9;
  border-radius: 999px;
  padding: 3px 12px;
  letter-spacing: 0.02em;
}

.qr-amount {
  margin-top: 16px;
  font-size: 34px;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: #0F172A;
  display: flex;
  align-items: baseline;
  gap: 3px;
}

.qr-amount em {
  font-style: normal;
  font-size: 20px;
  font-weight: 600;
  color: #475569;
}

.qr-box {
  position: relative;
  margin-top: 16px;
  width: 232px;
  height: 232px;
  border-radius: 16px;
  background: linear-gradient(160deg, #F0FDF4 0%, #ECFDF5 50%, #F8FAFC 100%);
  border: 1px solid #D1FAE5;
  display: flex;
  align-items: center;
  justify-content: center;
}

/* n-qr-code 自带白边与边框，去掉让码图与角标对齐 */
.qr-box :deep(.n-qr-code) {
  padding: 0 !important;
  border: none !important;
  border-radius: 0 !important;
  background: transparent !important;
}

.qr-corner {
  position: absolute;
  width: 18px;
  height: 18px;
  border: 2.5px solid #07C160;
  border-radius: 2px;
}

.qr-corner.tl { top: -1px; left: -1px; border-right: none; border-bottom: none; border-top-left-radius: 8px; }
.qr-corner.tr { top: -1px; right: -1px; border-left: none; border-bottom: none; border-top-right-radius: 8px; }
.qr-corner.bl { bottom: -1px; left: -1px; border-right: none; border-top: none; border-bottom-left-radius: 8px; }
.qr-corner.br { bottom: -1px; right: -1px; border-left: none; border-top: none; border-bottom-right-radius: 8px; }

.qr-tip {
  margin-top: 16px;
  font-size: 13px;
  font-weight: 500;
  color: #334155;
}

.qr-foot {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 11px;
  color: #94A3B8;
}

/* ===== 上游商品绑定 ===== */
.upstream-card .card-head {
  justify-content: space-between;
}

.head-main {
  flex: 1;
  min-width: 0;
}

.upstream-body {
  padding: 16px 20px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.stock-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.stock-label {
  font-size: 12px;
  font-weight: 500;
  color: #475569;
  flex-shrink: 0;
}

.stock-input {
  width: 140px;
}

.stock-hint {
  font-size: 11px;
  color: #94A3B8;
}

.head-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: #0F172A;
}

.head-desc {
  margin: 2px 0 0;
  font-size: 11px;
  color: #94A3B8;
}

/* 登录结果 */
.login-result {
  padding: 22px 20px 20px;
  text-align: center;
}

.result-mark {
  width: 44px;
  height: 44px;
  margin: 0 auto 10px;
  border-radius: 13px;
  background: #ECFDF5;
  border: 1px solid #A7F3D0;
  color: #059669;
  display: flex;
  align-items: center;
  justify-content: center;
}

.result-title {
  margin-bottom: 16px;
  color: #0F172A;
  font-size: 15px;
  font-weight: 600;
}

.result-list {
  margin-bottom: 16px;
  padding: 4px 14px;
  border-radius: 10px;
  background: #F8FAFC;
  border: 1px solid #F1F5F9;
  text-align: left;
}

.result-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 9px 0;
  border-bottom: 1px solid #E2E8F0;
  color: #64748B;
  font-size: 12px;
}

.result-row:last-child {
  border-bottom: none;
}

.result-row b {
  color: #0F172A;
  font-weight: 500;
}

.result-row .success-text {
  color: #059669;
}

.login-error {
  margin: -2px 0 12px;
  padding: 8px 10px;
  border: 1px solid #FECACA;
  border-radius: 8px;
  background: #FEF2F2;
  color: #B91C1C;
  font-size: 12px;
}

/* 表单 */
.security-form {
  padding: 16px 20px 20px;
}

.form-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.form-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  padding-top: 14px;
  border-top: 1px solid #F1F5F9;
}

.action-tip {
  font-size: 11px;
  color: #94A3B8;
}

@media (max-width: 860px) {
  .profile-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 600px) {
  .merchant-card {
    flex-direction: column;
    align-items: flex-start;
  }
  .balance-pane {
    align-items: flex-start;
    width: 100%;
  }
  .form-cols {
    grid-template-columns: 1fr;
  }
  .form-actions {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
}
</style>
