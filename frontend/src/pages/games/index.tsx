import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
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
import { useI18n } from '../../i18n'
import { Game, GameStatus, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: GameStatus[] = ['draft', 'online', 'offline', 'archived']

const statusColorMap: Record<GameStatus, string> = {
  draft: 'default',
  online: 'success',
  offline: 'warning',
  archived: 'error',
}

type GameFormValues = {
  gameCode: string
  name: string
  vendor?: string
  category?: string
  status: GameStatus
  isAgentable: boolean
  sort: number
  remark?: string
}

function resolveGameDeleteError(t: (key: string) => string, error: Error) {
  switch (error.message) {
    case 'game has active integration keys':
      return t('games.deleteBlockedKeys')
    case 'game has active agent access grants':
      return t('games.deleteBlockedAccess')
    case 'game has active registered players':
      return t('games.deleteBlockedPlayers')
    case 'game has active recharge orders':
      return t('games.deleteBlockedOrders')
    case 'game has active commission records':
      return t('games.deleteBlockedCommissions')
    case 'game has active commission rules':
      return t('games.deleteBlockedRules')
    case 'game has active rule snapshots':
      return t('games.deleteBlockedSnapshots')
    default:
      return error.message || t('games.deleteError')
  }
}

export default function GamesPage() {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<GameStatus | undefined>()
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [editingGame, setEditingGame] = useState<Game | null>(null)
  const [credentialGame, setCredentialGame] = useState<Game | null>(null)
  const [form] = Form.useForm<GameFormValues>()
  const canCreate = hasPermission('games:create')
  const canUpdate = hasPermission('games:update')
  const canUpdateStatus = hasPermission('games:status:update')
  const canDelete = hasPermission('games:delete')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['games', scopeParams.tenantID, scopeParams.brandID, keyword, status],
    queryFn: () => apiClient.listGames({ ...scopeParams, keyword: keyword || undefined, status }),
  })

  const createMutation = useMutation({
    mutationFn: apiClient.createGame,
    onSuccess: (result) => {
      message.success(t('games.createSuccess'))
      setIsCreateOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
      if (result.integrationCredential) {
        Modal.info({
          title: t('games.integrationCredential.title'),
          width: 680,
          content: (
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Alert type="warning" showIcon message={t('games.integrationCredential.onceWarning')} />
              <div>
                <Typography.Text strong>{t('games.integrationCredential.accessKey')}</Typography.Text>
                <Typography.Paragraph copyable code>{result.integrationCredential.accessKey}</Typography.Paragraph>
              </div>
              <div>
                <Typography.Text strong>{t('games.integrationCredential.secretKey')}</Typography.Text>
                <Typography.Paragraph copyable code>{result.integrationCredential.secretKey}</Typography.Paragraph>
              </div>
              <div>
                <Typography.Text strong>{t('games.integrationCredential.scopes')}</Typography.Text>
                <Space size={[6, 6]} wrap style={{ marginTop: 8 }}>
                  {result.integrationCredential.scopes.map((scope) => <Tag key={scope}>{scope}</Tag>)}
                </Space>
              </div>
            </Space>
          ),
        })
      }
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: Partial<Game> }) => apiClient.updateGame(id, payload),
    onSuccess: () => {
      message.success(t('games.updateSuccess'))
      setEditingGame(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['games'] })
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, nextStatus }: { id: number; nextStatus: GameStatus }) => apiClient.updateGameStatus(id, nextStatus),
    onSuccess: () => {
      message.success(t('games.statusSuccess'))
      queryClient.invalidateQueries({ queryKey: ['games'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: apiClient.deleteGame,
    onSuccess: (_, id) => {
      message.success(t('games.deleteSuccess'))
      if (editingGame?.id === id) {
        setEditingGame(null)
        form.resetFields()
      }
      if (credentialGame?.id === id) {
        setCredentialGame(null)
      }
      queryClient.invalidateQueries({ queryKey: ['games'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
    onError: (error) => {
      message.error(resolveGameDeleteError(t, error as Error))
    },
  })

  const credentialQuery = useQuery({
    queryKey: ['game-integration-keys', credentialGame?.id],
    queryFn: () => apiClient.listGameIntegrationKeys(credentialGame!.id),
    enabled: Boolean(credentialGame),
  })

  const rotateCredentialMutation = useMutation({
    mutationFn: ({ gameID, keyID }: { gameID: number; keyID: number }) => apiClient.rotateGameIntegrationKey(gameID, keyID),
    onSuccess: (credential) => {
      message.success(t('games.integrationCredential.rotateSuccess'))
      credentialQuery.refetch()
      Modal.info({
        title: t('games.integrationCredential.rotatedTitle'),
        width: 680,
            content: renderCredentialSecret(credential, t),
      })
    },
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])

  const confirmDeleteGame = (game: Game) => {
    Modal.confirm({
      title: t('games.deleteConfirmTitle'),
      content: t('games.deleteConfirmContent', { name: game.name }),
      okText: t('common.delete'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: () => deleteMutation.mutateAsync(game.id),
    })
  }

  const columns: ColumnsType<Game> = [
    {
      title: t('games.columns.game'),
      key: 'game',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text type="secondary">{record.gameCode}</Typography.Text>
        </Space>
      ),
    },
    { title: t('games.columns.vendor'), dataIndex: 'vendor', render: (value) => value || t('common.none') },
    { title: t('games.columns.category'), dataIndex: 'category', render: (value) => value || t('common.none') },
    {
      title: t('games.columns.status'),
      dataIndex: 'status',
      render: (value: GameStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    {
      title: t('games.columns.agentable'),
      dataIndex: 'isAgentable',
      render: (value: boolean) => <Tag color={value ? 'success' : 'default'}>{value ? t('common.yes') : t('common.no')}</Tag>,
    },
    { title: t('games.columns.currency'), dataIndex: 'currency' },
    { title: t('games.columns.sort'), dataIndex: 'sort' },
    { title: t('games.columns.launchAt'), dataIndex: 'launchAt', render: (value) => value || t('common.none') },
    {
      title: t('games.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Space wrap className="app-actions-inline">
          {canUpdate ? (
            <Button size="small" onClick={() => {
              setEditingGame(record)
              form.setFieldsValue({
                gameCode: record.gameCode,
                name: record.name,
                vendor: record.vendor,
                category: record.category,
                status: record.status,
                isAgentable: record.isAgentable,
                sort: record.sort,
                remark: record.remark,
              })
            }}>
              {t('games.edit')}
            </Button>
          ) : null}
          {canUpdate ? (
            <Button size="small" onClick={() => setCredentialGame(record)}>
              {t('games.integrationCredential.manage')}
            </Button>
          ) : null}
          {canUpdateStatus ? (
            <Select<GameStatus>
              value={record.status}
              style={{ width: 130 }}
              options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
              loading={statusMutation.isPending}
              onChange={(nextStatus) => {
                if (nextStatus !== record.status) {
                  statusMutation.mutate({ id: record.id, nextStatus })
                }
              }}
            />
          ) : null}
          {canDelete ? (
            <Button danger icon={<DeleteOutlined />} loading={deleteMutation.isPending} onClick={() => confirmDeleteGame(record)}>
              {t('common.delete')}
            </Button>
          ) : null}
        </Space>
      ),
    },
  ]

  const submitForm = async () => {
    const values = await form.validateFields()
    const payload = {
      gameCode: values.gameCode,
      name: values.name,
      vendor: values.vendor,
      category: values.category,
      status: values.status,
      isAgentable: values.isAgentable,
      sort: values.sort,
      remark: values.remark,
    }

    if (editingGame) {
      updateMutation.mutate({ id: editingGame.id, payload })
      return
    }

    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('common.scopeRequiredHint'))
      return
    }

    createMutation.mutate({ ...scopeParams, ...payload })
  }

  const closeModal = () => {
    setIsCreateOpen(false)
    setEditingGame(null)
    form.resetFields()
  }

  const credentialData = credentialQuery.data?.items ?? []

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('games.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('games.subtitle')}
            </Typography.Paragraph>
          </div>
          <ScopeNotice requireBrand />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('games.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<GameStatus | undefined>
                allowClear
                placeholder={t('games.statusPlaceholder')}
                value={status}
                onChange={(value) => setStatus(value)}
                style={{ width: 160 }}
                options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              {canCreate ? (
                <Button type="primary" onClick={() => setIsCreateOpen(true)}>
                  {t('games.create')}
                </Button>
              ) : null}
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('games.loadError')} description={(query.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('games.createError')} description={(createMutation.error as Error).message} /> : null}
      {updateMutation.error ? <Alert type="error" showIcon message={t('games.updateError')} description={(updateMutation.error as Error).message} /> : null}
      {statusMutation.error ? <Alert type="error" showIcon message={t('games.statusError')} description={(statusMutation.error as Error).message} /> : null}
      {deleteMutation.error ? <Alert type="error" showIcon message={t('games.deleteError')} description={resolveGameDeleteError(t, deleteMutation.error as Error)} /> : null}
      {credentialQuery.error ? <Alert type="error" showIcon message={t('games.integrationCredential.loadError')} description={(credentialQuery.error as Error).message} /> : null}
      {rotateCredentialMutation.error ? <Alert type="error" showIcon message={t('games.integrationCredential.rotateError')} description={(rotateCredentialMutation.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('games.tableTitle')} extra={<Typography.Text type="secondary">{t('games.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Game>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('games.empty')} /> }}
          />
        </div>
      </Card>

      <Modal
        className="app-modal-form"
        title={editingGame ? t('games.modalEditTitle') : t('games.modalCreateTitle')}
        open={(canCreate && isCreateOpen) || (canUpdate && Boolean(editingGame))}
        onCancel={closeModal}
        onOk={submitForm}
        okText={t('common.ok')}
        cancelText={t('common.cancel')}
        confirmLoading={createMutation.isPending || updateMutation.isPending}
        destroyOnClose
      >
        <Form<GameFormValues>
          className="app-form"
          form={form}
          layout="vertical"
          initialValues={{ status: 'draft', isAgentable: true, sort: 0 }}
        >
          <Form.Item label={t('games.form.gameCode')} name="gameCode" rules={[{ required: true, message: t('games.form.gameCodeRequired') }]}>
            <Input placeholder={t('games.form.gameCode')} />
          </Form.Item>
          <Form.Item label={t('games.form.name')} name="name" rules={[{ required: true, message: t('games.form.nameRequired') }]}>
            <Input placeholder={t('games.form.name')} />
          </Form.Item>
          <Form.Item label={t('games.form.vendor')} name="vendor">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('games.form.category')} name="category">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('games.form.status')} name="status" rules={[{ required: true, message: t('games.form.statusRequired') }]}>
            <Select options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
          </Form.Item>
          <Form.Item label={t('games.form.isAgentable')} name="isAgentable" rules={[{ required: true, message: t('games.form.isAgentableRequired') }]}>
            <Select options={[{ label: t('common.yes'), value: true }, { label: t('common.no'), value: false }]} />
          </Form.Item>
          <Form.Item label={t('games.form.sort')} name="sort" rules={[{ required: true, message: t('games.form.sortRequired') }]}>
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label={t('games.form.remark')} name="remark">
            <Input.TextArea rows={3} placeholder={t('common.optional')} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={t('games.integrationCredential.manageTitle')}
        open={Boolean(credentialGame)}
        onCancel={() => setCredentialGame(null)}
        footer={null}
        width={860}
        destroyOnClose
      >
        <Table
          rowKey="id"
          size="small"
          loading={credentialQuery.isLoading || credentialQuery.isFetching}
          dataSource={credentialData}
          pagination={false}
          locale={{ emptyText: <Empty description={t('games.integrationCredential.empty')} /> }}
          columns={[
            {
              title: t('games.integrationCredential.accessKey'),
              dataIndex: 'accessKey',
              render: (value: string) => <Typography.Text copyable code>{value}</Typography.Text>,
            },
            {
              title: t('games.integrationCredential.status'),
              dataIndex: 'status',
              render: (value: string) => <Tag color={value === 'active' ? 'success' : 'default'}>{value}</Tag>,
            },
            {
              title: t('games.integrationCredential.scopes'),
              dataIndex: 'scopes',
              render: (scopes: string[]) => <Space size={[4, 4]} wrap>{scopes.map((scope) => <Tag key={scope}>{scope}</Tag>)}</Space>,
            },
            { title: t('games.integrationCredential.lastUsedAt'), dataIndex: 'lastUsedAt', render: (value: string | null) => value || t('common.none') },
            { title: t('games.integrationCredential.rotatedAt'), dataIndex: 'rotatedAt', render: (value: string | null) => value || t('common.none') },
            {
              title: t('games.columns.actions'),
              key: 'actions',
              render: (_, credential) => (
                <Button
                  size="small"
                  danger
                  loading={rotateCredentialMutation.isPending}
                  onClick={() => {
                    if (!credentialGame) return
                    Modal.confirm({
                      title: t('games.integrationCredential.rotateConfirmTitle'),
                      content: t('games.integrationCredential.rotateConfirmContent'),
                      okText: t('games.integrationCredential.rotate'),
                      cancelText: t('common.cancel'),
                      onOk: () => rotateCredentialMutation.mutateAsync({ gameID: credentialGame.id, keyID: credential.id }),
                    })
                  }}
                >
                  {t('games.integrationCredential.rotate')}
                </Button>
              ),
            },
          ]}
        />
      </Modal>
    </Space>
  )
}

function renderCredentialSecret(credential: { accessKey: string; secretKey?: string; scopes: string[] }, t: (key: string) => string) {
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Alert type="warning" showIcon message={t('games.integrationCredential.onceWarning')} />
      <div>
        <Typography.Text strong>{t('games.integrationCredential.accessKey')}</Typography.Text>
        <Typography.Paragraph copyable code>{credential.accessKey}</Typography.Paragraph>
      </div>
      <div>
        <Typography.Text strong>{t('games.integrationCredential.secretKey')}</Typography.Text>
        <Typography.Paragraph copyable code>{credential.secretKey}</Typography.Paragraph>
      </div>
      <div>
        <Typography.Text strong>{t('games.integrationCredential.scopes')}</Typography.Text>
        <Space size={[6, 6]} wrap style={{ marginTop: 8 }}>
          {credential.scopes.map((scope) => <Tag key={scope}>{scope}</Tag>)}
        </Space>
      </div>
    </Space>
  )
}
