import { CheckCircleOutlined, ClockCircleOutlined, CloseCircleOutlined, DollarOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Descriptions,
  Empty,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Timeline,
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
import {
  type AuditWithdrawalRequestPayload,
  type ProcessWithdrawalPayoutPayload,
  type WithdrawalPayoutAction,
  type WithdrawalRequest,
  type WithdrawalRequestStatus,
  type WithdrawalReviewAction,
  apiClient,
} from '../../lib/api'
import { PlatformScopeSummary } from '../../platform-scope/PlatformScopeSummary'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const allStatuses: WithdrawalRequestStatus[] = ['pending', 'approved', 'paying', 'paid', 'failed', 'returned', 'rejected', 'closed', 'cancelled']
const approvedFlowStatuses: WithdrawalRequestStatus[] = ['approved', 'paying', 'paid', 'failed', 'returned']
const reviewActions: WithdrawalReviewAction[] = ['approve', 'reject']
const payoutActions: WithdrawalPayoutAction[] = ['success', 'fail', 'return']

const statusColorMap: Record<WithdrawalRequestStatus, string> = {
  pending: 'processing',
  approved: 'success',
  paying: 'blue',
  paid: 'gold',
  failed: 'error',
  returned: 'orange',
  rejected: 'red',
  closed: 'default',
  cancelled: 'default',
}

type ReviewFormValues = AuditWithdrawalRequestPayload
type PayoutFormValues = ProcessWithdrawalPayoutPayload & { receiptPayloadText?: string }
type ActionMode = 'review' | 'payout' | null

function getActionMode(record: WithdrawalRequest): ActionMode {
  if (record.status === 'pending') {
    return 'review'
  }
  if (record.status === 'approved') {
    return 'payout'
  }
  return null
}

function getReviewTimelineColor(status: WithdrawalRequestStatus) {
  if (approvedFlowStatuses.includes(status)) {
    return 'green'
  }
  if (status === 'rejected') {
    return 'red'
  }
  return 'gray'
}

function getPayoutTimelineColor(status: WithdrawalRequestStatus) {
  if (status === 'paid') {
    return 'gold'
  }
  if (status === 'failed') {
    return 'red'
  }
  if (status === 'returned') {
    return 'orange'
  }
  if (status === 'paying') {
    return 'blue'
  }
  return 'gray'
}

export default function WithdrawalRequestsPage() {
  const { t } = useI18n()
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const { hasPermission } = useAuth()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const requestNo = searchParams.get('requestNo') || ''
  const status = (searchParams.get('status') as WithdrawalRequestStatus | null) ?? undefined
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
  const [selectedRequest, setSelectedRequest] = useState<WithdrawalRequest | null>(null)
  const [actionMode, setActionMode] = useState<ActionMode>(null)
  const [reviewForm] = Form.useForm<ReviewFormValues>()
  const [payoutForm] = Form.useForm<PayoutFormValues>()
  const canAudit = hasPermission('withdrawal-requests:audit')
  const section = useMemo<'list' | 'process'>(() => (location.pathname.endsWith('/process') ? 'process' : 'list'), [location.pathname])

  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['withdrawal-requests', requestNo, status, agentID, activeTenantId, activeBrandId],
    queryFn: () => apiClient.listWithdrawalRequests({
      requestNo: requestNo.trim() || undefined,
      status,
      agentID,
      ...scopeParams,
    }),
  })

  const closeModal = () => {
    setSelectedRequest(null)
    setActionMode(null)
    reviewForm.resetFields()
    payoutForm.resetFields()
  }

  const invalidateLists = () => {
    queryClient.invalidateQueries({ queryKey: ['withdrawal-requests'] })
    queryClient.invalidateQueries({ queryKey: ['risk-cases'] })
    queryClient.invalidateQueries({ queryKey: ['risk-agent-accounts'] })
  }

  const reviewMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: AuditWithdrawalRequestPayload }) => apiClient.auditWithdrawalRequest(id, payload),
    onSuccess: (_, variables) => {
      message.success(t(`withdrawalRequests.actionSuccess.${variables.payload.action}`))
      closeModal()
      invalidateLists()
    },
  })

  const payoutMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: ProcessWithdrawalPayoutPayload }) => apiClient.processWithdrawalPayout(id, payload),
    onSuccess: (_, variables) => {
      message.success(t(`withdrawalRequests.actionSuccess.${variables.payload.action}`))
      closeModal()
      invalidateLists()
    },
  })

  const items = query.data?.items ?? []
  const summary = useMemo(() => ({
    total: items.length,
    pending: items.filter((item) => item.status === 'pending').length,
    approvedAmount: items
      .filter((item) => approvedFlowStatuses.includes(item.status))
      .reduce((sum, item) => sum + Number(item.amount || 0), 0),
    paidAmount: items
      .filter((item) => item.status === 'paid')
      .reduce((sum, item) => sum + Number(item.netAmount ?? item.amount ?? 0), 0),
  }), [items])

  const openActionModal = (record: WithdrawalRequest) => {
    const mode = getActionMode(record)
    if (!mode) {
      return
    }

    setSelectedRequest(record)
    setActionMode(mode)

    if (mode === 'review') {
      reviewForm.setFieldsValue({
        action: 'approve',
        remark: record.remark,
      })
      payoutForm.resetFields()
      return
    }

    payoutForm.setFieldsValue({
      action: 'success',
      reference: record.requestNo,
      remark: record.remark,
      receiptPayloadText: '',
    })
    reviewForm.resetFields()
  }

  const handleSubmit = async () => {
    if (!selectedRequest || !actionMode) {
      return
    }

    try {
      if (actionMode === 'review') {
        const values = await reviewForm.validateFields()
        reviewMutation.mutate({ id: selectedRequest.id, payload: values })
        return
      }

      const values = await payoutForm.validateFields()
      let receiptPayload: Record<string, unknown> | undefined
      const receiptPayloadText = values.receiptPayloadText?.trim()

      if (receiptPayloadText) {
        const parsed = JSON.parse(receiptPayloadText) as unknown
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          throw new Error(t('withdrawalRequests.form.receiptPayloadInvalid'))
        }
        receiptPayload = parsed as Record<string, unknown>
      }

      payoutMutation.mutate({
        id: selectedRequest.id,
        payload: {
          action: values.action,
          reference: values.reference,
          remark: values.remark,
          receiptPayload,
        },
      })
    } catch (error) {
      message.error((error as Error).message)
    }
  }

  const columns: ColumnsType<WithdrawalRequest> = [
    {
      title: t('withdrawalRequests.columns.requestNo'),
      key: 'requestNo',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.requestNo}</Typography.Text>
          <Typography.Text type="secondary">#{record.id}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('withdrawalRequests.columns.agent'),
      key: 'agent',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{record.agentName || `#${record.agentID}`}</Typography.Text>
          <Typography.Text type="secondary">ID {record.agentID}</Typography.Text>
        </Space>
      ),
    },
    { title: t('withdrawalRequests.columns.currency'), dataIndex: 'currency', width: 100 },
    { title: t('withdrawalRequests.columns.amount'), dataIndex: 'amount', width: 120 },
    {
      title: t('withdrawalRequests.columns.netAmount'),
      key: 'netAmount',
      width: 140,
      render: (_, record) => record.netAmount ?? record.amount,
    },
    {
      title: t('withdrawalRequests.columns.status'),
      dataIndex: 'status',
      width: 120,
      render: (value: WithdrawalRequestStatus) => <Tag color={statusColorMap[value]}>{t(`withdrawalRequests.status.${value}`)}</Tag>,
    },
    {
      title: t('withdrawalRequests.columns.lifecycle'),
      key: 'lifecycle',
      width: 260,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{record.submittedAt || record.createdAt}</Typography.Text>
          <Typography.Text type="secondary">
            {approvedFlowStatuses.includes(record.status)
              ? t('withdrawalRequests.lifecycle.approvedAt', { value: record.approvedAt || t('common.none') })
              : record.status === 'rejected'
                ? t('withdrawalRequests.lifecycle.reviewRejected')
                : t('withdrawalRequests.lifecycle.awaitingApproval')}
          </Typography.Text>
          <Typography.Text type="secondary">
            {record.status === 'paid'
              ? t('withdrawalRequests.lifecycle.paidAt', { value: record.paidAt || t('common.none') })
              : record.status === 'paying'
                ? t('withdrawalRequests.lifecycle.paying')
                : record.status === 'failed'
                  ? t('withdrawalRequests.lifecycle.failed')
                  : record.status === 'returned'
                    ? t('withdrawalRequests.lifecycle.returned')
                    : record.status === 'approved'
                      ? t('withdrawalRequests.lifecycle.awaitingPayout')
                      : t('withdrawalRequests.lifecycle.noPayoutYet')}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: t('withdrawalRequests.columns.actions'),
      key: 'actions',
      width: 180,
      render: (_, record) => {
        const mode = getActionMode(record)
        return (
          <Button onClick={() => openActionModal(record)} disabled={!canAudit || !mode}>
            {mode === 'payout' ? t('withdrawalRequests.payout') : t('withdrawalRequests.audit')}
          </Button>
        )
      },
    },
  ]

  const modalOpen = !!selectedRequest && !!actionMode
  const filteredItems = useMemo(
    () => (section === 'process' ? items.filter((item) => item.status === 'pending' || item.status === 'approved') : items),
    [items, section],
  )
  const sectionTitle = section === 'process' ? t('withdrawalRequests.section.processTitle') : t('withdrawalRequests.tableTitle')
  const sectionSubtitle = section === 'process' ? t('withdrawalRequests.section.processSubtitle') : t('withdrawalRequests.section.listSubtitle')

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
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('withdrawalRequests.summary.total')} value={summary.total} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('withdrawalRequests.summary.pending')} value={summary.pending} prefix={<ReloadOutlined />} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('withdrawalRequests.summary.approvedAmount')} value={summary.approvedAmount} precision={2} prefix={<CheckCircleOutlined />} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('withdrawalRequests.summary.paidAmount')} value={summary.paidAmount} precision={2} prefix={<DollarOutlined />} /></Card></Col>
          </Row>
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('withdrawalRequests.searchPlaceholder')}
                value={requestNo}
                onChange={(event) => setParam('requestNo', event.target.value)}
                onSearch={(value) => setParam('requestNo', value)}
                style={{ width: 260 }}
              />
              <Select<WithdrawalRequestStatus | undefined>
                allowClear
                placeholder={t('withdrawalRequests.statusPlaceholder')}
                value={status}
                onChange={(value) => setParam('status', value)}
                style={{ width: 180 }}
                options={allStatuses.map((value) => ({ value, label: t(`withdrawalRequests.status.${value}`) }))}
              />
              <AgentSelect
                tenantID={scopeParams.tenantID}
                brandID={scopeParams.brandID}
                value={agentID}
                onChange={(value) => setParam('agentID', value)}
                placeholder={t('withdrawalRequests.agentIdPlaceholder')}
                style={{ width: 160 }}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('withdrawalRequests.loadError')} description={(query.error as Error).message} /> : null}
      {reviewMutation.error ? <Alert type="error" showIcon message={t('withdrawalRequests.reviewError')} description={(reviewMutation.error as Error).message} /> : null}
      {payoutMutation.error ? <Alert type="error" showIcon message={t('withdrawalRequests.payoutError')} description={(payoutMutation.error as Error).message} /> : null}

      <Card className="app-table-card" title={sectionTitle} extra={<Typography.Text type="secondary">{t('withdrawalRequests.results', { count: filteredItems.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<WithdrawalRequest>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={filteredItems}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            scroll={{ x: 1360 }}
            expandable={{
              expandedRowRender: (record) => (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  <Descriptions size="small" column={2} bordered>
                    <Descriptions.Item label={t('withdrawalRequests.detail.bankAccountName')}>{record.bankAccountName || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.bankName')}>{record.bankName || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.bankAccountNo')}>{record.bankAccountNo || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.reviewedBy')}>{record.approvedBy || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.reviewedAt')}>{record.approvedAt || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.paidAt')}>{record.paidAt || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.feeAmount')}>{record.feeAmount ?? t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.taxAmount')}>{record.taxAmount ?? t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.accountId')}>{record.accountID ?? t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.riskNote')} span={2}>{record.riskNote || t('common.none')}</Descriptions.Item>
                    <Descriptions.Item label={t('withdrawalRequests.detail.remark')} span={2}>{record.remark || t('common.none')}</Descriptions.Item>
                  </Descriptions>
                  <Card size="small" title={t('withdrawalRequests.lifecycle.title')}>
                    <Timeline
                      items={[
                        {
                          color: 'blue',
                          dot: <ClockCircleOutlined />,
                          children: `${t('withdrawalRequests.lifecycle.submitted')} · ${record.submittedAt || record.createdAt}`,
                        },
                        {
                          color: getReviewTimelineColor(record.status),
                          dot: <CheckCircleOutlined />,
                          children: approvedFlowStatuses.includes(record.status)
                            ? `${t('withdrawalRequests.lifecycle.approved')} · ${record.approvedAt || t('common.none')}${record.approvedBy ? ` · ${record.approvedBy}` : ''}`
                            : record.status === 'rejected'
                              ? `${t('withdrawalRequests.lifecycle.rejected')} · ${record.approvedAt || t('common.none')}`
                              : t('withdrawalRequests.lifecycle.awaitingApproval'),
                        },
                        {
                          color: getPayoutTimelineColor(record.status),
                          dot: <DollarOutlined />,
                          children: record.status === 'paid'
                            ? `${t('withdrawalRequests.lifecycle.paid')} · ${record.paidAt || t('common.none')}`
                            : record.status === 'paying'
                              ? t('withdrawalRequests.lifecycle.paying')
                              : record.status === 'failed'
                                ? t('withdrawalRequests.lifecycle.failed')
                                : record.status === 'returned'
                                  ? t('withdrawalRequests.lifecycle.returned')
                                  : record.status === 'approved'
                                    ? t('withdrawalRequests.lifecycle.awaitingPayout')
                                    : record.status === 'closed'
                                      ? t('withdrawalRequests.lifecycle.closed')
                                      : record.status === 'cancelled'
                                        ? t('withdrawalRequests.lifecycle.cancelled')
                                        : t('withdrawalRequests.lifecycle.noPayoutYet'),
                        },
                      ]}
                    />
                  </Card>
                </Space>
              ),
            }}
            locale={{ emptyText: <Empty description={t('withdrawalRequests.empty')} /> }}
          />
        </div>
      </Card>

      <Modal
        title={actionMode === 'payout' ? t('withdrawalRequests.payoutModalTitle') : t('withdrawalRequests.reviewModalTitle')}
        open={modalOpen}
        onCancel={closeModal}
        onOk={() => void handleSubmit()}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
        confirmLoading={reviewMutation.isPending || payoutMutation.isPending}
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {selectedRequest ? (
            <Card size="small">
              <Descriptions size="small" column={1}>
                <Descriptions.Item label={t('withdrawalRequests.columns.requestNo')}>{selectedRequest.requestNo}</Descriptions.Item>
                <Descriptions.Item label={t('withdrawalRequests.columns.agent')}>{selectedRequest.agentName || `#${selectedRequest.agentID}`}</Descriptions.Item>
                <Descriptions.Item label={t('withdrawalRequests.columns.amount')}>{selectedRequest.amount} {selectedRequest.currency}</Descriptions.Item>
                <Descriptions.Item label={t('withdrawalRequests.columns.status')}>{t(`withdrawalRequests.status.${selectedRequest.status}`)}</Descriptions.Item>
              </Descriptions>
            </Card>
          ) : null}

          {actionMode === 'review' ? (
            <Form<ReviewFormValues> form={reviewForm} layout="vertical">
              <Form.Item name="action" label={t('withdrawalRequests.form.reviewAction')} rules={[{ required: true, message: t('withdrawalRequests.form.reviewActionRequired') }]}>
                <Select
                  options={reviewActions.map((value) => ({
                    value,
                    label: t(`withdrawalRequests.action.${value}`),
                  }))}
                />
              </Form.Item>
              <Form.Item name="remark" label={t('withdrawalRequests.form.remark')}>
                <Input.TextArea rows={3} placeholder={t('withdrawalRequests.form.remarkPlaceholder')} />
              </Form.Item>
            </Form>
          ) : (
            <Form<PayoutFormValues> form={payoutForm} layout="vertical">
              <Form.Item name="action" label={t('withdrawalRequests.form.payoutAction')} rules={[{ required: true, message: t('withdrawalRequests.form.payoutActionRequired') }]}>
                <Select
                  options={payoutActions.map((value) => ({
                    value,
                    label: t(`withdrawalRequests.action.${value}`),
                  }))}
                />
              </Form.Item>
              <Form.Item name="reference" label={t('withdrawalRequests.form.reference')} rules={[{ required: true, message: t('withdrawalRequests.form.referenceRequired') }]}>
                <Input placeholder={t('withdrawalRequests.form.referencePlaceholder')} />
              </Form.Item>
              <Form.Item name="receiptPayloadText" label={t('withdrawalRequests.form.receiptPayload')}>
                <Input.TextArea rows={4} placeholder={t('withdrawalRequests.form.receiptPayloadPlaceholder')} />
              </Form.Item>
              <Form.Item name="remark" label={t('withdrawalRequests.form.remark')}>
                <Input.TextArea rows={3} placeholder={t('withdrawalRequests.form.remarkPlaceholder')} />
              </Form.Item>
            </Form>
          )}

          <Alert
            type="info"
            showIcon
            icon={<CloseCircleOutlined />}
            message={actionMode === 'payout' ? t('withdrawalRequests.payoutHint') : t('withdrawalRequests.reviewHint')}
          />
        </Space>
      </Modal>
    </Space>
  )
}
