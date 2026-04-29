import { CheckOutlined, DownloadOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, Descriptions, Empty, Form, Input, Modal, Select, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../auth'
import { useI18n } from '../../i18n'
import { ConfirmSettlementBillPayload, CreateRecalculationTaskPayload, SettlementBill, SettlementBillStatus, SettlementPeriodType, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: SettlementBillStatus[] = ['pending', 'generated', 'confirmed', 'cancelled']
const periodTypeOrder: SettlementPeriodType[] = ['daily', 'weekly', 'monthly']

const normalizePeriodType = (value?: string | null): SettlementPeriodType | undefined => {
  if (value === 'daily' || value === 'weekly' || value === 'monthly') {
    return value
  }
  return undefined
}

const statusColorMap: Record<SettlementBillStatus, string> = {
  pending: 'default',
  generated: 'processing',
  confirmed: 'success',
  cancelled: 'error',
}

export default function SettlementBillsPage() {
  const { t } = useI18n()
  const navigate = useNavigate()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const [confirmForm] = Form.useForm<ConfirmSettlementBillPayload>()
  const [status, setStatus] = useState<SettlementBillStatus | undefined>()
  const [keyword, setKeyword] = useState('')
  const [confirmingRecord, setConfirmingRecord] = useState<SettlementBill | null>(null)
  const canExport = hasPermission('settlement-bills:export')
  const canConfirm = hasPermission('settlement-bills:confirm')
  const canCreateRecalculationTask = hasPermission('recalculation-tasks:create')
  const billNo = keyword.trim() || undefined
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['settlement-bills', scopeParams.tenantID, scopeParams.brandID, status, billNo],
    queryFn: () => apiClient.listSettlementBills({ ...scopeParams, status, billNo }),
  })

  const exportMutation = useMutation({
    mutationFn: (id: number) => apiClient.exportSettlementBill(id),
    onSuccess: () => {
      message.success(t('settlementBills.exportSuccess'))
    },
  })

  const confirmMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: ConfirmSettlementBillPayload }) => apiClient.confirmSettlementBill(id, payload),
    onSuccess: () => {
      message.success(t('settlementBills.confirmSuccess'))
      setConfirmingRecord(null)
      confirmForm.resetFields()
      void query.refetch()
    },
  })

  const openCreateRecalculationTask = (record: SettlementBill) => {
    const payload: CreateRecalculationTaskPayload = {
      taskType: 'settlement_bill',
      scope: 'settlement_bill',
      settlementBillID: record.id,
      agentID: record.agentID,
      remark: `${record.billNo}`,
    }
    navigate('/recalculation-tasks', { state: { createTaskDraft: payload } })
  }

  const openConfirmModal = (record: SettlementBill) => {
    setConfirmingRecord(record)
    confirmForm.setFieldsValue({ remark: record.remark ?? '' })
  }

  const closeConfirmModal = () => {
    if (confirmMutation.isPending) {
      return
    }
    setConfirmingRecord(null)
    confirmForm.resetFields()
  }

  const handleConfirm = async () => {
    if (!confirmingRecord) {
      return
    }
    const values = await confirmForm.validateFields()
    await confirmMutation.mutateAsync({
      id: confirmingRecord.id,
      payload: { remark: values.remark?.trim() || undefined },
    })
  }

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])
  const visiblePeriodTypes = useMemo(() => {
    const types = Array.from(new Set(data.map((item) => normalizePeriodType(item.periodType)).filter(Boolean))) as SettlementPeriodType[]
    return periodTypeOrder.filter((type) => types.includes(type))
  }, [data])

  const formatPeriodType = (periodType?: string | null) => {
    const normalized = normalizePeriodType(periodType)
    return normalized ? t(`settlementBills.periodType.${normalized}`) : t('settlementBills.periodType.unknown')
  }

  const columns: ColumnsType<SettlementBill> = [
    { title: t('settlementBills.columns.billNo'), dataIndex: 'billNo', width: 180 },
    {
      title: t('settlementBills.columns.agent'),
      key: 'agent',
      width: 140,
      render: (_, record) => record.agentName || `#${record.agentID}`,
    },
    {
      title: t('settlementBills.columns.period'),
      key: 'period',
      width: 220,
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text>{`${record.periodStart} ~ ${record.periodEnd}`}</Typography.Text>
          <Typography.Text type="secondary">{formatPeriodType(record.periodType)}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('settlementBills.columns.commissionAmount'),
      key: 'commissionAmount',
      width: 150,
      render: (_, record) => `${record.commissionAmount} ${record.currency}`,
    },
    {
      title: t('settlementBills.columns.adjustmentAmount'),
      key: 'adjustmentAmount',
      width: 150,
      render: (_, record) => `${record.adjustmentAmount} ${record.currency}`,
    },
    {
      title: t('settlementBills.columns.payableAmount'),
      key: 'payableAmount',
      width: 150,
      render: (_, record) => `${record.payableAmount} ${record.currency}`,
    },
    {
      title: t('settlementBills.columns.recordCount'),
      key: 'recordCount',
      width: 120,
      render: (_, record) => record.summaryPayload?.recordCount ?? record.details?.length ?? 0,
    },
    {
      title: t('settlementBills.columns.status'),
      dataIndex: 'status',
      width: 120,
      render: (value: SettlementBillStatus) => <Tag color={statusColorMap[value]}>{t(`settlementBills.status.${value}`)}</Tag>,
    },
    {
      title: t('settlementBills.columns.freezeVisibility'),
      key: 'freezeVisibility',
      width: 150,
      render: (_, record) => (
        <Tag color={(record.freezeVisible ?? record.status !== 'confirmed') ? 'warning' : 'success'}>
          {(record.freezeVisible ?? record.status !== 'confirmed') ? t('settlementBills.freezeState.visible') : t('settlementBills.freezeState.released')}
        </Tag>
      ),
    },
    { title: t('settlementBills.columns.generatedAt'), dataIndex: 'generatedAt', width: 180, render: (value) => value || t('common.none') },
    { title: t('settlementBills.columns.confirmedAt'), dataIndex: 'confirmedAt', width: 180, render: (value) => value || t('common.none') },
    {
      title: t('settlementBills.columns.actions'),
      key: 'actions',
      fixed: 'right',
      width: 320,
      render: (_, record) => (
        <Space size="small" wrap>
          {canConfirm ? (
            <Button
              size="small"
              type="link"
              icon={<CheckOutlined />}
              disabled={record.status !== 'generated'}
              onClick={() => openConfirmModal(record)}
            >
              {t('settlementBills.confirm')}
            </Button>
          ) : null}
          {canCreateRecalculationTask ? (
            <Button
              size="small"
              type="link"
              icon={<PlusOutlined />}
              onClick={() => openCreateRecalculationTask(record)}
            >
              {t('settlementBills.createRecalculationTask')}
            </Button>
          ) : null}
          {canExport ? (
            <Button
              size="small"
              type="link"
              icon={<DownloadOutlined />}
              loading={exportMutation.isPending}
              onClick={() => exportMutation.mutate(record.id)}
            >
              {t('settlementBills.export')}
            </Button>
          ) : null}
        </Space>
      ),
    },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('settlementBills.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('settlementBills.subtitle')}
            </Typography.Paragraph>
          </div>
          <ScopeNotice />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('settlementBills.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<SettlementBillStatus | undefined>
                allowClear
                placeholder={t('settlementBills.statusPlaceholder')}
                value={status}
                onChange={(value) => setStatus(value)}
                style={{ width: 180 }}
                options={statusOptions.map((item) => ({ label: t(`settlementBills.status.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('settlementBills.loadError')} description={(query.error as Error).message} /> : null}
      {exportMutation.error ? <Alert type="error" showIcon message={t('settlementBills.exportError')} description={(exportMutation.error as Error).message} /> : null}
      {confirmMutation.error ? <Alert type="error" showIcon message={t('settlementBills.confirmError')} description={(confirmMutation.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('settlementBills.tableTitle')} extra={<Typography.Text type="secondary">{t('settlementBills.results', { count: data.length })}</Typography.Text>}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {visiblePeriodTypes.length ? (
            <Alert
              type="info"
              showIcon
              message={t('settlementBills.periodType.summary')}
              description={visiblePeriodTypes.map((type) => formatPeriodType(type)).join(' / ')}
            />
          ) : null}
          <Descriptions size="small" column={{ xs: 1, sm: 2, md: 4 }}>
            <Descriptions.Item label={t('settlementBills.summary.totalBills')}>{data.length}</Descriptions.Item>
            <Descriptions.Item label={t('settlementBills.summary.confirmedBills')}>{data.filter((item) => item.status === 'confirmed').length}</Descriptions.Item>
            <Descriptions.Item label={t('settlementBills.summary.totalPayable')}>
              {data.reduce((sum, item) => sum + Number(item.payableAmount || 0), 0).toFixed(2)}
            </Descriptions.Item>
            <Descriptions.Item label={t('settlementBills.summary.freezeVisible')}>
              {data.filter((item) => item.freezeVisible ?? item.status !== 'confirmed').length}
            </Descriptions.Item>
          </Descriptions>
          <div className="app-table app-table--compact">
            <Table<SettlementBill>
              rowKey="id"
              loading={query.isLoading || query.isFetching}
              dataSource={data}
              columns={columns}
              pagination={{ pageSize: 10, showSizeChanger: false }}
              scroll={{ x: 1600 }}
              expandable={{
                expandedRowRender: (record) => (
                  <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                    <Descriptions size="small" column={{ xs: 1, sm: 2, md: 3 }}>
                      <Descriptions.Item label={t('settlementBills.detail.confirmedBy')}>{record.confirmedBy || t('common.none')}</Descriptions.Item>
                      <Descriptions.Item label={t('settlementBills.detail.remark')}>{record.remark || t('common.none')}</Descriptions.Item>
                      <Descriptions.Item label={t('settlementBills.detail.detailCount')}>{record.details?.length ?? 0}</Descriptions.Item>
                      <Descriptions.Item label={t('settlementBills.columns.freezeVisibility')}>
                        {(record.freezeVisible ?? record.status !== 'confirmed') ? t('settlementBills.freezeState.visible') : t('settlementBills.freezeState.released')}
                      </Descriptions.Item>
                    </Descriptions>
                    <Table<NonNullable<SettlementBill['details']>[number]>
                      rowKey="id"
                      size="small"
                      pagination={false}
                      dataSource={record.details ?? []}
                      locale={{ emptyText: <Empty description={t('settlementBills.detailsEmpty')} /> }}
                      columns={[
                        { title: t('settlementBills.detailColumns.referenceType'), dataIndex: 'referenceType' },
                        { title: t('settlementBills.detailColumns.referenceId'), dataIndex: 'referenceID' },
                        { title: t('settlementBills.detailColumns.recordNo'), dataIndex: 'recordNo', render: (value) => value || t('common.none') },
                        { title: t('settlementBills.detailColumns.orderNo'), dataIndex: 'orderNo', render: (value) => value || t('common.none') },
                        { title: t('settlementBills.detailColumns.amount'), render: (_, detail) => `${detail.amount} ${detail.currency}` },
                        { title: t('settlementBills.detailColumns.occurredAt'), dataIndex: 'occurredAt' },
                        { title: t('settlementBills.detailColumns.remark'), dataIndex: 'remark', render: (value) => value || t('common.none') },
                      ]}
                    />
                  </Space>
                ),
                rowExpandable: (record) => Boolean(record.details?.length),
              }}
              locale={{ emptyText: <Empty description={t('settlementBills.empty')} /> }}
            />
          </div>
        </Space>
      </Card>

      <Modal
        title={t('settlementBills.confirmModal.title')}
        open={Boolean(confirmingRecord)}
        onOk={() => void handleConfirm()}
        onCancel={closeConfirmModal}
        okText={t('settlementBills.confirmModal.confirm')}
        cancelText={t('common.cancel')}
        confirmLoading={confirmMutation.isPending}
        destroyOnClose
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Alert type="warning" showIcon message={t('settlementBills.confirmModal.alert')} />
          <Descriptions size="small" column={1} bordered>
            <Descriptions.Item label={t('settlementBills.columns.billNo')}>{confirmingRecord?.billNo ?? t('common.none')}</Descriptions.Item>
            <Descriptions.Item label={t('settlementBills.columns.agent')}>
              {confirmingRecord ? confirmingRecord.agentName || `#${confirmingRecord.agentID}` : t('common.none')}
            </Descriptions.Item>
            <Descriptions.Item label={t('settlementBills.columns.period')}>
              {confirmingRecord ? `${confirmingRecord.periodStart} ~ ${confirmingRecord.periodEnd}` : t('common.none')}
            </Descriptions.Item>
            <Descriptions.Item label={t('settlementBills.columns.payableAmount')}>
              {confirmingRecord ? `${confirmingRecord.payableAmount} ${confirmingRecord.currency}` : t('common.none')}
            </Descriptions.Item>
          </Descriptions>
          <Typography.Text type="secondary">{t('settlementBills.confirmModal.description')}</Typography.Text>
          <Form form={confirmForm} layout="vertical" preserve={false}>
            <Form.Item label={t('settlementBills.confirmModal.remark')} name="remark">
              <Input.TextArea rows={4} maxLength={200} placeholder={t('settlementBills.confirmModal.remarkPlaceholder')} />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </Space>
  )
}
