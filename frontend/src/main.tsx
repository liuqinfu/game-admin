import React from 'react'
import ReactDOM from 'react-dom/client'
import { App as AntdApp, ConfigProvider, theme as antdTheme } from 'antd'
import enUS from 'antd/locale/en_US'
import zhCN from 'antd/locale/zh_CN'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router-dom'
import { AuthProvider } from './auth'
import { router } from './app/router'
import { I18nProvider, useI18n } from './i18n'
import { PlatformScopeProvider } from './platform-scope/PlatformScopeProvider'
import 'antd/dist/reset.css'
import './styles/ui-polish.css'

const queryClient = new QueryClient()

function AppRoot() {
  const { locale } = useI18n()

  return (
    <ConfigProvider
      locale={locale === 'zh-CN' ? zhCN : enUS}
      theme={{
        algorithm: antdTheme.defaultAlgorithm,
        token: {
          colorPrimary: '#1e40af',
          colorSuccess: '#16a34a',
          colorWarning: '#d97706',
          colorError: '#dc2626',
          colorInfo: '#1e40af',
          colorBgBase: '#f8fafc',
          colorBgLayout: '#eef3f8',
          colorBgContainer: '#ffffff',
          colorBorder: '#dbe3f0',
          colorBorderSecondary: '#e7edf6',
          colorTextBase: '#0f172a',
          colorTextSecondary: '#64748b',
          fontFamily: "'Plus Jakarta Sans', 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
          borderRadius: 14,
          borderRadiusLG: 24,
          borderRadiusSM: 10,
          controlHeight: 42,
          controlHeightLG: 48,
          fontSize: 14,
          wireframe: false,
        },
        components: {
          Layout: {
            headerBg: 'transparent',
            bodyBg: 'transparent',
            siderBg: '#0f172a',
          },
          Card: {
            borderRadiusLG: 24,
            headerHeight: 58,
            paddingLG: 22,
          },
          Table: {
            headerBg: '#f7faff',
            headerColor: '#475569',
            rowHoverBg: '#f5f9ff',
            borderColor: '#e2e8f0',
            cellPaddingBlock: 14,
            cellPaddingInline: 18,
          },
          Button: {
            borderRadius: 14,
            fontWeight: 600,
            controlHeight: 42,
          },
          Input: {
            borderRadius: 14,
            controlHeight: 42,
          },
          Select: {
            borderRadius: 14,
            controlHeight: 42,
          },
          Alert: {
            borderRadiusLG: 18,
          },
          Form: {
            itemMarginBottom: 18,
          },
          Modal: {
            borderRadiusLG: 26,
          },
          Segmented: {
            trackBg: '#edf2fa',
          },
          Breadcrumb: {
            linkColor: '#64748b',
            lastItemColor: '#0f172a',
          },
        },
      }}
    >
      <AntdApp>
        <QueryClientProvider client={queryClient}>
          <PlatformScopeProvider>
            <RouterProvider router={router} />
          </PlatformScopeProvider>
        </QueryClientProvider>
      </AntdApp>
    </ConfigProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <I18nProvider>
      <AuthProvider>
        <AppRoot />
      </AuthProvider>
    </I18nProvider>
  </React.StrictMode>,
)
