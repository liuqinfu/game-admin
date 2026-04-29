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
  Modal,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useAuth } from '../../auth'
import { AgentSelect, GameSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { EnumDictionary, Rule, RuleScope, RuleStatus, RuleType, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: RuleStatus[] = ['draft', 'published', 'disabled']
const scopeOptions: RuleScope[] = ['platform', 'game', 'agent', 'agent_game']
const ruleTypeOptions: RuleType[] = ['ratio', 'fixed_share', 'differential', 'point', 'capped']

const statusColorMap: Record<RuleStatus, string> = {
  draft: 'default',
  published: 'success',
  disabled: 'warning',
}

type RuleFormValues = {
  ruleName: string
  scope: RuleScope
  ruleType: RuleType
  status: RuleStatus
  priority: number
  version: number
  agentID?: number
  gameID?: number
  maxSettlementDepth: number
  commissionRate: number
  fixedAmount: number
  minAgentLevel?: number
  rechargeTypes?: string[]
  activityTags?: string[]
  capAmount?: number
  currency: string
  effectiveFrom: string
  remark?: string
}

function normalizeTagValues(values?: string[]) {
  if (!values?.length) {
    return undefined
  }
  return Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)))
}

function getDictionaryByCode(items: EnumDictionary[] | undefined, code: string) {
  return items?.find((item) => item.code === code)
}

export default function RulesPage() {
  const { locale, t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [status, setStatus] = useState<RuleStatus | undefined>()
  const [scope, setScope] = useState<RuleScope | undefined>()
  const [keyword, setKeyword] = useState('')
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [form] = Form.useForm<RuleFormValues>()
  const canCreate = hasPermission('rules:create')
  const canPublish = hasPermission('rules:publish')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])
  const createScope = Form.useWatch('scope', form)

  const query = useQuery({
    queryKey: ['rules', scopeParams.tenantID, scopeParams.brandID, status, scope],
    queryFn: () => apiClient.listRules({ ...scopeParams, status, scope }),
  })
  const dictionariesQuery = useQuery({
    queryKey: ['enum-dictionaries', 'rules'],
    queryFn: () => apiClient.listEnumDictionaries(['recharge_type', 'activity_tag']),
  })

  const createMutation = useMutation({
    mutationFn: apiClient.createRule,
    onSuccess: () => {
      message.success(t('rules.createSuccess'))
      setIsCreateOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['rules'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard'] })
    },
  })

  const publishMutation = useMutation({
    mutationFn: ({ id, publishedBy }: { id: number; publishedBy?: string }) => apiClient.publishRule(id, publishedBy),
    onSuccess: () => {
      message.success(t('rules.publishSuccess'))
      queryClient.invalidateQueries({ queryKey: ['rules'] })
    },
  })

  const data = useMemo(() => {
    const items = query.data?.items ?? []
    if (!keyword.trim()) {
      return items
    }

    const normalized = keyword.trim().toLowerCase()
    return items.filter((item) =>
      [item.ruleName, item.remark, item.publishedBy]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(normalized)),
    )
  }, [keyword, query.data?.items])

  const rechargeTypeOptions = useMemo(() => {
    const dictionary = getDictionaryByCode(dictionariesQuery.data?.items, 'recharge_type')
    return (dictionary?.items ?? []).map((item) => ({
      label: locale === 'en-US' && item.labelEn ? item.labelEn : item.label || item.value,
      value: item.value,
    }))
  }, [dictionariesQuery.data?.items, locale])

  const activityTagOptions = useMemo(() => {
    const dictionary = getDictionaryByCode(dictionariesQuery.data?.items, 'activity_tag')
    return (dictionary?.items ?? []).map((item) => ({
      label: locale === 'en-US' && item.labelEn ? item.labelEn : item.label || item.value,
      value: item.value,
    }))
  }, [dictionariesQuery.data?.items, locale])

  const columns: ColumnsType<Rule> = [
    {
      title: t('rules.columns.rule'),
      dataIndex: 'ruleName',
      render: (value: string, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{value}</Typography.Text>
          <Typography.Text type="secondary">v{record.version}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('rules.columns.scopePriority'),
      key: 'scopePriority',
      render: (_, record) => (
        <Space direction="vertical" size={4}>
          <Space size={[4, 4]} wrap>
            <Tag>{t(`scope.${record.scope}`)}</Tag>
            <Tag color="purple">{record.ruleType ?? 'ratio'}</Tag>
            <Tag color="blue">P{record.priority}</Tag>
          </Space>
          {record.scope === 'agent' || record.scope === 'agent_game' || record.scope === 'game' ? (
            <Typography.Text type="secondary">
              {[
                record.scope === 'agent' || record.scope === 'agent_game' ? `${t('rules.columns.agentId')}: ${record.agentID ?? t('common.none')}` : null,
                record.scope === 'game' || record.scope === 'agent_game' ? `${t('rules.columns.gameId')}: ${record.gameID ?? t('common.none')}` : null,
              ]
                .filter(Boolean)
                .join(' · ')}
            </Typography.Text>
          ) : (
            <Typography.Text type="secondary">{t('rules.scopeSummary.platform')}</Typography.Text>
          )}
        </Space>
      ),
    },
    {
      title: t('rules.columns.rate'),
      key: 'commissionSummary',
      render: (_, record) => (
        <Tooltip title={t('rules.columns.priorityHint')}>
          <Space direction="vertical" size={0}>
            <Typography.Text>{`${(record.commissionRate * 100).toFixed(2)}%`}</Typography.Text>
            <Typography.Text type="secondary">{`${record.fixedAmount} ${record.currency}`}</Typography.Text>
          </Space>
        </Tooltip>
      ),
    },
    { title: t('rules.columns.depth'), dataIndex: 'maxSettlementDepth' },
    {
      title: t('rules.columns.status'),
      dataIndex: 'status',
      render: (value: RuleStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    { title: t('rules.columns.effectiveFrom'), dataIndex: 'effectiveFrom' },
    {
      title: t('rules.columns.actions'),
      key: 'actions',
      render: (_, record) => canPublish ? (
        <Button
          size="small"
          type="link"
          disabled={record.status !== 'draft'}
          loading={publishMutation.isPending}
          onClick={() => publishMutation.mutate({ id: record.id, publishedBy: 'admin-ui' })}
        >
          {t('rules.publish')}
        </Button>
      ) : null,
    },
  ]

  const handleCreate = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('rules.scopeRequiredHint'))
      return
    }
    const values = await form.validateFields()
    createMutation.mutate({
      ...scopeParams,
      ruleName: values.ruleName,
      scope: values.scope,
      ruleType: values.ruleType,
      status: values.status,
      priority: values.priority,
      version: values.version,
      agentID: values.agentID,
      gameID: values.gameID,
      maxSettlementDepth: values.maxSettlementDepth,
      commissionRate: values.commissionRate,
      fixedAmount: values.fixedAmount,
      minAgentLevel: values.minAgentLevel,
      rechargeTypes: normalizeTagValues(values.rechargeTypes),
      activityTags: normalizeTagValues(values.activityTags),
      capAmount: values.capAmount,
      currency: values.currency,
      effectiveFrom: new Date(values.effectiveFrom).toISOString(),
      remark: values.remark,
    })
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('rules.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('rules.subtitle')}
            </Typography.Paragraph>
          </div>
          <ScopeNotice requireBrand />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('rules.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<RuleStatus | undefined>
                allowClear
                placeholder={t('rules.statusPlaceholder')}
                value={status}
                onChange={(value) => setStatus(value)}
                style={{ width: 160 }}
                options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
              />
              <Select<RuleScope | undefined>
                allowClear
                placeholder={t('rules.scopePlaceholder')}
                value={scope}
                onChange={(value) => setScope(value)}
                style={{ width: 180 }}
                options={scopeOptions.map((item) => ({ label: t(`scope.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              {canCreate ? (
                <Button type="primary" onClick={() => setIsCreateOpen(true)}>
                  {t('rules.create')}
                </Button>
              ) : null}
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('rules.loadError')} description={(query.error as Error).message} /> : null}
      {dictionariesQuery.error ? <Alert type="error" showIcon message={t('rules.dictionaryLoadError')} description={(dictionariesQuery.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('rules.createError')} description={(createMutation.error as Error).message} /> : null}
      {publishMutation.error ? <Alert type="error" showIcon message={t('rules.publishError')} description={(publishMutation.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('rules.tableTitle')} extra={<Typography.Text type="secondary">{t('rules.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Rule>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('rules.empty')} /> }}
          />
        </div>
      </Card>

      <Modal
        className="app-modal-form app-modal-form--rules"
        title={t('rules.modalTitle')}
        open={canCreate && isCreateOpen}
        onCancel={() => setIsCreateOpen(false)}
        onOk={handleCreate}
        okText={t('common.ok')}
        cancelText={t('common.cancel')}
        confirmLoading={createMutation.isPending}
        width={960}
        destroyOnClose
      >
        <Form<RuleFormValues>
          className="app-form rules-form"
          form={form}
          layout="vertical"
          initialValues={{
            scope: 'platform',
            ruleType: 'ratio',
            status: 'draft',
            priority: 0,
            version: 1,
            maxSettlementDepth: 1,
            commissionRate: 0,
            fixedAmount: 0,
            currency: 'USD',
          }}
        >
          <div className="rules-form__section">
            <Typography.Text strong className="rules-form__section-title">{t('rules.form.sections.basic')}</Typography.Text>
            <Row gutter={[16, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.ruleName')} name="ruleName" rules={[{ required: true, message: t('rules.form.ruleNameRequired') }]}>
                  <Input placeholder={t('rules.form.ruleName')} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.scope')} name="scope" rules={[{ required: true, message: t('rules.form.scopeRequired') }]}>
                  <Select options={scopeOptions.map((item) => ({ label: t(`scope.${item}`), value: item }))} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.ruleType')} name="ruleType" rules={[{ required: true, message: t('rules.form.ruleTypeRequired') }]}>
                  <Select options={ruleTypeOptions.map((item) => ({ label: t(`rules.ruleType.${item}`), value: item }))} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.status')} name="status" rules={[{ required: true, message: t('rules.form.statusRequired') }]}>
                  <Select options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.priority')} name="priority" rules={[{ required: true, message: t('rules.form.priorityRequired') }]}>
                  <InputNumber style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.version')} name="version" rules={[{ required: true, message: t('rules.form.versionRequired') }]}>
                  <InputNumber min={1} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            </Row>
          </div>

          <div className="rules-form__section">
            <Typography.Text strong className="rules-form__section-title">{t('rules.form.sections.scope')}</Typography.Text>
            <Row gutter={[16, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.agentId')} name="agentID">
                  <AgentSelect
                    tenantID={scopeParams.tenantID}
                    brandID={scopeParams.brandID}
                    style={{ width: '100%' }}
                    disabled={createScope !== 'agent' && createScope !== 'agent_game'}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.gameId')} name="gameID">
                  <GameSelect
                    tenantID={scopeParams.tenantID}
                    brandID={scopeParams.brandID}
                    style={{ width: '100%' }}
                    disabled={createScope !== 'game' && createScope !== 'agent_game'}
                  />
                </Form.Item>
              </Col>
            </Row>
          </div>

          <div className="rules-form__section">
            <Typography.Text strong className="rules-form__section-title">{t('rules.form.sections.settlement')}</Typography.Text>
            <Row gutter={[16, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.maxSettlementDepth')} name="maxSettlementDepth" rules={[{ required: true, message: t('rules.form.maxSettlementDepthRequired') }]}>
                  <InputNumber min={1} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.minAgentLevel')} name="minAgentLevel">
                  <InputNumber min={0} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('rules.form.commissionRate')} name="commissionRate" rules={[{ required: true, message: t('rules.form.commissionRateRequired') }]}>
                  <InputNumber min={0} step={0.01} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('rules.form.fixedAmount')} name="fixedAmount" rules={[{ required: true, message: t('rules.form.fixedAmountRequired') }]}>
                  <InputNumber min={0} step={0.01} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={8}>
                <Form.Item label={t('rules.form.capAmount')} name="capAmount">
                  <InputNumber min={0} step={0.01} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.currency')} name="currency" rules={[{ required: true, message: t('rules.form.currencyRequired') }]}>
                  <Input placeholder={t('rules.form.currency')} />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.effectiveFrom')} name="effectiveFrom" rules={[{ required: true, message: t('rules.form.effectiveFromRequired') }]}>
                  <Input type="datetime-local" />
                </Form.Item>
              </Col>
            </Row>
          </div>

          <div className="rules-form__section">
            <Typography.Text strong className="rules-form__section-title">{t('rules.form.sections.filters')}</Typography.Text>
            <Row gutter={[16, 0]}>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.rechargeTypes')} name="rechargeTypes">
                  <Select
                    mode="multiple"
                    placeholder={t('rules.form.rechargeTypesPlaceholder')}
                    options={rechargeTypeOptions}
                    loading={dictionariesQuery.isLoading}
                    disabled={dictionariesQuery.isLoading || rechargeTypeOptions.length === 0}
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={12}>
                <Form.Item label={t('rules.form.activityTags')} name="activityTags">
                  <Select
                    mode="multiple"
                    placeholder={t('rules.form.activityTagsPlaceholder')}
                    options={activityTagOptions}
                    loading={dictionariesQuery.isLoading}
                    disabled={dictionariesQuery.isLoading || activityTagOptions.length === 0}
                  />
                </Form.Item>
              </Col>
              <Col span={24}>
                <Form.Item label={t('rules.form.remark')} name="remark">
                  <Input.TextArea rows={3} placeholder={t('common.optional')} />
                </Form.Item>
              </Col>
            </Row>
          </div>
        </Form>
      </Modal>
    </Space>
  )
}
