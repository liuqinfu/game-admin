import {
  AlertOutlined,
  ApartmentOutlined,
  ApiOutlined,
  AuditOutlined,
  BarChartOutlined,
  CloseOutlined,
  DashboardOutlined,
  GiftOutlined,
  GlobalOutlined,
  LinkOutlined,
  MoreOutlined,
  NotificationOutlined,
  ReadOutlined,
  ReconciliationOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  TeamOutlined,
  UserOutlined,
  WalletOutlined,
} from '@ant-design/icons'
import { Alert, Avatar, Breadcrumb, Button, Dropdown, Grid, Layout, Menu, Result, Segmented, Select, Typography } from 'antd'
import type { MenuProps } from 'antd'
import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth'
import { navGroups, type BusinessGroupKey } from '../config/permission-taxonomy'
import { useI18n } from '../i18n'
import { usePlatformScope } from '../platform-scope/usePlatformScope'
import './AdminLayout.css'

const { Header, Sider, Content } = Layout
const { useBreakpoint } = Grid

type NavItem = {
  key: string
  labelKey: string
  icon: React.ReactNode
}

type OpenTab = {
  key: string
  label: string
}

const navIcons: Record<string, React.ReactNode> = {
  '/': <DashboardOutlined />,
  '/agents': <TeamOutlined />,
  '/invite-codes': <GiftOutlined />,
  '/users': <UserOutlined />,
  '/bindings': <LinkOutlined />,
  '/binding-history': <ApartmentOutlined />,
  '/agent-invite-applications': <ApiOutlined />,
  '/tenant-brand/tenants': <ShopOutlined />,
  '/tenant-brand/brands': <ShopOutlined />,
  '/tenant-brand': <ShopOutlined />,
  '/platform-config-center': <ApiOutlined />,
  '/games': <ShopOutlined />,
  '/agent-game-access': <ShopOutlined />,
  '/rules': <ReadOutlined />,
  '/activity-reward-center/rules': <GiftOutlined />,
  '/activity-reward-center/records': <GiftOutlined />,
  '/activity-reward-center': <GiftOutlined />,
  '/orders': <NotificationOutlined />,
  '/commissions': <ReconciliationOutlined />,
  '/settlement-bills': <ReconciliationOutlined />,
  '/recalculation-tasks': <ReconciliationOutlined />,
  '/ledger': <ReconciliationOutlined />,
  '/withdrawal-requests/list': <WalletOutlined />,
  '/withdrawal-requests/process': <WalletOutlined />,
  '/withdrawal-requests': <WalletOutlined />,
  '/audit': <AuditOutlined />,
  '/risk/cases': <AlertOutlined />,
  '/risk/intelligence': <AlertOutlined />,
  '/risk': <AlertOutlined />,
  '/reports/agent': <BarChartOutlined />,
  '/reports/team': <BarChartOutlined />,
  '/reports/settlement': <BarChartOutlined />,
  '/reports/progress': <BarChartOutlined />,
  '/reports/data-platform': <BarChartOutlined />,
  '/reports': <BarChartOutlined />,
  '/rbac/users': <SafetyCertificateOutlined />,
  '/rbac/roles': <SafetyCertificateOutlined />,
  '/rbac/permissions': <SafetyCertificateOutlined />,
  '/rbac': <SafetyCertificateOutlined />,
}

const groupIcons: Record<BusinessGroupKey, React.ReactNode> = {
  overview: <DashboardOutlined />,
  agent: <TeamOutlined />,
  tenant: <GlobalOutlined />,
  game: <ShopOutlined />,
  growth: <GiftOutlined />,
  finance: <ReconciliationOutlined />,
  security: <SafetyCertificateOutlined />,
  system: <ApiOutlined />,
}

export default function AdminLayout() {
  const { locale, setLocale, t } = useI18n()
  const { hasAnyPermission, signOut, user } = useAuth()
  const {
    activeTenantId,
    activeBrandId,
    brands,
    tenants,
    isTenantsLoading,
    isBrandsLoading,
    tenantsError,
    brandsError,
    scopeLevel,
    isScopeLocked,
    setActiveTenantId,
    setActiveBrandId,
    clearScope,
  } = usePlatformScope()
  const location = useLocation()
  const navigate = useNavigate()
  const screens = useBreakpoint()
  const isMobile = !screens.xl
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)
  const navWrapRef = useRef<HTMLDivElement | null>(null)

  const visibleNavGroups = useMemo(
    () =>
      navGroups
        .map((group) => ({
          ...group,
          items: group.items
            .filter((item) => {
              const allowedByLevel = !item.allowedLevels || item.allowedLevels.includes(scopeLevel)
              const requiredPermissions = Array.isArray(item.permission) ? item.permission : [item.permission]
              return allowedByLevel && hasAnyPermission(requiredPermissions)
            })
            .map((item) => ({ ...item, icon: navIcons[item.key] })),
        }))
        .filter((group) => group.items.length > 0),
    [hasAnyPermission, scopeLevel],
  )

  const visibleNavItems = useMemo<NavItem[]>(() => visibleNavGroups.flatMap((group) => group.items), [visibleNavGroups])

  const selectedKey = useMemo(() => {
    const matched = [...visibleNavItems]
      .sort((a, b) => b.key.length - a.key.length)
      .find((item) => item.key !== '/' && location.pathname.startsWith(item.key))

    return matched?.key ?? (visibleNavItems[0]?.key ?? '/')
  }, [location.pathname, visibleNavItems])

  const currentNavItem = visibleNavItems.find((item) => item.key === selectedKey)
  const pageTitle = currentNavItem ? t(currentNavItem.labelKey) : t('layout.nav.dashboard')
  const activeTabKey = `${location.pathname}${location.search}`
  const activeGroupKey = useMemo(() => {
    const matchedGroup = visibleNavGroups.find((group) => group.items.some((item) => item.key === selectedKey))
    return matchedGroup ? `group:${matchedGroup.key}` : undefined
  }, [selectedKey, visibleNavGroups])
  const [openGroupKey, setOpenGroupKey] = useState<string | undefined>(activeGroupKey)

  useEffect(() => {
    setOpenGroupKey(activeGroupKey)
  }, [activeGroupKey])

  useEffect(() => {
    if (!navWrapRef.current || !selectedKey) {
      return
    }
    const frame = window.requestAnimationFrame(() => {
      const activeItem = navWrapRef.current?.querySelector('.ant-menu-item-selected')
      if (activeItem instanceof HTMLElement) {
        activeItem.scrollIntoView({ block: 'nearest' })
      }
    })
    return () => window.cancelAnimationFrame(frame)
  }, [openGroupKey, selectedKey])

  const breadcrumbs = useMemo(
    () => [
      { title: <Link to={visibleNavItems[0]?.key ?? '/'}>{t('common.admin')}</Link> },
      ...(currentNavItem && currentNavItem.key !== '/' ? [{ title: pageTitle }] : []),
    ],
    [currentNavItem, pageTitle, t, visibleNavItems],
  )

  const menuItems = useMemo<MenuProps['items']>(
    () =>
      visibleNavGroups.map((group) => ({
        key: `group:${group.key}`,
        icon: groupIcons[group.key],
        label: t(group.labelKey),
        children: group.items.map((item) => ({
          key: item.key,
          icon: item.icon,
          label: (
            <Link to={item.key} onClick={() => isMobile && setMobileMenuOpen(false)}>
              {t(item.labelKey)}
            </Link>
          ),
        })),
      })),
    [isMobile, t, visibleNavGroups],
  )

  const activeTenant = tenants.find((tenant) => tenant.id === activeTenantId)
  const activeBrand = brands.find((brand) => brand.id === activeBrandId)
  const accountName = user?.displayName || user?.username || t('common.admin')
  const activeScopeLabel = activeBrand?.name || activeTenant?.name || t('platformScope.scope.platform')
  const [openTabs, setOpenTabs] = useState<OpenTab[]>([])
  const accountMenuItems = useMemo<MenuProps['items']>(
    () => [
      {
        key: 'logout',
        label: t('common.logout'),
      },
    ],
    [t],
  )

  useEffect(() => {
    if (!activeTabKey) {
      return
    }
    setOpenTabs((current) => {
      if (current.some((tab) => tab.key === activeTabKey)) {
        return current.map((tab) => (tab.key === activeTabKey ? { ...tab, label: pageTitle } : tab))
      }
      return [...current, { key: activeTabKey, label: pageTitle }]
    })
  }, [activeTabKey, pageTitle])

  const closeTab = (targetKey: string) => {
    setOpenTabs((current) => {
      const nextTabs = current.filter((tab) => tab.key !== targetKey)
      if (targetKey === activeTabKey) {
        const fallbackTab = nextTabs[nextTabs.length - 1] ?? current.find((tab) => tab.key !== targetKey)
        navigate(fallbackTab?.key ?? '/', { replace: true })
      }
      return nextTabs
    })
  }

  const tabActionItems = useMemo<MenuProps['items']>(
    () => [
      {
        key: 'closeOthers',
        label: t('layout.tabs.closeOthers'),
        disabled: openTabs.length <= 1,
      },
      {
        key: 'closeAll',
        label: t('layout.tabs.closeAll'),
        disabled: openTabs.length === 0,
      },
    ],
    [openTabs.length, t],
  )

  if (visibleNavItems.length === 0) {
    return (
      <Layout className="admin-shell">
        <Layout className="admin-shell__main">
          <Content className="admin-shell__content">
            <div className="admin-shell__content-scroll">
              <Result
                status="403"
                title="403"
                subTitle={t('auth.noMenuPermission')}
                extra={<Button onClick={() => navigate('/login', { replace: true })}>{t('common.logout')}</Button>}
              />
            </div>
          </Content>
        </Layout>
      </Layout>
    )
  }

  return (
    <Layout className="admin-shell">
      <Sider
        className="admin-shell__sider"
        width={304}
        breakpoint="xl"
        collapsedWidth="0"
        collapsible
        trigger={null}
        collapsed={isMobile ? !mobileMenuOpen : false}
        onBreakpoint={(broken) => {
          if (!broken) {
            setMobileMenuOpen(false)
          }
        }}
      >
        <div className="admin-shell__brand">
          <div className="admin-shell__brand-mark">GA</div>
          <div className="admin-shell__brand-copy">
            <Typography.Text className="admin-shell__brand-eyebrow">{t('common.admin')}</Typography.Text>
            <Typography.Text className="admin-shell__brand-name">{t('common.appName')}</Typography.Text>
            <Typography.Text className="admin-shell__brand-subtitle">{t('layout.consoleTitle')}</Typography.Text>
          </div>
        </div>

        <div className="admin-shell__brand-card">
          <Typography.Text className="admin-shell__brand-card-label">{t('platformScope.controls.label')}</Typography.Text>
          <Typography.Text className="admin-shell__brand-card-value">{activeScopeLabel}</Typography.Text>
          <Typography.Text className="admin-shell__brand-card-copy">{t('layout.consoleSubtitle')}</Typography.Text>
        </div>

        <div ref={navWrapRef} className="admin-shell__nav-wrap">
          <Menu
            mode="inline"
            selectedKeys={[selectedKey]}
            openKeys={openGroupKey ? [openGroupKey] : []}
            items={menuItems}
            className="admin-shell__menu"
            onOpenChange={(keys) => {
              const latestKey = keys.at(-1)
              setOpenGroupKey(latestKey)
            }}
          />
        </div>

        <div className="admin-shell__sider-footer">
          <Typography.Text className="admin-shell__sider-footer-label">{t('common.admin')}</Typography.Text>
          <Typography.Text className="admin-shell__sider-footer-value">{accountName}</Typography.Text>
        </div>
      </Sider>

      <Layout className="admin-shell__main">
        <Header className="admin-shell__header">
          <div className="admin-shell__topbar">
            <div className="admin-shell__topbar-path">
              {isMobile ? (
                <Button className="admin-shell__menu-toggle" onClick={() => setMobileMenuOpen((open) => !open)}>
                  {mobileMenuOpen ? t('layout.mobile.collapse') : t('layout.mobile.expand')}
                </Button>
              ) : null}
              <Breadcrumb items={breadcrumbs} className="admin-shell__breadcrumb" />
            </div>

            <div className="admin-shell__topbar-actions">
              <div className="admin-shell__scope-panel">
                <Typography.Text className="admin-shell__scope-label">{t('platformScope.controls.label')}</Typography.Text>
                <Select<number | undefined>
                  allowClear
                  showSearch
                  className="admin-shell__scope-select"
                  placeholder={t('platformScope.controls.tenantPlaceholder')}
                  value={activeTenantId}
                  onChange={(value) => setActiveTenantId(value)}
                  loading={isTenantsLoading}
                  disabled={isScopeLocked}
                  options={tenants.map((tenant) => ({ label: `${tenant.name} (${tenant.code})`, value: tenant.id }))}
                  optionFilterProp="label"
                />
                <Select<number | undefined>
                  allowClear
                  showSearch
                  className="admin-shell__scope-select"
                  placeholder={t('platformScope.controls.brandPlaceholder')}
                  value={activeBrandId}
                  onChange={(value) => setActiveBrandId(value)}
                  loading={isBrandsLoading}
                  disabled={isScopeLocked || !activeTenantId}
                  options={brands.map((brand) => ({ label: `${brand.name} (${brand.code})`, value: brand.id }))}
                  optionFilterProp="label"
                />
                <Button onClick={clearScope} disabled={isScopeLocked}>
                  {t('platformScope.controls.clear')}
                </Button>
              </div>

              <div className="admin-shell__utility-bar">
                <div className="admin-shell__locale-switcher">
                  <GlobalOutlined />
                  <Segmented
                    size="small"
                    value={locale}
                    onChange={(value) => setLocale(value as 'zh-CN' | 'en-US')}
                    options={[
                      { label: t('common.zhCN'), value: 'zh-CN' },
                      { label: t('common.enUS'), value: 'en-US' },
                    ]}
                  />
                </div>
                <Dropdown
                  menu={{
                    items: accountMenuItems,
                    onClick: ({ key }) => {
                      if (key === 'logout') {
                        signOut()
                        navigate('/login', { replace: true })
                      }
                    },
                  }}
                  trigger={['click']}
                >
                  <button type="button" className="admin-shell__account admin-shell__account--interactive" aria-label={t('layout.accountMenu')}>
                    <Avatar className="admin-shell__avatar" size={40}>
                      {accountName.slice(0, 1).toUpperCase()}
                    </Avatar>
                    <div className="admin-shell__account-copy">
                      <Typography.Text className="admin-shell__account-name">{accountName}</Typography.Text>
                      <Typography.Text className="admin-shell__account-role">{t('common.admin')}</Typography.Text>
                    </div>
                  </button>
                </Dropdown>
              </div>
            </div>
          </div>

          {tenantsError || brandsError ? (
            <Alert
              type="warning"
              showIcon
              className="admin-shell__scope-alert"
              message={t('platformScope.controls.loadError')}
              description={(tenantsError ?? brandsError)?.message}
            />
          ) : null}
        </Header>

        <Content className="admin-shell__content">
          <div className="admin-shell__content-scroll">
            <div className="admin-shell__content-stack">
              <div className="admin-shell__tabs-shell">
                <div className="admin-shell__tabs-scroll">
                  {openTabs.map((tab) => {
                    const active = tab.key === activeTabKey
                    const closable = openTabs.length > 1
                    return (
                      <div key={tab.key} className={`admin-shell__tab${active ? ' admin-shell__tab--active' : ''}`}>
                        <button type="button" className="admin-shell__tab-trigger" onClick={() => navigate(tab.key)}>
                          <span className="admin-shell__tab-label">{tab.label}</span>
                        </button>
                        {closable ? (
                          <button
                            type="button"
                            className="admin-shell__tab-close"
                            onClick={(event) => {
                              event.stopPropagation()
                              closeTab(tab.key)
                            }}
                            aria-label={t('layout.tabs.closeTab')}
                          >
                            <CloseOutlined />
                          </button>
                        ) : null}
                      </div>
                    )
                  })}
                </div>
                <Dropdown
                  trigger={['click']}
                  menu={{
                    items: tabActionItems,
                    onClick: ({ key }) => {
                      if (key === 'closeOthers') {
                        setOpenTabs(activeTabKey ? openTabs.filter((tab) => tab.key === activeTabKey) : openTabs)
                      }
                      if (key === 'closeAll') {
                        const fallback = activeTabKey || '/'
                        setOpenTabs([{ key: fallback, label: pageTitle }])
                        navigate(fallback, { replace: true })
                      }
                    },
                  }}
                >
                  <button type="button" className="admin-shell__tabs-actions" aria-label={t('layout.tabs.manageTabs')}>
                    <MoreOutlined />
                  </button>
                </Dropdown>
              </div>
              <div className="admin-shell__content-panel">
                <Outlet />
              </div>
            </div>
          </div>
        </Content>
      </Layout>
    </Layout>
  )
}
