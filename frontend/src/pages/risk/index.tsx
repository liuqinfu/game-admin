import {
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  FundProjectionScreenOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  StopOutlined,
} from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useLocation, useSearchParams } from 'react-router-dom'
import { useAuth } from '../../auth'
import { AgentSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { type ReviewRiskCasePayload, type RiskCase, type RiskCaseStatus, type RiskIntelligenceItem, type RiskLevel, apiClient } from '../../lib/api'
import { PlatformScopeSummary } from '../../platform-scope/PlatformScopeSummary'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const riskColorMap: Record<RiskLevel, string> = {
  low: 'success',
  medium: 'warning',
  high: 'error',
}

const caseStatusColorMap: Record<RiskCaseStatus, string> = {
  pending: 'processing',
  released: 'default',
  confirmed: 'success',
}

const riskReviewActionOptions: ReviewRiskCasePayload['action'][] = ['confirm', 'release']

type RiskReviewFormValues = ReviewRiskCasePayload

export default function RiskPage() {
  const { t } = useI18n()
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const { hasPermission } = useAuth()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const minFrozenRatioParam = searchParams.get('minFrozenRatio')
  const minFrozenRatio = minFrozenRatioParam ? Number(minFrozenRatioParam) : 0.5
  const riskLevel = (searchParams.get('riskLevel') as RiskLevel | null) ?? undefined
  const status = (searchParams.get('status') as RiskCaseStatus | null) ?? undefined
  const agentID = Number(searchParams.get('agentID') || '') || undefined
  const setParam = (key: string, value?: string | number) => {
    const next = new URLSearchParams(searchParams)
    if (value === undefined || value === '') {
      next.delete(key)
    } else {
      next.set(key, String(value))
    }
    setSearchParams(next, { replace: true })
  }
  const [selectedCase, setSelectedCase] = useState<RiskCase | null>(null)
  const [reviewOpen, setReviewOpen] = useState(false)
  const [reviewForm] = Form.useForm<RiskReviewFormValues>()
  const canReview = hasPermission('risk:view')
  const section = useMemo<'cases' | 'intelligence'>(() => (location.pathname.endsWith('/intelligence') ? 'intelligence' : 'cases'), [location.pathname])

  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['risk-cases', minFrozenRatio, riskLevel, status, agentID, activeTenantId, activeBrandId],
    queryFn: () => apiClient.listRiskCases({ minFrozenRatio, riskLevel, status, agentID, ...scopeParams }),
    enabled: section === 'cases',
  })

  const intelligenceQuery = useQuery({
    queryKey: ['risk-intelligence', activeTenantId, activeBrandId],
    queryFn: () => apiClient.listRiskIntelligence(scopeParams),
    enabled: section === 'intelligence',
  })

  const reviewMutation = useMutation({
    mutationFn: ({ caseNo, payload }: { caseNo: string; payload: ReviewRiskCasePayload }) => apiClient.reviewRiskCase(caseNo, payload),
    onSuccess: (_, variables) => {
      message.success(t(`risk.reviewActionSuccess.${variables.payload.action}`))
      setReviewOpen(false)
      setSelectedCase(null)
      reviewForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['risk-cases'] })
      queryClient.invalidateQueries({ queryKey: ['withdrawal-requests'] })
      queryClient.invalidateQueries({ queryKey: ['risk-agent-accounts'] })
    },
  })

  const items = query.data?.items ?? []
  const summary = useMemo(() => ({
    total: items.length,
    pending: items.filter((item) => item.status === 'pending').length,
    confirmed: items.filter((item) => item.status === 'confirmed').length,
    released: items.filter((item) => item.status === 'released').length,
  }), [items])

  const openReviewModal = (record: RiskCase) => {
    setSelectedCase(record)
    reviewForm.setFieldsValue({
      action: 'confirm',
      remark: record.riskNote,
    })
    setReviewOpen(true)
  }

  const closeReviewModal = () => {
    setReviewOpen(false)
    setSelectedCase(null)
    reviewForm.resetFields()
  }

  const handleReview = async () => {
    if (!selectedCase) {
      return
    }
    const values = await reviewForm.validateFields()
    reviewMutation.mutate({ caseNo: selectedCase.caseNo, payload: values })
  }

  const renderRiskIntelligenceReason = (record: RiskIntelligenceItem) => {
    if (record.sourceType === 'risk_case') {
      return record.reason || t('risk.intelligence.reason.risk_case.default')
    }
    if (record.sourceType === 'withdrawal_request') {
      return t(`risk.intelligence.reason.withdrawal.${record.status}`)
    }
    if (record.sourceType === 'agent_network') {
      return t('risk.intelligence.reason.agent_network')
    }
    return record.reason || t('common.none')
  }

  const columns: ColumnsType<RiskCase> = [
    {
      title: t('risk.columns.caseNo'),
      key: 'caseNo',
      width: 180,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.caseNo}</Typography.Text>
          <Typography.Text type="secondary">{record.createdAt}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('risk.columns.agent'),
      key: 'agent',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.agentName || `#${record.agentID}`}</Typography.Text>
          <Typography.Text type="secondary">ID {record.agentID}</Typography.Text>
        </Space>
      ),
    },
    { title: t('risk.columns.currency'), dataIndex: 'currency', width: 100 },
    {
      title: t('risk.columns.riskLevel'),
      dataIndex: 'riskLevel',
      width: 120,
      render: (value: RiskLevel) => <Tag color={riskColorMap[value] ?? 'default'}>{t(`risk.level.${value}`)}</Tag>,
    },
    {
      title: t('risk.columns.caseStatus'),
      dataIndex: 'status',
      width: 140,
      render: (value: RiskCaseStatus) => <Tag color={caseStatusColorMap[value] ?? 'default'}>{t(`risk.caseStatus.${value}`)}</Tag>,
    },
    {
      title: t('risk.columns.frozenRatio'),
      dataIndex: 'frozenRatio',
      width: 140,
      render: (value: number) => `${(Number(value || 0) * 100).toFixed(2)}%`,
    },
    { title: t('risk.columns.frozenBalance'), dataIndex: 'frozenBalance', width: 140 },
    { title: t('risk.columns.withdrawableAmount'), dataIndex: 'withdrawableAmount', width: 160 },
    {
      title: t('risk.columns.reason'),
      dataIndex: 'reason',
      width: 280,
      ellipsis: true,
    },
    {
      title: t('risk.columns.actions'),
      key: 'actions',
      width: 160,
      render: (_, record) => (
        <Button onClick={() => openReviewModal(record)} disabled={!canReview || record.status !== 'pending'}>
          {t('risk.review')}
        </Button>
      ),
    },
    { title: t('risk.columns.updatedAt'), dataIndex: 'updatedAt', width: 180 },
  ]

  const intelligenceColumns: ColumnsType<RiskIntelligenceItem> = [
    {
      title: t('risk.intelligence.columns.source'),
      key: 'source',
      width: 220,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{t(`risk.intelligence.sourceType.${record.sourceType}`)}</Typography.Text>
          <Typography.Text type="secondary">{record.sourceID}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('risk.intelligence.columns.agent'),
      key: 'agent',
      width: 180,
      render: (_, record) => record.agentID ? (record.agentName || `#${record.agentID}`) : t('common.none'),
    },
    {
      title: t('risk.intelligence.columns.score'),
      dataIndex: 'score',
      width: 110,
      render: (value: number) => Number(value ?? 0).toFixed(2),
    },
    {
      title: t('risk.intelligence.columns.riskLevel'),
      dataIndex: 'riskLevel',
      width: 120,
      render: (value: RiskLevel) => <Tag color={riskColorMap[value] ?? 'default'}>{t(`risk.level.${value}`)}</Tag>,
    },
    {
      title: t('risk.intelligence.columns.intercepted'),
      dataIndex: 'intercepted',
      width: 120,
      render: (value: boolean) => <Tag color={value ? 'error' : 'default'}>{value ? t('common.yes') : t('common.no')}</Tag>,
    },
    {
      title: t('risk.intelligence.columns.action'),
      dataIndex: 'recommendedAction',
      width: 180,
      render: (value: string) => t(`risk.intelligence.action.${value}`),
    },
    {
      title: t('risk.intelligence.columns.reason'),
      dataIndex: 'reason',
      ellipsis: true,
      render: (_value, record) => renderRiskIntelligenceReason(record),
    },
  ]

  const sectionTitle = section === 'cases' ? t('risk.tableTitle') : t('risk.intelligence.title')
  const sectionSubtitle = section === 'cases' ? t('risk.section.casesSubtitle') : t('risk.section.intelligenceSubtitle')

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">{sectionTitle}</Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">{sectionSubtitle}</Typography.Paragraph>
          </div>
          <Alert
            type="warning"
            showIcon
            icon={<ExclamationCircleOutlined />}
            message={t('risk.manualReviewBanner')}
            description={t('risk.manualReviewDescription')}
          />
          <PlatformScopeSummary />
          <Space size="middle" wrap>
            <Card className="app-stat-card"><Statistic title={t('risk.summary.totalCases')} value={summary.total} prefix={<SafetyCertificateOutlined />} /></Card>
            <Card className="app-stat-card"><Statistic title={t('risk.summary.pendingCases')} value={summary.pending} prefix={<ExclamationCircleOutlined />} /></Card>
            <Card className="app-stat-card"><Statistic title={t('risk.summary.confirmedCases')} value={summary.confirmed} prefix={<CheckCircleOutlined />} /></Card>
            <Card className="app-stat-card"><Statistic title={t('risk.summary.releasedCases')} value={summary.released} prefix={<StopOutlined />} /></Card>
          </Space>
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <InputNumber
                min={0}
                max={1}
                step={0.05}
                value={minFrozenRatio}
                onChange={(value) => setParam('minFrozenRatio', value ?? undefined)}
                placeholder={t('risk.minFrozenRatioPlaceholder')}
                style={{ width: 180 }}
              />
              <AgentSelect
                tenantID={scopeParams.tenantID}
                brandID={scopeParams.brandID}
                value={agentID}
                onChange={(value) => setParam('agentID', value)}
                placeholder={t('risk.agentIdPlaceholder')}
                style={{ width: 160 }}
              />
              <Select<RiskLevel | undefined>
                allowClear
                value={riskLevel}
                onChange={(value) => setParam('riskLevel', value)}
                placeholder={t('risk.riskLevelPlaceholder')}
                style={{ width: 180 }}
                options={(['low', 'medium', 'high'] as RiskLevel[]).map((item) => ({ label: t(`risk.level.${item}`), value: item }))}
              />
              <Select<RiskCaseStatus | undefined>
                allowClear
                value={status}
                onChange={(value) => setParam('status', value)}
                placeholder={t('risk.caseStatusPlaceholder')}
                style={{ width: 180 }}
                options={(['pending', 'released', 'confirmed'] as RiskCaseStatus[]).map((item) => ({ label: t(`risk.caseStatus.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  if (section === 'cases') {
                    query.refetch()
                  } else {
                    intelligenceQuery.refetch()
                  }
                }}
                loading={query.isFetching || intelligenceQuery.isFetching}
              >
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('risk.loadError')} description={(query.error as Error).message} /> : null}
      {intelligenceQuery.error ? <Alert type="error" showIcon message={t('risk.intelligence.loadError')} description={(intelligenceQuery.error as Error).message} /> : null}
      {reviewMutation.error ? <Alert type="error" showIcon message={t('risk.reviewError')} description={(reviewMutation.error as Error).message} /> : null}

      {section === 'intelligence' ? <Card
        className="app-table-card"
        title={t('risk.intelligence.title')}
        extra={<Typography.Text type="secondary">{t('risk.intelligence.results', { count: intelligenceQuery.data?.items.length ?? 0 })}</Typography.Text>}
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert
            type="info"
            showIcon
            icon={<FundProjectionScreenOutlined />}
            message={t('risk.intelligence.banner')}
            description={t('risk.intelligence.description')}
          />
          <div className="app-table app-table--compact">
            <Table<RiskIntelligenceItem>
              rowKey={(record) => `${record.sourceType}-${record.sourceID}`}
              loading={intelligenceQuery.isLoading || intelligenceQuery.isFetching}
              dataSource={intelligenceQuery.data?.items ?? []}
              columns={intelligenceColumns}
              pagination={{ pageSize: 5, showSizeChanger: false }}
              scroll={{ x: 1200 }}
              locale={{ emptyText: <Empty description={t('risk.intelligence.empty')} /> }}
            />
          </div>
        </Space>
      </Card> : null}

      {section === 'cases' ? <Card className="app-table-card" title={t('risk.tableTitle')} extra={<Typography.Text type="secondary">{t('risk.results', { count: items.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<RiskCase>
            rowKey="caseNo"
            loading={query.isLoading || query.isFetching}
            dataSource={items}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            scroll={{ x: 1580 }}
            expandable={{
              expandedRowRender: (record) => (
                <Descriptions size="small" column={2} bordered>
                  <Descriptions.Item label={t('risk.detail.caseNo')}>{record.caseNo}</Descriptions.Item>
                  <Descriptions.Item label={t('risk.detail.freezeRequested')}>
                    {record.freezeRequested ? t('common.yes') : t('common.no')}
                  </Descriptions.Item>
                  <Descriptions.Item label={t('risk.detail.reason')} span={2}>{record.reason}</Descriptions.Item>
                  <Descriptions.Item label={t('risk.detail.latestWithdrawalRequestNo')}>
                    {record.latestWithdrawalRequestNo || t('common.none')}
                  </Descriptions.Item>
                  <Descriptions.Item label={t('risk.detail.latestWithdrawalAmount')}>
                    {record.latestWithdrawalAmount !== undefined ? record.latestWithdrawalAmount : t('common.none')}
                  </Descriptions.Item>
                  <Descriptions.Item label={t('risk.detail.riskNote')} span={2}>{record.riskNote || t('common.none')}</Descriptions.Item>
                </Descriptions>
              ),
            }}
            locale={{ emptyText: <Empty description={t('risk.empty')} /> }}
          />
        </div>
      </Card> : null}

      <Modal
        title={t('risk.modalTitle')}
        open={reviewOpen && !!selectedCase}
        onCancel={closeReviewModal}
        onOk={() => void handleReview()}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
        confirmLoading={reviewMutation.isPending}
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {selectedCase ? (
            <Card size="small">
              <Descriptions size="small" column={1}>
                <Descriptions.Item label={t('risk.columns.caseNo')}>{selectedCase.caseNo}</Descriptions.Item>
                <Descriptions.Item label={t('risk.columns.agent')}>{selectedCase.agentName || `#${selectedCase.agentID}`}</Descriptions.Item>
                <Descriptions.Item label={t('risk.columns.caseStatus')}>{t(`risk.caseStatus.${selectedCase.status}`)}</Descriptions.Item>
                <Descriptions.Item label={t('risk.columns.frozenBalance')}>{selectedCase.frozenBalance} {selectedCase.currency}</Descriptions.Item>
              </Descriptions>
            </Card>
          ) : null}
          <Form<RiskReviewFormValues> form={reviewForm} layout="vertical">
            <Form.Item name="action" label={t('risk.form.action')} rules={[{ required: true, message: t('risk.form.actionRequired') }]}>
              <Select
                options={riskReviewActionOptions.map((value) => ({
                  value,
                  label: t(`risk.action.${value}`),
                }))}
              />
            </Form.Item>
            <Form.Item name="remark" label={t('risk.form.remark')}>
              <Input.TextArea rows={3} placeholder={t('risk.form.remarkPlaceholder')} />
            </Form.Item>
          </Form>
          <Alert type="info" showIcon message={t('risk.reviewHint')} />
        </Space>
      </Modal>
    </Space>
  )
}
