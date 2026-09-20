<template>
  <div class="orders-page">
    <!-- ===== 核心指标：品牌渐变主卡 + 清爽次卡 ===== -->
    <section class="metrics-grid">
      <div class="metric-card hero-metric">
        <div class="hero-dots" aria-hidden="true"></div>
        <div class="metric-top">
          <span class="metric-label">今日交易额</span>
          <div class="metric-icon hero-icon">
            <n-icon :component="CashOutline" :size="16" />
          </div>
        </div>
        <div class="metric-val tnum">¥{{ fen2yuan(stats.todayMoney) }}</div>
        <div class="metric-foot">
          <span class="metric-tag hero-tag">今日累计成交</span>
        </div>
      </div>

      <div v-for="m in metricCards" :key="m.label" :class="['metric-card', 'wash-' + m.tone]">
        <div class="metric-top">
          <span class="metric-label">{{ m.label }}</span>
          <div class="metric-icon" :class="m.tone">
            <n-icon :component="m.icon" :size="16" />
          </div>
        </div>
        <div class="metric-val tnum">{{ m.value }}</div>
        <div class="metric-foot">
          <span class="metric-tag">{{ m.sub }}</span>
        </div>
      </div>
    </section>

    <!-- ===== 订单列表面板 ===== -->
    <section class="table-card">
      <div class="table-header">
        <div class="header-left">
          <h2 class="section-title">订单明细</h2>
          <span class="count-badge">{{ pagination.itemCount }} 笔记录</span>
        </div>
        <button class="btn-refresh" title="刷新数据" aria-label="刷新数据" @click="reload">
          <n-icon :component="RefreshOutline" :size="15" />
          <span>刷新</span>
        </button>
      </div>

      <!-- 搜索筛选栏 -->
      <div class="filter-bar">
        <n-input
          v-model:value="filters.kw"
          class="search-input"
          placeholder="搜索订单号"
          aria-label="搜索订单号"
          clearable
          @keyup.enter="reload"
        >
          <template #prefix><n-icon :component="SearchOutline" :size="14" class="search-icon" /></template>
        </n-input>
        <n-select v-model:value="filters.status" :options="statusOptions" placeholder="全部状态" aria-label="按状态筛选" clearable class="filter-select" />
        <div class="filter-actions">
          <n-button type="primary" size="medium" @click="reload">
            <template #icon><n-icon :component="FilterOutline" :size="14" /></template>
            筛选
          </n-button>
          <n-button quaternary size="medium" @click="reset">重置</n-button>
        </div>
      </div>

      <!-- 数据表格 -->
      <div class="table-wrapper">
        <n-data-table
          remote
          :bordered="false"
          :single-line="false"
          :columns="columns"
          :data="rows"
          :loading="loading"
          :pagination="pagination"
          :row-key="r => r.id"
        >
          <template #empty>
            <div class="table-empty">
              <div class="empty-icon-wrap">
                <n-icon :component="FileTrayOutline" :size="24" />
              </div>
              <span class="empty-title">暂无订单数据</span>
              <span class="empty-desc">
                {{ hasFilter ? '没有找到符合当前筛选条件的订单' : '发起支付并完成后，订单将在此展示' }}
              </span>
              <n-button v-if="hasFilter" size="small" secondary @click="reset">清空筛选</n-button>
            </div>
          </template>
        </n-data-table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, reactive, h, computed, onMounted } from 'vue'
import {
  CashOutline,
  WalletOutline,
  ReceiptOutline,
  LayersOutline,
  RefreshOutline,
  SearchOutline,
  FilterOutline,
  FileTrayOutline,
} from '@vicons/ionicons5'
import client, { call } from '../../api'
import { fen2yuan, fmtTime, orderStatusMap } from '../../utils/format'
import { message } from '../../ui'

const loading = ref(false)
const rows = ref([])
const stats = ref({})
const filters = reactive({ kw: '', status: null })

const hasFilter = computed(() => !!filters.kw || filters.status !== null)

const metricCards = computed(() => [
  { label: '累计收入', value: '¥' + fen2yuan(stats.value.totalIncome), sub: '历史累计成交', icon: WalletOutline, tone: 'tone-green' },
  { label: '今日订单数', value: String(Number(stats.value.todayCount ?? 0)), sub: '今日付款订单', icon: ReceiptOutline, tone: 'tone-amber' },
  { label: '累计订单总数', value: String(Number(stats.value.totalCount ?? 0)), sub: '历史所有订单', icon: LayersOutline, tone: 'tone-purple' },
])

const statusOptions = Object.entries(orderStatusMap).map(([v, m]) => ({ label: m.label, value: Number(v) }))

const pagination = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: page => { pagination.page = page; load() },
  onUpdatePageSize: size => { pagination.pageSize = size; pagination.page = 1; load() },
})

const STATUS_TONE = { 0: 'warning', 1: 'success', 2: 'info', 3: 'error' }

function statusPill(label, tone) {
  return h('span', { class: `status-pill ${tone}` }, [h('i'), label])
}

const columns = [
  {
    title: '平台单号', key: 'tradeNo', width: 220,
    render: r => h('span', { class: 'mono cell-code' }, r.tradeNo),
  },
  {
    title: '订单金额', key: 'money', width: 110, align: 'right',
    render: r => h('span', { class: 'tnum cell-money' }, '¥' + fen2yuan(r.money)),
  },
  {
    title: '支付状态', key: 'status', width: 95,
    render: r => statusPill(orderStatusMap[r.status]?.label ?? r.status, STATUS_TONE[r.status] || 'info'),
  },
  {
    title: '通知', key: 'notified', width: 95,
    render: r => {
      if (r.status !== 1) return h('span', { class: 'cell-empty' }, '—')
      if (!r.notifyUrl) return statusPill('未配置', 'info')
      if (r.notified === 1) return statusPill('已推送', 'success')
      if (Number(r.notifyAttempts) >= 5) return statusPill('推送失败', 'error')
      return statusPill('推送中', 'warning')
    },
  },
  {
    title: '创建时间', key: 'createdAt', width: 150,
    render: r => h('span', { class: 'tnum cell-sub' }, fmtTime(r.createdAt)),
  },
]

function buildDto() {
  const dto = { page: pagination.page, pageSize: pagination.pageSize }
  if (filters.kw) dto.kw = filters.kw
  if (filters.status !== null && filters.status !== undefined && filters.status !== '') dto.status = filters.status
  return dto
}

async function load() {
  loading.value = true
  try {
    const data = await call(client.orderSvc.list(buildDto()))
    rows.value = data.items || []
    pagination.itemCount = Number(data.total || 0)
  } catch (e) {
    rows.value = []
    pagination.itemCount = 0
    message.error(e.message || '订单加载失败')
  } finally {
    loading.value = false
  }
}

async function loadStats() {
  try {
    stats.value = (await call(client.orderSvc.stats())) || {}
  } catch (e) {
    message.error(e.message || '经营数据加载失败')
  }
}

function reload() {
  pagination.page = 1
  load()
  loadStats()
}

function reset() {
  filters.kw = ''
  filters.status = null
  reload()
}

onMounted(() => { load(); loadStats() })
</script>

<style scoped>
.orders-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ===== 核心指标卡：清爽四列纯白卡片 ===== */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

.metric-card {
  position: relative;
  background: #FFFFFF;
  border: 1px solid var(--line);
  border-radius: 16px;
  padding: 18px 20px 16px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.03);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, transform 0.15s ease;
}

.metric-card:hover {
  border-color: #CBD5E1;
  box-shadow: 0 8px 20px -6px rgba(15, 23, 42, 0.1);
  transform: translateY(-2px);
}

/* 次卡的同色系浅晕染，与图标 chip 呼应 */
.wash-tone-green { background: #FFFFFF; }
.wash-tone-amber { background: #FFFFFF; }
.wash-tone-purple { background: #FFFFFF; }

/* 企业蓝主指标卡 */
.hero-metric {
  color: #FFFFFF;
  background:
    radial-gradient(300px 140px at 100% -20%, rgba(125, 211, 252, 0.3), transparent 65%),
    linear-gradient(160deg, #3B82F6 0%, #1D4ED8 100%);
  border: none;
  box-shadow: 0 10px 24px -8px rgba(29, 78, 216, 0.4);
}
.hero-metric:hover {
  box-shadow: 0 14px 28px -8px rgba(29, 78, 216, 0.45);
}
.hero-dots {
  position: absolute;
  inset: 0;
  background-image: radial-gradient(rgba(255, 255, 255, 0.5) 1.2px, transparent 1.2px);
  background-size: 16px 16px;
  mask-image: radial-gradient(circle at 100% 0%, black 0%, transparent 60%);
  opacity: 0.5;
  pointer-events: none;
}
.hero-metric .metric-label { color: rgba(255, 255, 255, 0.82); position: relative; z-index: 1; }
.hero-metric .metric-val { color: #FFFFFF; position: relative; z-index: 1; }
.hero-icon {
  background: rgba(255, 255, 255, 0.18) !important;
  border: 1px solid rgba(255, 255, 255, 0.22) !important;
  color: #FFFFFF !important;
  position: relative;
  z-index: 1;
}
.hero-tag { color: rgba(255, 255, 255, 0.68) !important; position: relative; z-index: 1; }

.metric-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.metric-label {
  font-size: 13px;
  color: #64748B;
  font-weight: 500;
}

.metric-icon {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: #F8FAFC;
  border: 1px solid #F1F5F9;
  color: #475569;
  display: flex;
  align-items: center;
  justify-content: center;
}

.metric-icon.tone-green { background: #ECFDF5; border-color: #D1FAE5; color: #059669; }
.metric-icon.tone-amber { background: #FFFBEB; border-color: #FDE68A; color: #D97706; }
.metric-icon.tone-purple { background: #F5F3FF; border-color: #E9D5FF; color: #7C3AED; }

.metric-val {
  font-size: 24px;
  font-weight: 600;
  color: #0F172A;
  line-height: 1.2;
  letter-spacing: -0.02em;
}

.metric-foot {
  margin-top: 8px;
  display: flex;
  align-items: center;
}

.metric-tag {
  font-size: 11px;
  color: #94A3B8;
}

/* ===== 订单列表卡片 ===== */
.table-card {
  background: #FFFFFF;
  border: 1px solid var(--line);
  border-radius: 16px;
  box-shadow: 0 1px 2px rgba(15, 23, 42, 0.03);
  overflow: hidden;
}

.table-header {
  padding: 16px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid #F1F5F9;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: #0F172A;
}

.count-badge {
  font-size: 11px;
  color: #64748B;
  background: #F1F5F9;
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 500;
}

.btn-refresh {
  height: 30px;
  padding: 0 10px;
  border-radius: 6px;
  background: #FFFFFF;
  border: 1px solid #E2E8F0;
  color: #475569;
  font-size: 12px;
  font-weight: 500;
  display: flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-refresh:hover {
  background: #F8FAFC;
  color: #0F172A;
  border-color: #CBD5E1;
}

/* 筛选工具栏 */
.filter-bar {
  padding: 12px 20px;
  background: #FAFAFA;
  border-bottom: 1px solid #F1F5F9;
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.search-input {
  width: 260px;
}

.search-icon {
  color: #94A3B8;
}

.filter-select {
  width: 130px;
}

.filter-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

/* 表格主体 */
.table-wrapper {
  padding: 0 8px 12px;
}

:deep(.n-data-table-th) {
  height: 40px;
  font-size: 11px;
  letter-spacing: 0.04em;
  background: transparent !important;
  color: #94A3B8 !important;
  font-weight: 600;
  border-bottom: 1px solid var(--line) !important;
}

:deep(.n-data-table-td) {
  height: 48px;
  padding-top: 8px;
  padding-bottom: 8px;
  font-size: 13px;
  border-bottom: 1px solid #F1F5F9 !important;
}

:deep(.n-data-table-tr:hover .n-data-table-td) {
  background-color: #F8FAFC !important;
}

:deep(.n-data-table__pagination) {
  padding: 14px 12px 6px;
}

.cell-code {
  color: #0F172A;
  font-size: 12px;
  font-weight: 500;
}

.cell-sub {
  color: #64748B;
  font-size: 12px;
}

.cell-main {
  color: #0F172A;
  font-weight: 500;
}

.cell-money {
  color: #0F172A;
  font-weight: 600;
  font-size: 13px;
}

.cell-empty {
  color: #CBD5E1;
}

.cell-tag {
  display: inline-block;
  padding: 2px 6px;
  border-radius: 4px;
  background: #F1F5F9;
  color: #475569;
  font-size: 11px;
}

/* 空状态 */
.table-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
  gap: 8px;
}

.empty-icon-wrap {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background: #F1F5F9;
  color: #94A3B8;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 4px;
}

.empty-title {
  font-size: 14px;
  font-weight: 500;
  color: #334155;
}

.empty-desc {
  font-size: 12px;
  color: #94A3B8;
  margin-bottom: 8px;
}

/* 响应式 */
@media (max-width: 1024px) {
  .metrics-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .metrics-grid {
    grid-template-columns: 1fr;
  }
  .filter-bar {
    flex-direction: column;
    align-items: stretch;
  }
  .search-input, .filter-select {
    width: 100%;
  }
  .filter-actions {
    margin-left: 0;
  }
}
</style>
