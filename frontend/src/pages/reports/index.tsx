import { DatabaseOutlined, ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Empty, Input, Space, Statistic, Table, Tag, Typography } from 'antd'
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useLocation, useSearchParams } from 'react-router-dom'
import { AgentSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { PlatformScopeSummary } from '../../platform-scope/PlatformScopeSummary'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'
import {
  AgentPerformanceReportItem,
  DataPlatformLayerItem,
  DataPlatformMetricItem,
  GameSettlementReportItem,
  SettlementProgressReportItem,
  TeamPerformanceReportItem,
  apiClient,
} from '../../lib/api'

export default function ReportsPage() {
  const { t } = useI18n()
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const reportSection = useMemo<'agent' | 'team' | 'settlement' | 'progress' | 'data-platform'>(() => {
    const section = location.pathname.split('/')[2]
    if (section === 'team' || section === 'settlement' || section === 'progress' || section === 'data-platform') {
      return section
    }
    return 'agent'
  }, [location.pathname])
  const agentID = Number(searchParams.get('agentID') || '') || undefined
  const currency = searchParams.get('currency') || ''
  const progressCurrency = searchParams.get('progressCurrency') || ''
  const teamAgentID = Number(searchParams.get('teamAgentID') || '') || undefined
  const teamCurrency = searchParams.get('teamCurrency') || ''
  const setParam = (key: string, value?: string | number) => {
    const next = new URLSearchParams(searchParams)
    if (value === undefined || value === '') {
      next.delete(key)
    } else {
      next.set(key, String(value))
    }
    setSearchParams(next, { replace: true })
  }

  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const agentReportQuery = useQuery({
    queryKey: ['report-agent-performance', agentID, activeTenantId, activeBrandId],
    queryFn: () => apiClient.listAgentPerformanceReport({
      agentID,
      ...scopeParams,
    }),
    enabled: reportSection === 'agent',
  })

  const settlementReportQuery = useQuery({
    queryKey: ['report-game-settlement', currency, activeTenantId, activeBrandId],
    queryFn: () => apiClient.listGameSettlementReport({ currency: currency || undefined, ...scopeParams }),
    enabled: reportSection === 'settlement',
  })

  const settlementProgressQuery = useQuery({
    queryKey: ['report-settlement-progress', progressCurrency, activeTenantId, activeBrandId],
    queryFn: () => apiClient.listSettlementProgressReport({ currency: progressCurrency || undefined, ...scopeParams }),
    enabled: reportSection === 'progress',
  })

  const teamReportQuery = useQuery({
    queryKey: ['report-team-performance', teamAgentID, teamCurrency, activeTenantId, activeBrandId],
    queryFn: () => apiClient.listTeamPerformanceReport({
      agentID: teamAgentID,
      currency: teamCurrency || undefined,
      ...scopeParams,
    }),
    enabled: reportSection === 'team',
  })

  const dataPlatformLayersQuery = useQuery({
    queryKey: ['report-data-platform-layers', activeTenantId, activeBrandId],
    queryFn: () => apiClient.listDataPlatformLayers(scopeParams),
    enabled: reportSection === 'data-platform',
  })

  const dataPlatformMetricsQuery = useQuery({
    queryKey: ['report-data-platform-metrics', activeTenantId, activeBrandId],
    queryFn: () => apiClient.listDataPlatformMetrics(scopeParams),
    enabled: reportSection === 'data-platform',
  })

  const agentSummary = useMemo(() => {
    const items = agentReportQuery.data?.items ?? []
    return {
      totalAgents: items.length,
      totalBalance: items.reduce((sum, item) => sum + Number(item.balance || 0), 0),
      totalFrozen: items.reduce((sum, item) => sum + Number(item.frozenBalance || 0), 0),
    }
  }, [agentReportQuery.data?.items])

  const settlementSummary = useMemo(() => {
    const items = settlementReportQuery.data?.items ?? []
    return {
      totalCurrencies: items.length,
      totalPayable: items.reduce((sum, item) => sum + Number(item.totalPayableAmount || 0), 0),
      totalBills: items.reduce((sum, item) => sum + Number(item.totalBills || 0), 0),
    }
  }, [settlementReportQuery.data?.items])

  const teamSummary = useMemo(() => {
    const items = teamReportQuery.data?.items ?? []
    return {
      totalTeams: items.length,
      totalDescendants: items.reduce((sum, item) => sum + Number(item.totalDescendants || 0), 0),
      totalTeamBalance: items.reduce((sum, item) => sum + Number(item.totalTeamBalance || 0), 0),
      totalPendingWithdrawalAmount: items.reduce((sum, item) => sum + Number(item.pendingWithdrawalAmount || 0), 0),
    }
  }, [teamReportQuery.data?.items])

  const progressSummary = useMemo(() => {
    const items = settlementProgressQuery.data?.items ?? []
    return {
      totalCurrencies: items.length,
      totalBills: items.reduce((sum, item) => sum + Number(item.totalBills || 0), 0),
      completedPayableAmount: items.reduce((sum, item) => sum + Number(item.completedPayableAmount || 0), 0),
      pendingPayableAmount: items.reduce((sum, item) => sum + Number(item.pendingPayableAmount || 0), 0),
    }
  }, [settlementProgressQuery.data?.items])

  const dataPlatformSummary = useMemo(() => {
    const layers = dataPlatformLayersQuery.data?.items ?? []
    const metrics = dataPlatformMetricsQuery.data?.items ?? []
    return {
      readyLayers: layers.filter((item) => item.status === 'ready').length,
      totalLayers: layers.length,
      totalRecords: layers.reduce((sum, item) => sum + Number(item.recordCount || 0), 0),
      metricCount: metrics.length,
    }
  }, [dataPlatformLayersQuery.data?.items, dataPlatformMetricsQuery.data?.items])

  const agentColumns: ColumnsType<AgentPerformanceReportItem> = [
    { title: t('reports.agent.columns.agent'), key: 'agent', render: (_, record) => record.agentName || `#${record.agentID}` },
    { title: t('reports.agent.columns.currency'), dataIndex: 'currency', width: 100 },
    { title: t('reports.agent.columns.balance'), dataIndex: 'balance', width: 120 },
    { title: t('reports.agent.columns.frozenBalance'), dataIndex: 'frozenBalance', width: 140 },
    { title: t('reports.agent.columns.withdrawableAmount'), dataIndex: 'withdrawableAmount', width: 160 },
    { title: t('reports.agent.columns.totalEntries'), dataIndex: 'totalEntries', width: 120 },
    { title: t('reports.agent.columns.incomeAmount'), dataIndex: 'incomeAmount', width: 140 },
    { title: t('reports.agent.columns.freezeAmount'), dataIndex: 'freezeAmount', width: 140 },
    { title: t('reports.agent.columns.unfreezeAmount'), dataIndex: 'unfreezeAmount', width: 150 },
    { title: t('reports.agent.columns.debitAmount'), dataIndex: 'debitAmount', width: 140 },
    { title: t('reports.agent.columns.reverseAmount'), dataIndex: 'reverseAmount', width: 140 },
    { title: t('reports.agent.columns.adjustAmount'), dataIndex: 'adjustAmount', width: 140 },
  ]

  const settlementColumns: ColumnsType<GameSettlementReportItem> = [
    { title: t('reports.settlement.columns.currency'), dataIndex: 'currency', width: 100 },
    { title: t('reports.settlement.columns.totalBills'), dataIndex: 'totalBills', width: 120 },
    { title: t('reports.settlement.columns.confirmedBills'), dataIndex: 'confirmedBills', width: 140 },
    { title: t('reports.settlement.columns.pendingBills'), dataIndex: 'pendingBills', width: 130 },
    { title: t('reports.settlement.columns.generatedBills'), dataIndex: 'generatedBills', width: 140 },
    { title: t('reports.settlement.columns.cancelledBills'), dataIndex: 'cancelledBills', width: 140 },
    { title: t('reports.settlement.columns.totalCommissionAmount'), dataIndex: 'totalCommissionAmount', width: 180 },
    { title: t('reports.settlement.columns.totalAdjustmentAmount'), dataIndex: 'totalAdjustmentAmount', width: 180 },
    { title: t('reports.settlement.columns.totalPayableAmount'), dataIndex: 'totalPayableAmount', width: 170 },
  ]

  const teamColumns: ColumnsType<TeamPerformanceReportItem> = [
    { title: t('reports.team.columns.agent'), key: 'agent', width: 180, render: (_, record) => record.agentName || `#${record.agentID}` },
    { title: t('reports.team.columns.level'), dataIndex: 'level', width: 90 },
    { title: t('reports.team.columns.currency'), dataIndex: 'currency', width: 100 },
    { title: t('reports.team.columns.directDescendants'), dataIndex: 'directDescendants', width: 140 },
    { title: t('reports.team.columns.totalDescendants'), dataIndex: 'totalDescendants', width: 140 },
    { title: t('reports.team.columns.activeDescendants'), dataIndex: 'activeDescendants', width: 150 },
    { title: t('reports.team.columns.leafDescendants'), dataIndex: 'leafDescendants', width: 140 },
    { title: t('reports.team.columns.boundPlayers'), dataIndex: 'boundPlayers', width: 120 },
    { title: t('reports.team.columns.totalTeamBalance'), dataIndex: 'totalTeamBalance', width: 150 },
    { title: t('reports.team.columns.totalTeamFrozenBalance'), dataIndex: 'totalTeamFrozenBalance', width: 170 },
    { title: t('reports.team.columns.totalWithdrawableAmount'), dataIndex: 'totalWithdrawableAmount', width: 170 },
    { title: t('reports.team.columns.pendingWithdrawals'), dataIndex: 'pendingWithdrawals', width: 140 },
    { title: t('reports.team.columns.pendingWithdrawalAmount'), dataIndex: 'pendingWithdrawalAmount', width: 180 },
    { title: t('reports.team.columns.confirmedBills'), dataIndex: 'confirmedBills', width: 130 },
    { title: t('reports.team.columns.confirmedBillAmount'), dataIndex: 'confirmedBillAmount', width: 160 },
  ]

  const progressColumns: ColumnsType<SettlementProgressReportItem> = [
    { title: t('reports.progress.columns.currency'), dataIndex: 'currency', width: 100 },
    { title: t('reports.progress.columns.totalBills'), dataIndex: 'totalBills', width: 120 },
    { title: t('reports.progress.columns.generatedBills'), dataIndex: 'generatedBills', width: 130 },
    { title: t('reports.progress.columns.confirmedBills'), dataIndex: 'confirmedBills', width: 130 },
    { title: t('reports.progress.columns.cancelledBills'), dataIndex: 'cancelledBills', width: 130 },
    { title: t('reports.progress.columns.progressPercent'), dataIndex: 'progressPercent', width: 140, render: (value) => `${Number(value ?? 0).toFixed(2)}%` },
    { title: t('reports.progress.columns.pendingCommissionAmount'), dataIndex: 'pendingCommissionAmount', width: 180 },
    { title: t('reports.progress.columns.pendingPayableAmount'), dataIndex: 'pendingPayableAmount', width: 170 },
    { title: t('reports.progress.columns.completedPayableAmount'), dataIndex: 'completedPayableAmount', width: 180 },
    { title: t('reports.progress.columns.lastGeneratedAt'), dataIndex: 'lastGeneratedAt', width: 180, render: (value) => value || '-' },
    { title: t('reports.progress.columns.lastConfirmedAt'), dataIndex: 'lastConfirmedAt', width: 180, render: (value) => value || '-' },
  ]

  const dataPlatformLayerColumns: ColumnsType<DataPlatformLayerItem> = [
    {
      title: t('reports.dataPlatform.columns.layer'),
      dataIndex: 'layer',
      width: 100,
      render: (value: string) => <Tag color="geekblue">{t(`reports.dataPlatform.layer.${value}`)}</Tag>,
    },
    {
      title: t('reports.dataPlatform.columns.status'),
      dataIndex: 'status',
      width: 120,
      render: (value: string) => <Tag color={value === 'ready' ? 'success' : 'warning'}>{t(`reports.dataPlatform.status.${value}`)}</Tag>,
    },
    { title: t('reports.dataPlatform.columns.syncMode'), dataIndex: 'syncMode', width: 140, render: (value: string) => t(`reports.dataPlatform.syncMode.${value}`) },
    { title: t('reports.dataPlatform.columns.tableCount'), dataIndex: 'tableCount', width: 120 },
    { title: t('reports.dataPlatform.columns.recordCount'), dataIndex: 'recordCount', width: 140 },
    { title: t('reports.dataPlatform.columns.lastSyncedAt'), dataIndex: 'lastSyncedAt', width: 180, render: (value) => value || '-' },
    {
      title: t('reports.dataPlatform.columns.description'),
      dataIndex: 'description',
      ellipsis: true,
      render: (_value, record) => t(`reports.dataPlatform.description.${record.layer}`),
    },
  ]

  const dataPlatformMetricColumns: ColumnsType<DataPlatformMetricItem> = [
    {
      title: t('reports.dataPlatform.metricColumns.metric'),
      dataIndex: 'key',
      width: 220,
      render: (value: string) => t(`reports.dataPlatform.metric.${value}`),
    },
    {
      title: t('reports.dataPlatform.metricColumns.value'),
      key: 'value',
      width: 160,
      render: (_, record) => {
        const suffix = record.unit === 'percent' ? '%' : record.unit === 'count' ? '' : ''
        return `${Number(record.value ?? 0).toFixed(record.unit === 'count' ? 0 : 2)}${suffix}`
      },
    },
    {
      title: t('reports.dataPlatform.metricColumns.description'),
      dataIndex: 'description',
      ellipsis: true,
      render: (_value, record) => t(`reports.dataPlatform.metricDescription.${record.key}`),
    },
  ]

  const sectionTitle = {
    agent: t('reports.agent.title'),
    team: t('reports.team.title'),
    settlement: t('reports.settlement.title'),
    progress: t('reports.progress.title'),
    'data-platform': t('reports.dataPlatform.title'),
  }[reportSection]

  const sectionSubtitle = {
    agent: t('reports.section.agentSubtitle'),
    team: t('reports.section.teamSubtitle'),
    settlement: t('reports.section.settlementSubtitle'),
    progress: t('reports.section.progressSubtitle'),
    'data-platform': t('reports.section.dataPlatformSubtitle'),
  }[reportSection]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">{sectionTitle}</Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">{sectionSubtitle}</Typography.Paragraph>
          </div>
          <PlatformScopeSummary />
        </Space>
      </Card>
      {reportSection === 'data-platform' ? (
        <Card
          title={t('reports.dataPlatform.title')}
          extra={
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                dataPlatformLayersQuery.refetch()
                dataPlatformMetricsQuery.refetch()
              }}
              loading={dataPlatformLayersQuery.isFetching || dataPlatformMetricsQuery.isFetching}
            >
              {t('common.refresh')}
            </Button>
          }
        >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Typography.Paragraph type="secondary">{t('reports.dataPlatform.subtitle')}</Typography.Paragraph>
          {(dataPlatformLayersQuery.error || dataPlatformMetricsQuery.error) ? (
            <Alert
              type="error"
              showIcon
              message={t('reports.dataPlatform.loadError')}
              description={(dataPlatformLayersQuery.error as Error | null)?.message ?? (dataPlatformMetricsQuery.error as Error | null)?.message}
            />
          ) : null}
          <Space size="large" wrap>
            <Card className="app-stat-card"><Statistic title={t('reports.dataPlatform.summary.readyLayers')} value={dataPlatformSummary.readyLayers} suffix={`/ ${dataPlatformSummary.totalLayers}`} prefix={<DatabaseOutlined />} /></Card>
            <Card className="app-stat-card"><Statistic title={t('reports.dataPlatform.summary.totalRecords')} value={dataPlatformSummary.totalRecords} /></Card>
            <Card className="app-stat-card"><Statistic title={t('reports.dataPlatform.summary.metricCount')} value={dataPlatformSummary.metricCount} /></Card>
          </Space>
          <Table<DataPlatformLayerItem>
            rowKey="layer"
            loading={dataPlatformLayersQuery.isLoading || dataPlatformLayersQuery.isFetching}
            dataSource={dataPlatformLayersQuery.data?.items ?? []}
            columns={dataPlatformLayerColumns}
            pagination={false}
            scroll={{ x: 1200 }}
            locale={{ emptyText: <Empty description={t('reports.dataPlatform.emptyLayers')} /> }}
          />
          <Table<DataPlatformMetricItem>
            rowKey="key"
            loading={dataPlatformMetricsQuery.isLoading || dataPlatformMetricsQuery.isFetching}
            dataSource={dataPlatformMetricsQuery.data?.items ?? []}
            columns={dataPlatformMetricColumns}
            pagination={false}
            scroll={{ x: 900 }}
            locale={{ emptyText: <Empty description={t('reports.dataPlatform.emptyMetrics')} /> }}
          />
        </Space>
        </Card>
      ) : null}

      {reportSection === 'agent' ? (
        <Card title={t('reports.agent.title')} extra={<Button icon={<ReloadOutlined />} onClick={() => agentReportQuery.refetch()} loading={agentReportQuery.isFetching}>{t('common.refresh')}</Button>}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap>
            <AgentSelect
              tenantID={scopeParams.tenantID}
              brandID={scopeParams.brandID}
              value={agentID}
              onChange={(value) => setParam('agentID', value)}
              placeholder={t('reports.agent.agentIdPlaceholder')}
              style={{ width: 220 }}
            />
          </Space>
          {agentReportQuery.error ? <Alert type="error" showIcon message={t('reports.agent.loadError')} description={(agentReportQuery.error as Error).message} /> : null}
          <Space size="large" wrap>
            <Typography.Text>{t('reports.agent.summary.totalAgents', { count: agentSummary.totalAgents })}</Typography.Text>
            <Typography.Text>{t('reports.agent.summary.totalBalance', { amount: agentSummary.totalBalance.toFixed(2) })}</Typography.Text>
            <Typography.Text>{t('reports.agent.summary.totalFrozen', { amount: agentSummary.totalFrozen.toFixed(2) })}</Typography.Text>
          </Space>
          <Table<AgentPerformanceReportItem>
            rowKey={(record) => `${record.agentID}-${record.currency}`}
            loading={agentReportQuery.isLoading || agentReportQuery.isFetching}
            dataSource={agentReportQuery.data?.items ?? []}
            columns={agentColumns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            scroll={{ x: 1600 }}
            locale={{ emptyText: <Empty description={t('reports.agent.empty')} /> }}
          />
        </Space>
        </Card>
      ) : null}

      {reportSection === 'team' ? (
        <Card title={t('reports.team.title')} extra={<Button icon={<ReloadOutlined />} onClick={() => teamReportQuery.refetch()} loading={teamReportQuery.isFetching}>{t('common.refresh')}</Button>}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap>
            <AgentSelect
              tenantID={scopeParams.tenantID}
              brandID={scopeParams.brandID}
              value={teamAgentID}
              onChange={(value) => setParam('teamAgentID', value)}
              placeholder={t('reports.team.agentIdPlaceholder')}
              style={{ width: 220 }}
            />
            <Input value={teamCurrency} onChange={(event) => setParam('teamCurrency', event.target.value)} placeholder={t('reports.team.currencyPlaceholder')} style={{ width: 220 }} />
          </Space>
          {teamReportQuery.error ? <Alert type="error" showIcon message={t('reports.team.loadError')} description={(teamReportQuery.error as Error).message} /> : null}
          <Space size="large" wrap>
            <Typography.Text>{t('reports.team.summary.totalTeams', { count: teamSummary.totalTeams })}</Typography.Text>
            <Typography.Text>{t('reports.team.summary.totalDescendants', { count: teamSummary.totalDescendants })}</Typography.Text>
            <Typography.Text>{t('reports.team.summary.totalTeamBalance', { amount: teamSummary.totalTeamBalance.toFixed(2) })}</Typography.Text>
            <Typography.Text>{t('reports.team.summary.totalPendingWithdrawalAmount', { amount: teamSummary.totalPendingWithdrawalAmount.toFixed(2) })}</Typography.Text>
          </Space>
          <Table<TeamPerformanceReportItem>
            rowKey={(record) => `${record.agentID}-${record.currency}`}
            loading={teamReportQuery.isLoading || teamReportQuery.isFetching}
            dataSource={teamReportQuery.data?.items ?? []}
            columns={teamColumns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            scroll={{ x: 2300 }}
            locale={{ emptyText: <Empty description={t('reports.team.empty')} /> }}
          />
        </Space>
        </Card>
      ) : null}

      {reportSection === 'progress' ? (
        <Card title={t('reports.progress.title')} extra={<Button icon={<ReloadOutlined />} onClick={() => settlementProgressQuery.refetch()} loading={settlementProgressQuery.isFetching}>{t('common.refresh')}</Button>}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap>
            <Input value={progressCurrency} onChange={(event) => setParam('progressCurrency', event.target.value)} placeholder={t('reports.progress.currencyPlaceholder')} style={{ width: 220 }} />
          </Space>
          {settlementProgressQuery.error ? <Alert type="error" showIcon message={t('reports.progress.loadError')} description={(settlementProgressQuery.error as Error).message} /> : null}
          <Space size="large" wrap>
            <Typography.Text>{t('reports.progress.summary.totalCurrencies', { count: progressSummary.totalCurrencies })}</Typography.Text>
            <Typography.Text>{t('reports.progress.summary.totalBills', { count: progressSummary.totalBills })}</Typography.Text>
            <Typography.Text>{t('reports.progress.summary.completedPayableAmount', { amount: progressSummary.completedPayableAmount.toFixed(2) })}</Typography.Text>
            <Typography.Text>{t('reports.progress.summary.pendingPayableAmount', { amount: progressSummary.pendingPayableAmount.toFixed(2) })}</Typography.Text>
          </Space>
          <Table<SettlementProgressReportItem>
            rowKey={(record) => record.currency}
            loading={settlementProgressQuery.isLoading || settlementProgressQuery.isFetching}
            dataSource={settlementProgressQuery.data?.items ?? []}
            columns={progressColumns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            scroll={{ x: 1900 }}
            locale={{ emptyText: <Empty description={t('reports.progress.empty')} /> }}
          />
        </Space>
        </Card>
      ) : null}

      {reportSection === 'settlement' ? (
        <Card title={t('reports.settlement.title')} extra={<Button icon={<ReloadOutlined />} onClick={() => settlementReportQuery.refetch()} loading={settlementReportQuery.isFetching}>{t('common.refresh')}</Button>}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space wrap>
            <Input value={currency} onChange={(event) => setParam('currency', event.target.value)} placeholder={t('reports.settlement.currencyPlaceholder')} style={{ width: 220 }} />
          </Space>
          {settlementReportQuery.error ? <Alert type="error" showIcon message={t('reports.settlement.loadError')} description={(settlementReportQuery.error as Error).message} /> : null}
          <Space size="large" wrap>
            <Typography.Text>{t('reports.settlement.summary.totalCurrencies', { count: settlementSummary.totalCurrencies })}</Typography.Text>
            <Typography.Text>{t('reports.settlement.summary.totalBills', { count: settlementSummary.totalBills })}</Typography.Text>
            <Typography.Text>{t('reports.settlement.summary.totalPayable', { amount: settlementSummary.totalPayable.toFixed(2) })}</Typography.Text>
          </Space>
          <Table<GameSettlementReportItem>
            rowKey={(record) => record.currency}
            loading={settlementReportQuery.isLoading || settlementReportQuery.isFetching}
            dataSource={settlementReportQuery.data?.items ?? []}
            columns={settlementColumns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            scroll={{ x: 1400 }}
            locale={{ emptyText: <Empty description={t('reports.settlement.empty')} /> }}
          />
        </Space>
        </Card>
      ) : null}
    </Space>
  )
}
