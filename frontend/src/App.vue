<template>
  <n-config-provider :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <n-dialog-provider>
      <n-message-provider>
        <router-view />
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>

<script setup>
import { zhCN, dateZhCN, NConfigProvider, NDialogProvider, NMessageProvider } from 'naive-ui'

const BRAND = '#2563EB'
const BRAND_HOVER = '#1D4ED8'
const BRAND_PRESSED = '#1E40AF'
const BORDER = '#E2E8F0'
const INK_900 = '#0F172A'
const INK_600 = '#475569'

const themeOverrides = {
  common: {
    primaryColor: BRAND,
    primaryColorHover: BRAND_HOVER,
    primaryColorPressed: BRAND_PRESSED,
    primaryColorSuppl: BRAND_HOVER,
    borderColor: BORDER,
    borderRadius: '8px',
    textColorBase: INK_900,
    textColor1: INK_900,
    textColor2: INK_600,
    fontFamily: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'PingFang SC', 'Microsoft YaHei', sans-serif",
    fontSize: '13px',
  },
  Input: {
    border: `1px solid ${BORDER}`,
    borderHover: '1px solid #94A3B8',
    borderFocus: `1px solid ${BRAND}`,
    boxShadowFocus: '0 0 0 3px rgba(37, 99, 235, 0.12)',
    heightMedium: '36px',
    heightLarge: '42px',
    borderRadius: '8px',
    color: '#FFFFFF',
    textColor: INK_900,
    placeholderColor: '#94A3B8',
  },
  Button: {
    heightMedium: '36px',
    heightLarge: '42px',
    fontWeight: '500',
    borderRadiusMedium: '8px',
    borderRadiusLarge: '8px',
    boxShadowHoverPrimary: '0 2px 6px 0 rgba(37, 99, 235, 0.25)',
  },
  Select: {
    peers: {
      InternalSelection: {
        border: `1px solid ${BORDER}`,
        borderHover: '1px solid #94A3B8',
        borderFocus: `1px solid ${BRAND}`,
        boxShadowFocus: '0 0 0 3px rgba(37, 99, 235, 0.12)',
        heightMedium: '36px',
        borderRadius: '8px',
        textColor: INK_900,
        placeholderColor: '#94A3B8',
      },
    },
  },
  DataTable: {
    borderColor: '#F1F5F9',
    thColor: '#F8FAFC',
    thTextColor: '#64748B',
    thFontWeight: '600',
    tdColorHover: '#F8FAFC',
    thColorHover: '#F8FAFC',
    fontSizeMedium: '13px',
  },
  Form: {
    labelTextColor: '#334155',
    labelFontSize: '13px',
    labelFontWeight: '500',
  },
  Pagination: {
    itemTextColor: '#475569',
    itemColorActive: '#EFF6FF',
    itemBorderRadius: '6px',
  },
  Dialog: {
    borderRadius: '12px',
  },
}
</script>
