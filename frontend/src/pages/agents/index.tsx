import { ReloadOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, Divider, Empty, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useAuth } from '../../auth'
import { useI18n } from '../../i18n'
import { Agent, AgentStatus, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: AgentStatus[] = ['pending', 'active', 'frozen', 'disabled', 'closed']

const statusColorMap: Record<AgentStatus, string> = {
  pending: 'processing',
  active: 'success',
  frozen: 'warning',
  disabled: 'default',
  closed: 'error',
}

type AgentFormValues = {
  agentNo: string
  name: string
  displayName?: string
  phone?: string
  email?: string
  status: AgentStatus
  level: number
  remark?: string
}

export default function AgentsPage() {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<AgentStatus | undefined>()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [form] = Form.useForm<AgentFormValues>()
  const canCreate = hasPermission('agents:create')
  const canUpdateStatus = hasPermission('agents:status:update')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['agents', scopeParams.tenantID, scopeParams.brandID, keyword, status],
    queryFn: () => apiClient.listAgents({ ...scopeParams, keyword: keyword || undefined, status }),
  })

  const createMutation = useMutation({
    mutationFn: apiClient.createAgent,
    onSuccess: () => {
      message.success(t('agents.createSuccess'))
      setIsCreateOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['agents'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, nextStatus }: { id: number; nextStatus: AgentStatus }) => apiClient.updateAgentStatus(id, nextStatus),
    onSuccess: () => {
      message.success(t('agents.statusSuccess'))
      queryClient.invalidateQueries({ queryKey: ['agents'] })
    },
  })

  const data = useMemo(() => {
    const items = query.data?.items ?? []
    if (!keyword.trim()) {
      return items
    }
    const normalized = keyword.trim().toLowerCase()
    return items.filter((item) =>
      [item.name, item.agentNo, item.displayName, item.phone, item.email]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(normalized)),
    )
  }, [keyword, query.data?.items])

  const summary = useMemo(() => ({
    total: data.length,
    active: data.filter((item) => item.status === 'active').length,
    pending: data.filter((item) => item.status === 'pending').length,
    frozen: data.filter((item) => item.status === 'frozen').length,
    highestLevel: data.length > 0 ? Math.max(...data.map((item) => item.level ?? 0)) : 0,
  }), [data])

  const columns: ColumnsType<Agent> = [
    {
      title: t('agents.columns.agent'),
      key: 'agent',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text type="secondary">{record.agentNo}</Typography.Text>
        </Space>
      ),
    },
    { title: t('agents.columns.displayName'), dataIndex: 'displayName', render: (value) => value || t('common.none') },
    { title: t('agents.columns.parentAgentId'), dataIndex: 'parentAgentID', render: (value) => value ?? t('common.none') },
    { title: t('agents.columns.level'), dataIndex: 'level' },
    { title: t('agents.columns.status'), dataIndex: 'status', render: (value: AgentStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag> },
    { title: t('agents.columns.phone'), dataIndex: 'phone', render: (value) => value || t('common.none') },
    { title: t('agents.columns.currency'), dataIndex: 'currency' },
    { title: t('agents.columns.createdAt'), dataIndex: 'createdAt' },
    {
      title: t('agents.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Select<AgentStatus>
          value={record.status}
          style={{ width: 140 }}
          disabled={!canUpdateStatus}
          options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
          loading={statusMutation.isPending}
          onChange={(nextStatus) => {
            if (nextStatus !== record.status) {
              statusMutation.mutate({ id: record.id, nextStatus })
            }
          }}
        />
      ),
    },
  ]

  const handleCreate = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('common.scopeRequiredHint'))
      return
    }
    const values = await form.validateFields()
    createMutation.mutate({
      ...scopeParams,
      agentNo: values.agentNo,
      name: values.name,
      displayName: values.displayName,
      phone: values.phone,
      email: values.email,
      status: values.status,
      level: values.level,
      remark: values.remark,
    })
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-hero-card" bordered={false}>
        <div className="app-section-stack">
            <div className="app-page__heading">
              <Typography.Text className="app-hero-kicker">{t('agents.title')}</Typography.Text>
              <Typography.Title level={3} className="app-page__title">{t('agents.tableTitle')}</Typography.Title>
              <Typography.Paragraph className="app-page__subtitle">{t('agents.subtitle')}</Typography.Paragraph>
            </div>

            <ScopeNotice requireBrand />

            <div className="app-toolbar">
              <div className="app-toolbar__filters">
                <Input.Search
                  allowClear
                  placeholder={t('agents.searchPlaceholder')}
                  value={keyword}
                  onChange={(event) => setKeyword(event.target.value)}
                  onSearch={(value) => setKeyword(value)}
                  style={{ width: 320 }}
                />
                <Select<AgentStatus | undefined>
                  allowClear
                  placeholder={t('agents.statusPlaceholder')}
                  value={status}
                  onChange={(value) => setStatus(value)}
                  style={{ width: 180 }}
                  options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
                />
              </div>
              <div className="app-toolbar__actions">
                {canCreate ? <Button type="primary" onClick={() => setIsCreateOpen(true)}>{t('agents.create')}</Button> : null}
                <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
              </div>
            </div>
            <div className="app-summary-strip">
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('agents.visible')}</span>
                <span className="app-summary-pill__value">{summary.total}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('agents.active')}</span>
                <span className="app-summary-pill__value">{summary.active}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('agents.pending')}</span>
                <span className="app-summary-pill__value">{summary.pending}</span>
              </div>
              <div className="app-summary-pill">
                <span className="app-summary-pill__label">{t('agents.frozen')}</span>
                <span className="app-summary-pill__value">{summary.frozen}</span>
              </div>
            </div>

            <div className="app-mini-grid">
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('agents.columns.level')}</span>
                <span className="app-mini-stat__value">{summary.highestLevel}</span>
              </div>
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('agents.columns.actions')}</span>
                <span className="app-mini-stat__value">{canUpdateStatus ? 1 : 0}</span>
              </div>
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('agents.create')}</span>
                <span className="app-mini-stat__value">{canCreate ? 1 : 0}</span>
              </div>
              <div className="app-mini-stat">
                <span className="app-mini-stat__label">{t('agents.results', { count: data.length })}</span>
                <span className="app-mini-stat__value">{data.length}</span>
              </div>
            </div>
          </div>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('agents.loadError')} description={(query.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('agents.createError')} description={(createMutation.error as Error).message} /> : null}
      {statusMutation.error ? <Alert type="error" showIcon message={t('agents.statusError')} description={(statusMutation.error as Error).message} /> : null}

      <Card className="app-workspace-card" title={t('agents.tableTitle')} extra={<Typography.Text type="secondary" className="app-compact-note">{t('agents.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Agent>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('agents.empty')} /> }}
          />
        </div>
      </Card>

      <Modal
        className="app-modal-form"
        title={t('agents.modalTitle')}
        open={canCreate && isCreateOpen}
        onCancel={() => setIsCreateOpen(false)}
        onOk={handleCreate}
        okText={t('common.ok')}
        cancelText={t('common.cancel')}
        confirmLoading={createMutation.isPending}
        destroyOnClose
      >
        <Divider />
        <Form<AgentFormValues> className="app-form" form={form} layout="vertical" initialValues={{ status: 'pending', level: 1 }}>
          <Form.Item label={t('agents.form.agentNo')} name="agentNo" rules={[{ required: true, message: t('agents.form.agentNoRequired') }]}>
            <Input placeholder={t('agents.form.agentNoPlaceholder')} />
          </Form.Item>
          <Form.Item label={t('agents.form.name')} name="name" rules={[{ required: true, message: t('agents.form.nameRequired') }]}>
            <Input placeholder={t('agents.form.namePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('agents.form.displayName')} name="displayName">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('agents.form.phone')} name="phone">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('agents.form.email')} name="email">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('agents.form.status')} name="status" rules={[{ required: true, message: t('agents.form.statusRequired') }]}>
            <Select options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
          </Form.Item>
          <Form.Item label={t('agents.form.level')} name="level" rules={[{ required: true, message: t('agents.form.levelRequired') }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('agents.form.remark')} name="remark">
            <Input.TextArea rows={3} placeholder={t('common.optional')} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
