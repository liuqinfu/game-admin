import { ReloadOutlined } from '@ant-design/icons'
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
import { useAuth } from '../../auth'
import { AgentSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { AgentInviteApplication, AgentInviteApplicationStatus, apiClient } from '../../lib/api'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: AgentInviteApplicationStatus[] = ['pending', 'approved', 'rejected']

const statusColorMap: Record<AgentInviteApplicationStatus, string> = {
  pending: 'processing',
  approved: 'success',
  rejected: 'error',
}

type ApplicationFormValues = {
  applicantAgentID: number
  inviteCode: string
  applyRemark?: string
}

type AuditFormValues = {
  status: AgentInviteApplicationStatus
  auditRemark?: string
}

export default function AgentInviteApplicationsPage() {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [createForm] = Form.useForm<ApplicationFormValues>()
  const [auditForm] = Form.useForm<Record<number, AuditFormValues>>()
  const canCreate = hasPermission('agent-invite-applications:create')
  const canAudit = hasPermission('agent-invite-applications:audit')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['agent-invite-applications', scopeParams.tenantID, scopeParams.brandID],
    queryFn: () => apiClient.listAgentInviteApplications(scopeParams),
  })

  const createMutation = useMutation({
    mutationFn: apiClient.createAgentInviteApplication,
    onSuccess: () => {
      message.success(t('agentInviteApplications.createSuccess'))
      createForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['agent-invite-applications'] })
    },
  })

  const auditMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: AuditFormValues }) => apiClient.auditAgentInviteApplication(id, payload),
    onSuccess: () => {
      message.success(t('agentInviteApplications.auditSuccess'))
      queryClient.invalidateQueries({ queryKey: ['agent-invite-applications'] })
    },
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])

  const summary = useMemo(() => ({
    total: data.length,
    pending: data.filter((item) => item.status === 'pending').length,
    approved: data.filter((item) => item.status === 'approved').length,
    rejected: data.filter((item) => item.status === 'rejected').length,
  }), [data])

  const columns: ColumnsType<AgentInviteApplication> = [
    {
      title: t('agentInviteApplications.columns.applicationId'),
      dataIndex: 'id',
    },
    {
      title: t('agentInviteApplications.columns.applicantAgentId'),
      dataIndex: 'applicantAgentID',
    },
    {
      title: t('agentInviteApplications.columns.inviterAgentId'),
      dataIndex: 'inviterAgentID',
    },
    {
      title: t('agentInviteApplications.columns.status'),
      dataIndex: 'status',
      render: (value: AgentInviteApplicationStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    {
      title: t('agentInviteApplications.columns.applyRemark'),
      dataIndex: 'applyRemark',
      render: (value) => value || t('common.none'),
    },
    {
      title: t('agentInviteApplications.columns.auditRemark'),
      dataIndex: 'auditRemark',
      render: (value) => value || t('common.none'),
    },
    {
      title: t('agentInviteApplications.columns.auditBy'),
      dataIndex: 'auditBy',
      render: (value) => value || t('common.none'),
    },
    {
      title: t('agentInviteApplications.columns.createdAt'),
      dataIndex: 'createdAt',
    },
    {
      title: t('agentInviteApplications.columns.actions'),
      key: 'actions',
      render: (_, record) => canAudit ? (
        <Space.Compact>
          <Form.Item name={[record.id, 'status']} initialValue={record.status === 'pending' ? 'approved' : record.status} noStyle>
            <Select style={{ width: 140 }} options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
          </Form.Item>
          <Form.Item name={[record.id, 'auditRemark']} initialValue="" noStyle>
            <Input style={{ width: 220 }} placeholder={t('agentInviteApplications.auditPlaceholder')} />
          </Form.Item>
          <Button
            type="primary"
            loading={auditMutation.isPending}
            onClick={async () => {
              const values = await auditForm.validateFields([[record.id, 'status']])
              const payload = values[record.id] as AuditFormValues
              auditMutation.mutate({ id: record.id, payload })
            }}
          >
            {t('agentInviteApplications.audit')}
          </Button>
        </Space.Compact>
      ) : null,
    },
  ]

  const handleCreate = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('common.scopeRequiredHint'))
      return
    }
    const values = await createForm.validateFields()
    createMutation.mutate(values)
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('agentInviteApplications.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('agentInviteApplications.subtitle')}
            </Typography.Paragraph>
          </div>
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('agentInviteApplications.visible')} value={summary.total} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('agentInviteApplications.pending')} value={summary.pending} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('agentInviteApplications.approved')} value={summary.approved} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('agentInviteApplications.rejected')} value={summary.rejected} /></Card></Col>
          </Row>
          <div className="app-toolbar">
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('agentInviteApplications.loadError')} description={(query.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('agentInviteApplications.createError')} description={(createMutation.error as Error).message} /> : null}
      {auditMutation.error ? <Alert type="error" showIcon message={t('agentInviteApplications.auditError')} description={(auditMutation.error as Error).message} /> : null}

      {canCreate ? (
        <Form<ApplicationFormValues> form={createForm} layout="vertical">
          <Card className="app-table-card" title={t('agentInviteApplications.createCardTitle')} extra={<Button type="primary" onClick={handleCreate} loading={createMutation.isPending}>{t('agentInviteApplications.create')}</Button>}>
            <Row gutter={[16, 16]}>
              <Col xs={24} md={8}>
                <Form.Item label={t('agentInviteApplications.form.applicantAgentId')} name="applicantAgentID" rules={[{ required: true, message: t('agentInviteApplications.form.applicantAgentIdRequired') }]}>
                  <AgentSelect tenantID={scopeParams.tenantID} brandID={scopeParams.brandID} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('agentInviteApplications.form.inviteCode')} name="inviteCode" rules={[{ required: true, message: t('agentInviteApplications.form.inviteCodeRequired') }]}>
                  <Input placeholder={t('agentInviteApplications.form.inviteCodePlaceholder')} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('agentInviteApplications.form.applyRemark')} name="applyRemark">
                  <Input placeholder={t('common.optional')} />
                </Form.Item>
              </Col>
            </Row>
          </Card>
        </Form>
      ) : null}

      <Form form={auditForm} component={false}>
        <Card className="app-table-card" title={t('agentInviteApplications.tableTitle')} extra={<Typography.Text type="secondary">{data.length} {t('agentInviteApplications.results')}</Typography.Text>}>
          <div className="app-table app-table--compact">
            <Table<AgentInviteApplication>
              rowKey="id"
              loading={query.isLoading || query.isFetching}
              dataSource={data}
              columns={columns}
              pagination={{ pageSize: 10, showSizeChanger: false }}
            />
          </div>
        </Card>
      </Form>
    </Space>
  )
}
