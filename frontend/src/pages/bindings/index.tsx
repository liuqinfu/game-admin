import { ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  InputNumber,
  Row,
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
import { useAuth } from '../../auth'
import { useI18n } from '../../i18n'
import { Binding, BindingStatus, Player, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusColorMap: Record<BindingStatus, string> = {
  bound: 'success',
  pending_change: 'processing',
  changed: 'warning',
  released: 'default',
}

type BindingFormValues = {
  playerID: number
  inviteCode: string
  remark?: string
}

export default function BindingsPage() {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [form] = Form.useForm<BindingFormValues>()
  const canCreate = hasPermission('bindings:create')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const playersQuery = useQuery({
    queryKey: ['players', scopeParams.tenantID, scopeParams.brandID, keyword],
    queryFn: () => apiClient.listPlayers({ ...scopeParams, keyword: keyword || undefined }),
  })

  const bindingsQuery = useQuery({
    queryKey: ['bindings', scopeParams.tenantID, scopeParams.brandID],
    queryFn: () => apiClient.listBindings(scopeParams),
  })

  const createMutation = useMutation({
    mutationFn: apiClient.createBinding,
    onSuccess: () => {
      message.success(t('bindings.createSuccess'))
      setIsCreateOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['bindings'] })
      queryClient.invalidateQueries({ queryKey: ['binding-history'] })
    },
  })

  const playerMap = useMemo(() => {
    const map = new Map<number, Player>()
    for (const item of playersQuery.data?.items ?? []) {
      map.set(item.id, item)
    }
    return map
  }, [playersQuery.data?.items])

  const data = useMemo(() => {
    const items = bindingsQuery.data?.items ?? []
    const normalized = keyword.trim().toLowerCase()
    if (!normalized) {
      return items
    }

    return items.filter((item) => {
      const player = playerMap.get(item.playerID)
      return [
        item.id,
        item.playerID,
        item.agentID,
        item.inviteCodeID,
        item.remark,
        player?.playerNo,
        player?.platformUserID,
        player?.nickname,
        player?.phone,
      ]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(normalized))
    })
  }, [bindingsQuery.data?.items, keyword, playerMap])

  const summary = useMemo(() => ({
    total: data.length,
    active: data.filter((item) => item.status === 'bound').length,
    pending: data.filter((item) => item.status === 'pending_change').length,
    released: data.filter((item) => item.status === 'released').length,
  }), [data])

  const columns: ColumnsType<Binding> = [
    {
      title: t('bindings.columns.player'),
      key: 'player',
      render: (_, record) => {
        const player = playerMap.get(record.playerID)
        return (
          <Space direction="vertical" size={0}>
            <Typography.Text strong>{player?.nickname || player?.playerNo || t('common.none')}</Typography.Text>
            <Typography.Text type="secondary">#{record.playerID}</Typography.Text>
          </Space>
        )
      },
    },
    {
      title: t('bindings.columns.agentId'),
      dataIndex: 'agentID',
    },
    {
      title: t('bindings.columns.inviteCodeId'),
      dataIndex: 'inviteCodeID',
      render: (value) => value ?? t('common.none'),
    },
    {
      title: t('bindings.columns.status'),
      dataIndex: 'status',
      render: (value: BindingStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    {
      title: t('bindings.columns.source'),
      dataIndex: 'source',
      render: (value) => t(`bindings.source.${value}`),
    },
    {
      title: t('bindings.columns.boundAt'),
      dataIndex: 'boundAt',
    },
    {
      title: t('bindings.columns.remark'),
      dataIndex: 'remark',
      render: (value) => value || t('common.none'),
    },
  ]

  const handleCreate = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('common.scopeRequiredHint'))
      return
    }
    const values = await form.validateFields()
    createMutation.mutate(values)
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('bindings.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('bindings.subtitle')}
            </Typography.Paragraph>
          </div>
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('bindings.visible')} value={summary.total} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('bindings.bound')} value={summary.active} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('bindings.pendingChange')} value={summary.pending} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('bindings.released')} value={summary.released} /></Card></Col>
          </Row>
          <ScopeNotice requireBrand />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('bindings.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
            </div>
            <div className="app-toolbar__actions">
              {canCreate ? (
                <Button type="primary" onClick={() => setIsCreateOpen(true)}>
                  {t('bindings.create')}
                </Button>
              ) : null}
              <Button icon={<ReloadOutlined />} onClick={() => { void playersQuery.refetch(); void bindingsQuery.refetch() }} loading={playersQuery.isFetching || bindingsQuery.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {playersQuery.error ? <Alert type="error" showIcon message={t('bindings.playersLoadError')} description={(playersQuery.error as Error).message} /> : null}
      {bindingsQuery.error ? <Alert type="error" showIcon message={t('bindings.loadError')} description={(bindingsQuery.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('bindings.createError')} description={(createMutation.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('bindings.tableTitle')} extra={<Typography.Text type="secondary">{data.length} {t('bindings.results')}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Binding>
            rowKey="id"
            loading={playersQuery.isLoading || bindingsQuery.isLoading || playersQuery.isFetching || bindingsQuery.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('bindings.empty')} /> }}
          />
        </div>
      </Card>

      {canCreate ? (
        <Form<BindingFormValues> form={form} layout="vertical" component={false}>
          <Card className="app-table-card" title={t('bindings.createCardTitle')} extra={
            <Button type="primary" onClick={handleCreate} loading={createMutation.isPending}>
              {t('bindings.submit')}
            </Button>
          }>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={8}>
                <Form.Item label={t('bindings.form.playerId')} name="playerID" rules={[{ required: true, message: t('bindings.form.playerIdRequired') }]}>
                  <InputNumber min={1} style={{ width: '100%' }} placeholder={t('bindings.form.playerIdPlaceholder')} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('bindings.form.inviteCode')} name="inviteCode" rules={[{ required: true, message: t('bindings.form.inviteCodeRequired') }]}>
                  <Input placeholder={t('bindings.form.inviteCodePlaceholder')} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('bindings.form.remark')} name="remark">
                  <Input placeholder={t('common.optional')} />
                </Form.Item>
              </Col>
            </Row>
          </Card>
        </Form>
      ) : null}
    </Space>
  )
}
