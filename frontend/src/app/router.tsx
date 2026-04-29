import { Suspense, lazy } from 'react'
import { createBrowserRouter, Navigate, Outlet, useLocation } from 'react-router-dom'
import { Result, Spin } from 'antd'
import { useAuth } from '../auth'
import { useI18n } from '../i18n'
import { navGroups } from '../config/permission-taxonomy'
import type { PermissionCode } from '../lib/api'
import type { PlatformScopeLevel } from '../platform-scope/PlatformScopeProvider'
import { usePlatformScope } from '../platform-scope/usePlatformScope'

const AdminLayout = lazy(() => import('../layouts/AdminLayout'))
const DashboardPage = lazy(() => import('../pages/dashboard'))
const AgentsPage = lazy(() => import('../pages/agents'))
const InviteCodesPage = lazy(() => import('../pages/invite-codes'))
const UsersPage = lazy(() => import('../pages/users'))
const BindingsPage = lazy(() => import('../pages/bindings'))
const BindingHistoryPage = lazy(() => import('../pages/binding-history'))
const AgentInviteApplicationsPage = lazy(() => import('../pages/agent-invite-applications'))
const GamesPage = lazy(() => import('../pages/games'))
const AgentGameAccessPage = lazy(() => import('../pages/agent-game-access'))
const RulesPage = lazy(() => import('../pages/rules'))
const ActivityRewardCenterPage = lazy(() => import('../pages/activity-reward-center'))
const OrdersPage = lazy(() => import('../pages/orders'))
const CommissionsPage = lazy(() => import('../pages/commissions'))
const SettlementBillsPage = lazy(() => import('../pages/settlement-bills'))
const RecalculationTasksPage = lazy(() => import('../pages/recalculation-tasks'))
const LedgerPage = lazy(() => import('../pages/ledger'))
const AuditPage = lazy(() => import('../pages/audit'))
const RiskPage = lazy(() => import('../pages/risk'))
const ReportsPage = lazy(() => import('../pages/reports'))
const RbacPage = lazy(() => import('../pages/rbac'))
const WithdrawalRequestsPage = lazy(() => import('../pages/withdrawal-requests'))
const TenantBrandManagementPage = lazy(() => import('../pages/tenant-brand'))
const PlatformConfigCenterPage = lazy(() => import('../pages/platform-config-center'))
const LoginPage = lazy(() => import('../pages/login'))

type ProtectedRouteProps = {
  permission: PermissionCode | PermissionCode[]
  children: React.ReactNode
  allowedLevels?: PlatformScopeLevel[]
}

function RequireAuth() {
  const location = useLocation()
  const { isAuthenticated, isReady } = useAuth()

  if (!isReady) {
    return <FallbackPage />
  }

  if (!isAuthenticated) {
    return <Navigate replace to="/login" state={{ from: location }} />
  }

  return <Outlet />
}

function LoginRoute() {
  const { isAuthenticated, isReady } = useAuth()
  const location = useLocation()
  const redirectTo = (location.state as { from?: { pathname?: string } } | undefined)?.from?.pathname ?? '/'

  if (!isReady) {
    return <FallbackPage />
  }

  if (isAuthenticated) {
    return <Navigate replace to={redirectTo} />
  }

  return <LoginPage />
}

function FallbackPage() {
  return (
    <div style={{ minHeight: '100vh', display: 'grid', placeItems: 'center' }}>
      <Spin size="large" />
    </div>
  )
}

function LazyPage({ children }: { children: React.ReactNode }) {
  return <Suspense fallback={<FallbackPage />}>{children}</Suspense>
}

function ProtectedRoute({ permission, children, allowedLevels }: ProtectedRouteProps) {
  const { hasPermission } = useAuth()
  const { t } = useI18n()
  const { scopeLevel } = usePlatformScope()

  const allowedByPermission = Array.isArray(permission)
    ? permission.some((item) => hasPermission(item))
    : hasPermission(permission)
  const allowedByLevel = !allowedLevels || allowedLevels.includes(scopeLevel)

  if (!allowedByPermission || !allowedByLevel) {
    return <Result status="403" title="403" subTitle={t('auth.noMenuPermission')} />
  }

  return <>{children}</>
}


const navItemByPath = navGroups
  .flatMap((group) => group.items)
  .reduce<Record<string, (typeof navGroups)[number]['items'][number]>>((acc, item) => {
    acc[item.key.replace(/^\//, '') || '/'] = item
    return acc
  }, {})

function routeGuard(path: string, children: React.ReactNode) {
  const item = navItemByPath[path]
  if (!item) {
    return children
  }
  return <ProtectedRoute permission={item.permission} allowedLevels={item.allowedLevels}>{children}</ProtectedRoute>
}

function NotFoundPage() {
  const { t } = useI18n()
  return <Result status="404" title="404" subTitle={t('common.notFound')} />
}

export const router = createBrowserRouter([
  {
    path: '/login',
    element: (
      <LazyPage>
        <LoginRoute />
      </LazyPage>
    ),
  },
  {
    element: <RequireAuth />,
    children: [
      {
        path: '/',
        element: (
          <LazyPage>
            <AdminLayout />
          </LazyPage>
        ),
        children: [
          { index: true, element: routeGuard('/', <LazyPage><DashboardPage /></LazyPage>) },
          { path: 'agents', element: routeGuard('agents', <LazyPage><AgentsPage /></LazyPage>) },
          { path: 'invite-codes', element: routeGuard('invite-codes', <LazyPage><InviteCodesPage /></LazyPage>) },
          { path: 'users', element: routeGuard('users', <LazyPage><UsersPage /></LazyPage>) },
          { path: 'bindings', element: routeGuard('bindings', <LazyPage><BindingsPage /></LazyPage>) },
          { path: 'binding-history', element: routeGuard('binding-history', <LazyPage><BindingHistoryPage /></LazyPage>) },
          { path: 'agent-invite-applications', element: routeGuard('agent-invite-applications', <LazyPage><AgentInviteApplicationsPage /></LazyPage>) },
          { path: 'tenant-brand', element: <Navigate replace to="/tenant-brand/tenants" /> },
          { path: 'tenant-brand/tenants', element: routeGuard('tenant-brand/tenants', <LazyPage><TenantBrandManagementPage /></LazyPage>) },
          { path: 'tenant-brand/brands', element: routeGuard('tenant-brand/brands', <LazyPage><TenantBrandManagementPage /></LazyPage>) },
          { path: 'platform-config-center', element: routeGuard('platform-config-center', <LazyPage><PlatformConfigCenterPage /></LazyPage>) },
          { path: 'games', element: routeGuard('games', <LazyPage><GamesPage /></LazyPage>) },
          { path: 'agent-game-access', element: routeGuard('agent-game-access', <LazyPage><AgentGameAccessPage /></LazyPage>) },
          { path: 'rules', element: routeGuard('rules', <LazyPage><RulesPage /></LazyPage>) },
          { path: 'activity-reward-center', element: <Navigate replace to="/activity-reward-center/rules" /> },
          { path: 'activity-reward-center/rules', element: routeGuard('activity-reward-center/rules', <LazyPage><ActivityRewardCenterPage /></LazyPage>) },
          { path: 'activity-reward-center/records', element: routeGuard('activity-reward-center/records', <LazyPage><ActivityRewardCenterPage /></LazyPage>) },
          { path: 'orders', element: routeGuard('orders', <LazyPage><OrdersPage /></LazyPage>) },
          { path: 'commissions', element: routeGuard('commissions', <LazyPage><CommissionsPage /></LazyPage>) },
          { path: 'settlement-bills', element: routeGuard('settlement-bills', <LazyPage><SettlementBillsPage /></LazyPage>) },
          { path: 'recalculation-tasks', element: routeGuard('recalculation-tasks', <LazyPage><RecalculationTasksPage /></LazyPage>) },
          { path: 'ledger', element: routeGuard('ledger', <LazyPage><LedgerPage /></LazyPage>) },
          { path: 'withdrawal-requests', element: <Navigate replace to="/withdrawal-requests/list" /> },
          { path: 'withdrawal-requests/list', element: routeGuard('withdrawal-requests/list', <LazyPage><WithdrawalRequestsPage /></LazyPage>) },
          { path: 'withdrawal-requests/process', element: routeGuard('withdrawal-requests/process', <LazyPage><WithdrawalRequestsPage /></LazyPage>) },
          { path: 'audit', element: routeGuard('audit', <LazyPage><AuditPage /></LazyPage>) },
          { path: 'risk', element: <Navigate replace to="/risk/cases" /> },
          { path: 'risk/cases', element: routeGuard('risk/cases', <LazyPage><RiskPage /></LazyPage>) },
          { path: 'risk/intelligence', element: routeGuard('risk/intelligence', <LazyPage><RiskPage /></LazyPage>) },
          { path: 'reports', element: <Navigate replace to="/reports/agent" /> },
          { path: 'reports/agent', element: routeGuard('reports/agent', <LazyPage><ReportsPage /></LazyPage>) },
          { path: 'reports/team', element: routeGuard('reports/team', <LazyPage><ReportsPage /></LazyPage>) },
          { path: 'reports/settlement', element: routeGuard('reports/settlement', <LazyPage><ReportsPage /></LazyPage>) },
          { path: 'reports/progress', element: routeGuard('reports/progress', <LazyPage><ReportsPage /></LazyPage>) },
          { path: 'reports/data-platform', element: routeGuard('reports/data-platform', <LazyPage><ReportsPage /></LazyPage>) },
          { path: 'rbac', element: <Navigate replace to="/rbac/users" /> },
          { path: 'rbac/users', element: routeGuard('rbac/users', <LazyPage><RbacPage /></LazyPage>) },
          { path: 'rbac/roles', element: routeGuard('rbac/roles', <LazyPage><RbacPage /></LazyPage>) },
          { path: 'rbac/permissions', element: routeGuard('rbac/permissions', <LazyPage><RbacPage /></LazyPage>) },
        ],
      },
    ],
  },
  {
    path: '*',
    element: <NotFoundPage />,
  },
])
