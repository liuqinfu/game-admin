import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Form,
  Input,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd'
import { useMemo } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { AgentSelect, GameSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { AccessStatus, AgentGameAccessListItem, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: AccessStatus[] = ['enabled', 'disabled']

const statusColorMap: Record<AccessStatus, string> = {
  enabled: 'success',
  disabled: 'default',
}

type AccessFormValues = {
  agentID: number
  gameID: number
  status: AccessStatus
  remark?: string
}

export default function AgentGameAccessPage() {
  const { t } = useI18n()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [form] = Form.useForm<AccessFormValues>()
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['agent-game-access', scopeParams.tenantID, scopeParams.brandID],
    queryFn: () => apiClient.listAgentGameAccess(scopeParams),
  })

  const upsertMutation = useMutation({
    mutationFn: apiClient.upsertAgentGameAccess,
    onSuccess: () => {
      message.success(t('agentGameAccess.saveSuccess'))
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['agent-game-access'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: ({ agentID, gameID }: { agentID: number; gameID: number }) => apiClient.deleteAgentGameAccess(agentID, gameID),
    onSuccess: () => {
      message.success(t('agentGameAccess.deleteSuccess'))
      queryClient.invalidateQueries({ queryKey: ['agent-game-access'] })
    },
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])

  const summary = useMemo(() => ({
    total: data.length,
    enabled: data.filter((item) => item.status === 'enabled').length,
    disabled: data.filter((item) => item.status === 'disabled').length,
  }), [data])

  const columns: ColumnsType<AgentGameAccessListItem> = [
    {
      title: t('agentGameAccess.columns.agent'),
      key: 'agent',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.agentName}</Typography.Text>
          <Typography.Text type="secondary">#{record.agentID}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('agentGameAccess.columns.game'),
      key: 'game',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.gameName}</Typography.Text>
          <Typography.Text type="secondary">{record.gameCode}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('agentGameAccess.columns.status'),
      dataIndex: 'status',
      render: (value: AccessStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    {
      title: t('agentGameAccess.columns.grantedAt'),
      dataIndex: 'grantedAt',
    },
    {
      title: t('agentGameAccess.columns.grantedBy'),
      dataIndex: 'grantedBy',
      render: (value) => value || t('common.none'),
    },
    {
      title: t('agentGameAccess.columns.effectiveTo'),
      dataIndex: 'effectiveTo',
      render: (value) => value || t('common.none'),
    },
    {
      title: t('agentGameAccess.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Button
          danger
          icon={<DeleteOutlined />}
          loading={deleteMutation.isPending}
          onClick={() => deleteMutation.mutate({ agentID: record.agentID, gameID: record.gameID })}
        >
          {t('agentGameAccess.delete')}
        </Button>
      ),
    },
  ]

  const handleSubmit = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('common.scopeRequiredHint'))
      return
    }
    const values = await form.validateFields()
    upsertMutation.mutate(values)
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('agentGameAccess.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('agentGameAccess.subtitle')}
            </Typography.Paragraph>
          </div>
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={8}><Card className="app-stat-card"><Statistic title={t('agentGameAccess.visible')} value={summary.total} /></Card></Col>
            <Col xs={24} sm={12} lg={8}><Card className="app-stat-card"><Statistic title={t('agentGameAccess.enabled')} value={summary.enabled} /></Card></Col>
            <Col xs={24} sm={12} lg={8}><Card className="app-stat-card"><Statistic title={t('agentGameAccess.disabled')} value={summary.disabled} /></Card></Col>
          </Row>
          <ScopeNotice requireBrand />
          <div className="app-toolbar">
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('agentGameAccess.loadError')} description={(query.error as Error).message} /> : null}
      {upsertMutation.error ? <Alert type="error" showIcon message={t('agentGameAccess.saveError')} description={(upsertMutation.error as Error).message} /> : null}
      {deleteMutation.error ? <Alert type="error" showIcon message={t('agentGameAccess.deleteError')} description={(deleteMutation.error as Error).message} /> : null}

      <Form<AccessFormValues> form={form} layout="vertical">
        <Card className="app-table-card" title={t('agentGameAccess.createCardTitle')} extra={<Button type="primary" onClick={handleSubmit} loading={upsertMutation.isPending}>{t('agentGameAccess.save')}</Button>}>
          <Row gutter={[16, 16]}>
            <Col xs={24} md={6}>
              <Form.Item label={t('agentGameAccess.form.agentId')} name="agentID" rules={[{ required: true, message: t('agentGameAccess.form.agentIdRequired') }]}>
                <AgentSelect tenantID={scopeParams.tenantID} brandID={scopeParams.brandID} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={24} md={6}>
              <Form.Item label={t('agentGameAccess.form.gameId')} name="gameID" rules={[{ required: true, message: t('agentGameAccess.form.gameIdRequired') }]}>
                <GameSelect tenantID={scopeParams.tenantID} brandID={scopeParams.brandID} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={24} md={6}>
              <Form.Item label={t('agentGameAccess.form.status')} name="status" rules={[{ required: true, message: t('agentGameAccess.form.statusRequired') }]} initialValue="enabled">
                <Select options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
              </Form.Item>
            </Col>
            <Col xs={24} md={6}>
              <Form.Item label={t('agentGameAccess.form.remark')} name="remark">
                <Input placeholder={t('common.optional')} />
              </Form.Item>
            </Col>
          </Row>
        </Card>
      </Form>

      <Card className="app-table-card" title={t('agentGameAccess.tableTitle')} extra={<Typography.Text type="secondary">{data.length} {t('agentGameAccess.results')}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<AgentGameAccessListItem>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
          />
        </div>
      </Card>
    </Space>
  )
}
