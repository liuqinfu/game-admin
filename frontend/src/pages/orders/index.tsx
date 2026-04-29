import { EditOutlined, ReloadOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, Empty, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useSearchParams } from 'react-router-dom'
import { useAuth } from '../../auth'
import { useI18n } from '../../i18n'
import { Order, OrderStatus, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: OrderStatus[] = ['pending', 'paid', 'failed', 'cancelled', 'closed']
const riskReviewStatuses = ['open', 'reviewing', 'resolved'] as const

type OrderRiskVisibility = 'visible' | 'hidden'

const statusColorMap: Record<OrderStatus, string> = {
  pending: 'processing',
  paid: 'success',
  failed: 'error',
  refunded: 'warning',
  cancelled: 'default',
  closed: 'default',
}

const riskVisibilityColorMap: Record<OrderRiskVisibility, string> = {
  visible: 'warning',
  hidden: 'default',
}

const reviewStatusColorMap: Record<(typeof riskReviewStatuses)[number], string> = {
  open: 'error',
  reviewing: 'processing',
  resolved: 'success',
}

function getOrderRiskVisibility(order: Order): OrderRiskVisibility {
  return order.freezeVisible === false ? 'hidden' : 'visible'
}

function getOrderReviewStatus(order: Order): (typeof riskReviewStatuses)[number] | undefined {
  if (order.reviewStatus && riskReviewStatuses.includes(order.reviewStatus as (typeof riskReviewStatuses)[number])) {
    return order.reviewStatus as (typeof riskReviewStatuses)[number]
  }
  if (order.riskNote || order.freezeReason || order.freezeVisible !== undefined) {
    return getOrderRiskVisibility(order) === 'visible' ? 'reviewing' : 'resolved'
  }
  return undefined
}

export default function OrdersPage() {
  const { t } = useI18n()
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [searchParams, setSearchParams] = useSearchParams()
  const { hasPermission } = useAuth()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const profitGapOnly = searchParams.get('view') === 'profit-gaps'
  const [status, setStatus] = useState<OrderStatus | undefined>()
  const [keyword, setKeyword] = useState('')
  const [editingOrder, setEditingOrder] = useState<Order | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [form] = Form.useForm<{ paidAmount?: number; paymentChannelCost?: number; grossProfitAmount?: number; remark?: string }>()
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])
  const canEditProfitFacts = hasPermission('ledger:view')
  const effectiveStatus = profitGapOnly ? 'paid' : status

  const setViewParam = (nextView: 'all' | 'profit-gaps') => {
    const next = new URLSearchParams(searchParams)
    if (nextView === 'profit-gaps') {
      next.set('view', 'profit-gaps')
    } else {
      next.delete('view')
    }
    setSearchParams(next, { replace: true })
  }

  const query = useQuery({
    queryKey: ['orders', scopeParams.tenantID, scopeParams.brandID, effectiveStatus],
    queryFn: () => apiClient.listOrders({ ...scopeParams, status: effectiveStatus }),
  })

  const profitGapOrders = useMemo(() => {
    const items = query.data?.items ?? []
    return items.filter((item) => item.status === 'paid' && (item.paymentChannelCost === null || item.paymentChannelCost === undefined || item.grossProfitAmount === null || item.grossProfitAmount === undefined))
  }, [query.data?.items])

  const updateProfitFactsMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: { paidAmount?: number; paymentChannelCost?: number; grossProfitAmount?: number; remark?: string } }) =>
      apiClient.updateOrderProfitFacts(id, payload),
    onSuccess: () => {
      message.success(t('orders.updateProfitFactsSuccess'))
      setModalOpen(false)
      setEditingOrder(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['orders'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      queryClient.invalidateQueries({ queryKey: ['report-data-platform-metrics'] })
    },
  })

  const data = useMemo(() => {
    const items = profitGapOnly ? profitGapOrders : (query.data?.items ?? [])
    if (!keyword.trim()) {
      return items
    }
    const normalized = keyword.trim().toLowerCase()
    return items.filter((item) =>
      [item.orderNo, item.playerID, item.agentID, item.currency, item.freezeReason, item.riskNote, item.reviewStatus]
        .filter((value) => value !== undefined && value !== null)
        .some((value) => String(value).toLowerCase().includes(normalized)),
    )
  }, [keyword, profitGapOnly, profitGapOrders, query.data?.items])

  const riskSummary = useMemo(() => ({
    visible: data.filter((item) => item.freezeVisible !== undefined || item.riskNote || item.freezeReason || item.reviewStatus).filter((item) => getOrderRiskVisibility(item) === 'visible').length,
    hidden: data.filter((item) => item.freezeVisible !== undefined || item.riskNote || item.freezeReason || item.reviewStatus).filter((item) => getOrderRiskVisibility(item) === 'hidden').length,
    reviewing: data.filter((item) => getOrderReviewStatus(item) === 'reviewing').length,
    paid: data.filter((item) => item.status === 'paid').length,
  }), [data])

  const profitGapSummary = useMemo(() => ({
    total: profitGapOrders.length,
    missingChannelCost: profitGapOrders.filter((item) => item.paymentChannelCost === null || item.paymentChannelCost === undefined).length,
    missingGrossProfit: profitGapOrders.filter((item) => item.grossProfitAmount === null || item.grossProfitAmount === undefined).length,
  }), [profitGapOrders])

  const columns: ColumnsType<Order> = [
    { title: t('orders.columns.order'), dataIndex: 'orderNo' },
    { title: t('orders.columns.playerId'), dataIndex: 'playerID' },
    { title: t('orders.columns.agentId'), dataIndex: 'agentID', render: (value) => value ?? t('common.none') },
    { title: t('orders.columns.gameId'), dataIndex: 'gameID', render: (value) => value ?? t('common.none') },
    { title: t('orders.columns.amount'), render: (_, record) => `${record.amount} ${record.currency}` },
    { title: t('orders.columns.paidAmount'), render: (_, record) => `${record.paidAmount ?? record.amount} ${record.currency}` },
    { title: t('orders.columns.paymentChannelCost'), render: (_, record) => record.paymentChannelCost ?? t('common.none') },
    { title: t('orders.columns.grossProfitAmount'), render: (_, record) => record.grossProfitAmount ?? t('common.none') },
    { title: t('orders.columns.channel'), dataIndex: 'channel' },
    { title: t('orders.columns.status'), dataIndex: 'status', render: (value: OrderStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag> },
    {
      title: t('orders.columns.riskVisibility'),
      key: 'riskVisibility',
      render: (_, record) => {
        const hasRiskSignals = record.freezeVisible !== undefined || record.riskNote || record.freezeReason || record.reviewStatus
        if (!hasRiskSignals) {
          return <Typography.Text type="secondary">{t('common.none')}</Typography.Text>
        }
        const visibility = getOrderRiskVisibility(record)
        return <Tag color={riskVisibilityColorMap[visibility]}>{t(`orders.riskVisibility.${visibility}`)}</Tag>
      },
    },
    {
      title: t('orders.columns.reviewStatus'),
      key: 'reviewStatus',
      render: (_, record) => {
        const reviewStatus = getOrderReviewStatus(record)
        if (!reviewStatus) {
          return <Typography.Text type="secondary">{t('common.none')}</Typography.Text>
        }
        return <Tag color={reviewStatusColorMap[reviewStatus]}>{t(`risk.caseStatus.${reviewStatus}`)}</Tag>
      },
    },
    { title: t('orders.columns.riskNote'), key: 'riskNote', render: (_, record) => record.freezeReason || record.riskNote || t('common.none') },
    { title: t('orders.columns.createdAt'), dataIndex: 'createdAt' },
    {
      title: t('orders.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        canEditProfitFacts && record.status === 'paid'
          ? (
            <Button
              type="link"
              icon={<EditOutlined />}
              onClick={() => {
                setEditingOrder(record)
                setModalOpen(true)
                form.setFieldsValue({
                  paidAmount: record.paidAmount ?? record.amount,
                  paymentChannelCost: record.paymentChannelCost ?? undefined,
                  grossProfitAmount: record.grossProfitAmount ?? undefined,
                  remark: '',
                })
              }}
            >
              {t('orders.updateProfitFacts')}
            </Button>
            )
          : <Typography.Text type="secondary">{t('common.none')}</Typography.Text>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-hero-card" bordered={false}>
        <div className="app-section-stack">
            <div className="app-page__heading">
              <Typography.Text className="app-hero-kicker">{t('orders.title')}</Typography.Text>
              <Typography.Title level={3} className="app-page__title">{t('orders.tableTitle')}</Typography.Title>
              <Typography.Paragraph className="app-page__subtitle">{t('orders.subtitle')}</Typography.Paragraph>
            </div>

            <ScopeNotice />

            <Alert
              type="info"
              showIcon
              message={t('orders.riskBanner')}
              description={t('orders.riskBannerDescription', {
                visible: riskSummary.visible,
                hidden: riskSummary.hidden,
                reviewing: riskSummary.reviewing,
              })}
            />

            {profitGapSummary.total > 0 ? (
              <Alert
                type={profitGapOnly ? 'warning' : 'info'}
                showIcon
                message={t('orders.profitGapBanner', { count: profitGapSummary.total })}
                description={t('orders.profitGapBannerDescription', {
                  missingChannelCost: profitGapSummary.missingChannelCost,
                  missingGrossProfit: profitGapSummary.missingGrossProfit,
                })}
                action={(
                  <Space wrap>
                    <Button size="small" type={profitGapOnly ? 'default' : 'primary'} onClick={() => setViewParam(profitGapOnly ? 'all' : 'profit-gaps')}>
                      {profitGapOnly ? t('orders.viewAll') : t('orders.viewProfitGaps')}
                    </Button>
                    {profitGapOnly && canEditProfitFacts && profitGapOrders[0] ? (
                      <Button
                        size="small"
                        onClick={() => {
                          const record = profitGapOrders[0]
                          setEditingOrder(record)
                          setModalOpen(true)
                          form.setFieldsValue({
                            paidAmount: record.paidAmount ?? record.amount,
                            paymentChannelCost: record.paymentChannelCost ?? undefined,
                            grossProfitAmount: record.grossProfitAmount ?? undefined,
                            remark: '',
                          })
                        }}
                      >
                        {t('orders.fixNextProfitGap')}
                      </Button>
                    ) : null}
                  </Space>
                )}
              />
            ) : null}

            <div className="app-toolbar">
              <div className="app-toolbar__filters">
                <Input.Search
                  allowClear
                  placeholder={t('orders.searchPlaceholder')}
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  onSearch={(value) => setKeyword(value)}
                  style={{ width: 320 }}
                />
                <Select<OrderStatus | undefined>
                  allowClear
                  placeholder={profitGapOnly ? t('orders.statusLockedPlaceholder') : t('orders.statusPlaceholder')}
                  value={effectiveStatus}
                  onChange={(value) => setStatus(value)}
                  disabled={profitGapOnly}
                  style={{ width: 180 }}
                  options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
                />
              </div>
              <div className="app-toolbar__actions">
                <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
              </div>
            </div>
            <div className="app-summary-strip">
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('orders.columns.reviewStatus')}</span>
                <span className="app-summary-pill__value">{riskSummary.reviewing}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('orders.riskVisibility.visible')}</span>
                <span className="app-summary-pill__value">{riskSummary.visible}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('orders.riskVisibility.hidden')}</span>
                <span className="app-summary-pill__value">{riskSummary.hidden}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('orders.columns.status')}</span>
                <span className="app-summary-pill__value">{riskSummary.paid}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('orders.profitGapCount')}</span>
                <span className="app-summary-pill__value">{profitGapSummary.total}</span>
              </div>
            </div>

            <div className="app-mini-grid">
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('orders.columns.order')}</span>
                <span className="app-mini-stat__value">{data.length}</span>
              </div>
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('orders.matchingOrders', { count: data.length })}</span>
                <span className="app-mini-stat__value">{data.length}</span>
              </div>
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('orders.columns.channel')}</span>
                <span className="app-mini-stat__value">{new Set(data.map((item) => item.channel).filter(Boolean)).size}</span>
              </div>
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('orders.columns.playerId')}</span>
                <span className="app-mini-stat__value">{new Set(data.map((item) => item.playerID).filter(Boolean)).size}</span>
              </div>
            </div>
          </div>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('orders.loadError')} description={(query.error as Error).message} /> : null}
      {updateProfitFactsMutation.error ? <Alert type="error" showIcon message={t('orders.updateProfitFactsError')} description={(updateProfitFactsMutation.error as Error).message} /> : null}

      <Card className="app-workspace-card" title={t('orders.tableTitle')} extra={<Typography.Text type="secondary" className="app-compact-note">{t('orders.matchingOrders', { count: data.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Order>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('orders.empty')} /> }}
            scroll={{ x: 1400 }}
          />
        </div>
      </Card>

      <Modal
        title={t('orders.updateProfitFactsModalTitle')}
        open={modalOpen}
        onCancel={() => {
          setModalOpen(false)
          setEditingOrder(null)
        }}
        onOk={async () => {
          if (!editingOrder) {
            return
          }
          const values = await form.validateFields()
          updateProfitFactsMutation.mutate({ id: editingOrder.id, payload: values })
        }}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
        confirmLoading={updateProfitFactsMutation.isPending}
        destroyOnHidden
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {editingOrder ? (
            <Typography.Text type="secondary">
              {t('orders.updateProfitFactsHint', { orderNo: editingOrder.orderNo })}
            </Typography.Text>
          ) : null}
          <Form form={form} layout="vertical">
            <Form.Item name="paidAmount" label={t('orders.columns.paidAmount')} rules={[{ required: true, message: t('orders.form.paidAmountRequired') }]}>
              <InputNumber min={0.01} precision={2} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="paymentChannelCost" label={t('orders.columns.paymentChannelCost')}>
              <InputNumber min={0} precision={2} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="grossProfitAmount" label={t('orders.columns.grossProfitAmount')}>
              <InputNumber min={0} precision={2} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="remark" label={t('orders.form.remark')}>
              <Input.TextArea rows={3} placeholder={t('orders.form.remarkPlaceholder')} />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </Space>
  )
}
