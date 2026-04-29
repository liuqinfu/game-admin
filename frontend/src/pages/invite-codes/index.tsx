import { ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useAuth } from '../../auth'
import { AgentSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { InviteCode, InviteCodeStatus, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: InviteCodeStatus[] = ['active', 'disabled', 'expired']

const statusColorMap: Record<InviteCodeStatus, string> = {
  active: 'success',
  disabled: 'warning',
  expired: 'default',
}

type InviteCodeFormValues = {
  agentID: number
  code: string
  status: InviteCodeStatus
  channel?: string
  remark?: string
  maxUseCount: number
  isPrimary: boolean
}

export default function InviteCodesPage() {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<InviteCodeStatus | undefined>()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [form] = Form.useForm<InviteCodeFormValues>()
  const canCreate = hasPermission('invite-codes:create')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['invite-codes', scopeParams.tenantID, scopeParams.brandID, status],
    queryFn: () => apiClient.listInviteCodes({ ...scopeParams, status }),
  })

  const createMutation = useMutation({
    mutationFn: apiClient.createInviteCode,
    onSuccess: () => {
      message.success(t('inviteCodes.createSuccess'))
      setIsCreateOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['invite-codes'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })

  const data = useMemo(() => {
    const items = query.data?.items ?? []
    if (!keyword.trim()) {
      return items
    }

    const normalized = keyword.trim().toLowerCase()
    return items.filter((item) =>
      [item.code, item.channel, item.remark]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(normalized)),
    )
  }, [keyword, query.data?.items])

  const columns: ColumnsType<InviteCode> = [
    {
      title: t('inviteCodes.columns.code'),
      dataIndex: 'code',
      render: (value: string, record) => (
        <Space>
          <Typography.Text strong>{value}</Typography.Text>
          {record.isPrimary ? <Tag color="blue">{t('inviteCodes.columns.primary')}</Tag> : null}
        </Space>
      ),
    },
    { title: t('inviteCodes.columns.agentId'), dataIndex: 'agentID' },
    { title: t('inviteCodes.columns.channel'), dataIndex: 'channel', render: (value) => value || t('common.none') },
    {
      title: t('inviteCodes.columns.status'),
      dataIndex: 'status',
      render: (value: InviteCodeStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    { title: t('inviteCodes.columns.used'), render: (_, record) => `${record.usedCount}/${record.maxUseCount || '∞'}` },
    { title: t('inviteCodes.columns.expiredAt'), dataIndex: 'expiredAt', render: (value) => value || t('common.none') },
    { title: t('inviteCodes.columns.createdAt'), dataIndex: 'createdAt' },
  ]

  const handleCreate = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('common.scopeRequiredHint'))
      return
    }
    const values = await form.validateFields()
    createMutation.mutate({
      agentID: values.agentID,
      code: values.code,
      status: values.status,
      channel: values.channel,
      remark: values.remark,
      maxUseCount: values.maxUseCount,
      isPrimary: values.isPrimary,
    })
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card" bordered={false}>
        <Space direction="vertical" size="middle" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('inviteCodes.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('inviteCodes.subtitle')}
            </Typography.Paragraph>
          </div>
          <ScopeNotice requireBrand />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('inviteCodes.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<InviteCodeStatus | undefined>
                allowClear
                placeholder={t('inviteCodes.statusPlaceholder')}
                value={status}
                onChange={(value) => setStatus(value)}
                style={{ width: 160 }}
                options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              {canCreate ? (
                <Button type="primary" onClick={() => setIsCreateOpen(true)}>
                  {t('inviteCodes.create')}
                </Button>
              ) : null}
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('inviteCodes.loadError')} description={(query.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('inviteCodes.createError')} description={(createMutation.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('inviteCodes.tableTitle')} extra={<Typography.Text type="secondary">{t('inviteCodes.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table">
          <Table<InviteCode>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('inviteCodes.empty')} /> }}
          />
        </div>
      </Card>

      <Modal
        className="app-modal-form"
        title={t('inviteCodes.modalTitle')}
        open={canCreate && isCreateOpen}
        onCancel={() => setIsCreateOpen(false)}
        onOk={handleCreate}
        okText={t('common.ok')}
        cancelText={t('common.cancel')}
        confirmLoading={createMutation.isPending}
        destroyOnClose
      >
        <Form<InviteCodeFormValues>
          className="app-form"
          form={form}
          layout="vertical"
          initialValues={{ status: 'active', maxUseCount: 1, isPrimary: false }}
        >
          <Form.Item label={t('inviteCodes.form.agentId')} name="agentID" rules={[{ required: true, message: t('inviteCodes.form.agentIdRequired') }]}>
            <AgentSelect tenantID={scopeParams.tenantID} brandID={scopeParams.brandID} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('inviteCodes.form.code')} name="code" rules={[{ required: true, message: t('inviteCodes.form.codeRequired') }]}>
            <Input placeholder={t('inviteCodes.form.code')} />
          </Form.Item>
          <Form.Item label={t('inviteCodes.form.status')} name="status" rules={[{ required: true, message: t('inviteCodes.form.statusRequired') }]}>
            <Select options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
          </Form.Item>
          <Form.Item label={t('inviteCodes.form.channel')} name="channel">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('inviteCodes.form.maxUseCount')} name="maxUseCount" rules={[{ required: true, message: t('inviteCodes.form.maxUseCountRequired') }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('inviteCodes.form.isPrimary')} name="isPrimary" rules={[{ required: true, message: t('inviteCodes.form.isPrimaryRequired') }]}>
            <Select options={[{ label: t('common.yes'), value: true }, { label: t('common.no'), value: false }]} />
          </Form.Item>
          <Form.Item label={t('inviteCodes.form.remark')} name="remark">
            <Input.TextArea rows={3} placeholder={t('common.optional')} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
