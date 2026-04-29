import { ArrowUpOutlined, LinkOutlined, ReloadOutlined, SafetyCertificateOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Col, Empty, Progress, Row, Space, Statistic, Table, Tag, Typography } from 'antd'
import { useMemo } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { buildOperatingMetricComparisons, findScopedOperatingConfigs } from '../../config/operating-params'
import { useI18n } from '../../i18n'
import { Agent, DataPlatformMetricItem, Game, InviteCode, Rule, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

export default function DashboardPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const dashboardQuery = useQuery({
    queryKey: ['dashboard', scopeParams.tenantID, scopeParams.brandID],
    queryFn: async () => {
      const [agents, inviteCodes, games, rules, platformConfigs, metrics] = await Promise.all([
        apiClient.listAgents(scopeParams),
        apiClient.listInviteCodes(scopeParams),
        apiClient.listGames(scopeParams),
        apiClient.listRules(scopeParams),
        apiClient.listPlatformConfigs(scopeParams),
        apiClient.listDataPlatformMetrics(scopeParams),
      ])
      return { agents, inviteCodes, games, rules, platformConfigs, metrics }
    },
  })

  const stats = useMemo(() => {
    const agents = dashboardQuery.data?.agents.items ?? []
    const inviteCodes = dashboardQuery.data?.inviteCodes.items ?? []
    const games = dashboardQuery.data?.games.items ?? []
    const rules = dashboardQuery.data?.rules.items ?? []
    const totalResources = agents.length + inviteCodes.length + games.length + rules.length

    return {
      agents: agents.length,
      activeAgents: agents.filter((item) => item.status === 'active').length,
      inviteCodes: inviteCodes.length,
      activeInviteCodes: inviteCodes.filter((item) => item.status === 'active').length,
      games: games.length,
      onlineGames: games.filter((item) => item.status === 'online').length,
      rules: rules.length,
      publishedRules: rules.filter((item) => item.status === 'published').length,
      totalResources,
      coverage: totalResources > 0
        ? Math.round(((agents.filter((item) => item.status === 'active').length
          + inviteCodes.filter((item) => item.status === 'active').length
          + games.filter((item) => item.status === 'online').length
          + rules.filter((item) => item.status === 'published').length) / totalResources) * 100)
        : 0,
      agentActivationRate: agents.length > 0 ? Math.round((agents.filter((item) => item.status === 'active').length / agents.length) * 100) : 0,
      inviteAvailabilityRate: inviteCodes.length > 0 ? Math.round((inviteCodes.filter((item) => item.status === 'active').length / inviteCodes.length) * 100) : 0,
      catalogReadinessRate: games.length + rules.length > 0
        ? Math.round(((games.filter((item) => item.status === 'online').length + rules.filter((item) => item.status === 'published').length) / (games.length + rules.length)) * 100)
        : 0,
      recentAgents: agents.slice(0, 5),
      recentGames: games.slice(0, 5),
      recentInviteCodes: inviteCodes.slice(0, 5),
      recentRules: rules.slice(0, 5),
    }
  }, [dashboardQuery.data])

  const operatingSummary = useMemo(() => {
    const configs = dashboardQuery.data?.platformConfigs.items ?? []
    const metrics = dashboardQuery.data?.metrics.items ?? []
    const scopedConfigs = findScopedOperatingConfigs(configs)
    const comparisons = buildOperatingMetricComparisons(metrics, configs)
    return {
      configuredCount: scopedConfigs.filter((item) => item.config).length,
      totalCount: scopedConfigs.length,
      missingNames: scopedConfigs.filter((item) => !item.config).map((item) => t(item.template.titleKey)),
      comparisons,
      metricMap: metrics.reduce<Record<string, DataPlatformMetricItem>>((acc, item) => {
        acc[item.key] = item
        return acc
      }, {}),
    }
  }, [dashboardQuery.data?.metrics.items, dashboardQuery.data?.platformConfigs.items, t])

  const operatingCoverage = operatingSummary.totalCount > 0
    ? Math.round((operatingSummary.configuredCount / operatingSummary.totalCount) * 100)
    : 0
  const profitDataCoverage = operatingSummary.metricMap.profitDataCoverage ? Number(operatingSummary.metricMap.profitDataCoverage.value ?? 0) : undefined

  const metricLabelByKey: Record<string, string> = {
    activityROI: t('reports.dataPlatform.metric.activityROI'),
    commissionCost: t('reports.dataPlatform.metric.commissionCost'),
    estimatedNetMargin: t('reports.dataPlatform.metric.estimatedNetMargin'),
    withdrawalSuccessRate: t('reports.dataPlatform.metric.withdrawalSuccessRate'),
    riskInterceptRate: t('reports.dataPlatform.metric.riskInterceptRate'),
  }

  const formatMetricValue = (item?: DataPlatformMetricItem) => {
    if (!item) {
      return '--'
    }
    if (item.unit === 'percent') {
      return `${Number(item.value ?? 0).toFixed(2)}%`
    }
    if (item.unit === 'count') {
      return `${Number(item.value ?? 0).toFixed(0)}`
    }
    return Number(item.value ?? 0).toFixed(2)
  }

  const agentColumns: ColumnsType<Agent> = [
    { title: t('agents.form.name'), dataIndex: 'name' },
    { title: t('agents.form.agentNo'), dataIndex: 'agentNo' },
    { title: t('common.status'), dataIndex: 'status', render: (value: string) => <Tag>{t(`status.${value}`)}</Tag> },
  ]

  const gameColumns: ColumnsType<Game> = [
    { title: t('games.form.name'), dataIndex: 'name' },
    { title: t('games.form.gameCode'), dataIndex: 'gameCode' },
    { title: t('common.status'), dataIndex: 'status', render: (value: string) => <Tag>{t(`status.${value}`)}</Tag> },
  ]

  const inviteColumns: ColumnsType<InviteCode> = [
    { title: t('inviteCodes.form.code'), dataIndex: 'code' },
    { title: t('inviteCodes.columns.agentId'), dataIndex: 'agentID' },
    { title: t('common.status'), dataIndex: 'status', render: (value: string) => <Tag>{t(`status.${value}`)}</Tag> },
  ]

  const ruleColumns: ColumnsType<Rule> = [
    { title: t('rules.columns.rule'), dataIndex: 'ruleName' },
    { title: t('rules.columns.scope'), dataIndex: 'scope', render: (value: string) => <Tag>{t(`scope.${value}`)}</Tag> },
    { title: t('common.status'), dataIndex: 'status', render: (value: string) => <Tag>{t(`status.${value}`)}</Tag> },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />

      <Card className="app-hero-card" bordered={false}>
        <div className="app-section-stack">
            <div className="app-page__heading">
              <Typography.Text className="app-hero-kicker">{t('dashboard.hero.eyebrow')}</Typography.Text>
              <Typography.Title level={2} className="app-page__title">{t('dashboard.title')}</Typography.Title>
              <Typography.Paragraph className="app-page__subtitle">{t('dashboard.subtitle')}</Typography.Paragraph>
            </div>

            <div className="app-chip-row">
              <span className="app-chip">{t('dashboard.hero.activeAgents', { count: stats.activeAgents })}</span>
              <span className="app-chip">{t('dashboard.hero.onlineGames', { count: stats.onlineGames })}</span>
              <span className="app-chip">{t('dashboard.hero.publishedRules', { count: stats.publishedRules })}</span>
            </div>

            <div className="app-summary-strip">
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('dashboard.stats.agents')}</span>
                <span className="app-summary-pill__value">{stats.agents}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('dashboard.stats.inviteCodes')}</span>
                <span className="app-summary-pill__value">{stats.inviteCodes}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('dashboard.stats.games')}</span>
                <span className="app-summary-pill__value">{stats.games}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('dashboard.stats.rules')}</span>
                <span className="app-summary-pill__value">{stats.rules}</span>
              </div>
            </div>

            <div className="app-shell-grid app-shell-grid--triple">
              <Card title={t('dashboard.overview.networkTitle')} className="app-signal-card" bordered={false}>
                <div className="app-signal-card__meta">
                  <span className="app-signal-card__value">{`${stats.agentActivationRate}%`}</span>
                  <span className="app-signal-card__fraction">{`${stats.activeAgents} / ${stats.agents}`}</span>
                </div>
                <Typography.Paragraph className="app-signal-card__description">{t('dashboard.overview.networkDescription')}</Typography.Paragraph>
              </Card>
              <Card title={t('dashboard.overview.growthTitle')} className="app-signal-card" bordered={false}>
                <div className="app-signal-card__meta">
                  <span className="app-signal-card__value">{`${stats.inviteAvailabilityRate}%`}</span>
                  <span className="app-signal-card__fraction">{`${stats.activeInviteCodes} / ${stats.inviteCodes}`}</span>
                </div>
                <Typography.Paragraph className="app-signal-card__description">{t('dashboard.overview.growthDescription')}</Typography.Paragraph>
              </Card>
              <Card title={t('dashboard.overview.catalogTitle')} className="app-signal-card" bordered={false}>
                <div className="app-signal-card__meta">
                  <span className="app-signal-card__value">{`${stats.catalogReadinessRate}%`}</span>
                  <span className="app-signal-card__fraction">{`${stats.onlineGames + stats.publishedRules} / ${stats.games + stats.rules}`}</span>
                </div>
                <Typography.Paragraph className="app-signal-card__description">{t('dashboard.overview.catalogDescription')}</Typography.Paragraph>
              </Card>
            </div>

            <Card className="app-metric-card" bordered={false}>
              <Typography.Text className="app-metric-card__label">{t('dashboard.overview.readiness')}</Typography.Text>
              <Typography.Text className="app-metric-card__value">{`${stats.coverage}%`}</Typography.Text>
              <Progress percent={stats.coverage} showInfo={false} strokeColor="#1d4ed8" trailColor="#dbeafe" />
              <Typography.Text className="app-metric-card__copy">{t('dashboard.overview.readinessHint')}</Typography.Text>
              <Button icon={<ReloadOutlined />} onClick={() => dashboardQuery.refetch()} loading={dashboardQuery.isFetching}>
                {t('common.refresh')}
              </Button>
            </Card>
        </div>
      </Card>

      {dashboardQuery.error ? <Alert type="error" showIcon message={t('dashboard.loadError')} description={(dashboardQuery.error as Error).message} /> : null}

      {profitDataCoverage !== undefined && profitDataCoverage < 100 ? (
        <Alert
          type="warning"
          showIcon
          message={t('dashboard.operating.profitGapAlertTitle', { coverage: profitDataCoverage.toFixed(2) })}
          description={t('dashboard.operating.profitGapAlertDescription', { missing: `${(100 - profitDataCoverage).toFixed(2)}%` })}
          action={(
            <Button type="link" icon={<LinkOutlined />} onClick={() => navigate('/orders?view=profit-gaps')}>
              {t('dashboard.operating.profitGapAlertAction')}
            </Button>
          )}
        />
      ) : null}

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={10}>
          <Card className="app-workspace-card" title={t('dashboard.operating.title')}>
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <div>
                <Space align="baseline" size="middle">
                  <Typography.Text strong>
                    {t('dashboard.operating.coverage', { count: operatingSummary.configuredCount, total: operatingSummary.totalCount })}
                  </Typography.Text>
                  <Typography.Text type="secondary">{`${operatingCoverage}%`}</Typography.Text>
                </Space>
                <Progress percent={operatingCoverage} showInfo={false} strokeColor="#1677ff" />
              </div>
              {operatingSummary.missingNames.length > 0 ? (
                <Alert
                  type="warning"
                  showIcon
                  message={t('dashboard.operating.missingTitle')}
                  description={t('dashboard.operating.missingDescription', { names: operatingSummary.missingNames.join(' / ') })}
                />
              ) : (
                <Alert type="success" showIcon message={t('dashboard.operating.completeTitle')} description={t('dashboard.operating.completeDescription')} />
              )}
            </Space>
          </Card>
        </Col>
        <Col xs={24} xl={14}>
          <Card className="app-workspace-card" title={t('dashboard.operating.kpiTitle')}>
            <Row gutter={[12, 12]}>
              {operatingSummary.comparisons.map((comparison) => {
                const metricItem = comparison.template.metricKey ? operatingSummary.metricMap[comparison.template.metricKey] : undefined
                const tagColor = comparison.status === 'healthy'
                  ? 'success'
                  : comparison.status === 'attention'
                    ? 'error'
                    : comparison.status === 'missing'
                      ? 'default'
                      : 'processing'
                return (
                  <Col key={comparison.template.key} xs={24} md={12}>
                    <Card size="small">
                      <Space direction="vertical" size={8} style={{ width: '100%' }}>
                        <Space align="center" style={{ justifyContent: 'space-between', width: '100%' }}>
                          <Typography.Text strong>{t(comparison.template.titleKey)}</Typography.Text>
                          <Tag color={tagColor}>{t(`dashboard.operating.status.${comparison.status}`)}</Tag>
                        </Space>
                        <Typography.Text type="secondary">
                          {metricLabelByKey[comparison.template.metricKey ?? ''] || t('dashboard.operating.targetMetric')}
                        </Typography.Text>
                        <Typography.Text>{t('dashboard.operating.actualValue', { value: formatMetricValue(metricItem) })}</Typography.Text>
                        <Typography.Text type="secondary">
                          {comparison.targetValue !== undefined
                            ? t('dashboard.operating.targetValue', {
                                value: metricItem?.unit === 'percent'
                                  ? `${comparison.targetValue.toFixed(2)}%`
                                  : comparison.targetValue.toFixed(2),
                              })
                            : t('dashboard.operating.targetMissing')}
                        </Typography.Text>
                      </Space>
                    </Card>
                  </Col>
                )
              })}
            </Row>
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={12}>
          <Card title={t('dashboard.recentAgents')} className="app-workspace-card">
            <div className="app-table app-table--compact">
              <Table<Agent> rowKey="id" pagination={false} dataSource={stats.recentAgents} columns={agentColumns} locale={{ emptyText: <Empty description={t('dashboard.emptyAgents')} /> }} />
            </div>
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title={t('dashboard.recentGames')} className="app-workspace-card">
            <div className="app-table app-table--compact">
              <Table<Game> rowKey="id" pagination={false} dataSource={stats.recentGames} columns={gameColumns} locale={{ emptyText: <Empty description={t('dashboard.emptyGames')} /> }} />
            </div>
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title={t('dashboard.recentInviteCodes')} className="app-workspace-card">
            <div className="app-table app-table--compact">
              <Table<InviteCode> rowKey="id" pagination={false} dataSource={stats.recentInviteCodes} columns={inviteColumns} locale={{ emptyText: <Empty description={t('dashboard.emptyInviteCodes')} /> }} />
            </div>
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card title={t('dashboard.recentRules')} className="app-workspace-card">
            <div className="app-table app-table--compact">
              <Table<Rule> rowKey="id" pagination={false} dataSource={stats.recentRules} columns={ruleColumns} locale={{ emptyText: <Empty description={t('dashboard.emptyRules')} /> }} />
            </div>
          </Card>
        </Col>
      </Row>
    </Space>
  )
}
