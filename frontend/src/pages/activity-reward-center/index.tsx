import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useAuth } from '../../auth'
import { AgentSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import {
  type ActivityRewardRecord,
  type ActivityRewardRecordStatus,
  type ActivityRewardRule,
  type ActivityRewardRuleStatus,
  apiClient,
} from '../../lib/api'
import { PlatformScopeSummary } from '../../platform-scope/PlatformScopeSummary'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

type RewardRuleFormValues = {
  name: string
  activityType: string
  rewardType: string
  status: ActivityRewardRuleStatus
  rewardValue: number
  currency: string
  triggerValue?: number
  dailyLimit?: number
  totalLimit?: number
  schedule?: [dayjs.Dayjs, dayjs.Dayjs]
  remark?: string
}

type GenerateRecordFormValues = {
  playerID: number
  agentID?: number
  referenceType: string
  referenceID: string
  remark?: string
}

type ReverseRecordFormValues = {
  remark?: string
}

const ruleStatuses: ActivityRewardRuleStatus[] = ['draft', 'active', 'disabled']
const recordStatusOptions: ActivityRewardRecordStatus[] = ['pending', 'granted', 'failed', 'reversed']
const activityTypeOptions = ['login', 'recharge', 'bet', 'referral']
const rewardTypeOptions = ['cash', 'bonus', 'coupon']

const ruleStatusColorMap: Record<ActivityRewardRuleStatus, string> = {
  draft: 'default',
  active: 'success',
  disabled: 'warning',
}

const recordStatusColorMap: Record<ActivityRewardRecordStatus, string> = {
  pending: 'processing',
  granted: 'success',
  failed: 'error',
  reversed: 'default',
}

const formatNumber = (value?: number | null, suffix = '') => {
  if (value === undefined || value === null) {
    return '-'
  }

  return `${value}${suffix}`
}

export default function ActivityRewardCenterPage() {
  const { t } = useI18n()
  const location = useLocation()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const { message } = App.useApp()
  const { hasPermission } = useAuth()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const queryClient = useQueryClient()
  const [form] = Form.useForm<RewardRuleFormValues>()
  const [generateForm] = Form.useForm<GenerateRecordFormValues>()
  const [reverseForm] = Form.useForm<ReverseRecordFormValues>()
  const view = useMemo<'rules' | 'records'>(() => (location.pathname.endsWith('/records') ? 'records' : 'rules'), [location.pathname])
  const keyword = searchParams.get('keyword') || ''
  const ruleStatus = (searchParams.get('ruleStatus') as ActivityRewardRuleStatus | null) ?? undefined
  const recordStatus = (searchParams.get('recordStatus') as ActivityRewardRecordStatus | null) ?? undefined
  const activityType = searchParams.get('activityType') || undefined
  const setParam = (key: string, value?: string | number) => {
    const next = new URLSearchParams(searchParams)
    if (value === undefined || value === '') {
      next.delete(key)
    } else {
      next.set(key, String(value))
    }
    setSearchParams(next, { replace: true })
  }
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [isGenerateOpen, setIsGenerateOpen] = useState(false)
  const [isReverseOpen, setIsReverseOpen] = useState(false)
  const [selectedRule, setSelectedRule] = useState<ActivityRewardRule | null>(null)
  const [selectedRecord, setSelectedRecord] = useState<ActivityRewardRecord | null>(null)

  const scopeParams = useMemo(
    () => ({ tenantID: activeTenantId, brandID: activeBrandId }),
    [activeBrandId, activeTenantId],
  )

  const canCreate = hasPermission('activity-rewards:create')
  const canManageStatus = hasPermission('activity-rewards:status:update')

  const rulesQuery = useQuery({
    queryKey: ['activity-reward-rules', scopeParams.tenantID, scopeParams.brandID, ruleStatus, activityType, keyword],
    queryFn: () => apiClient.listActivityRewardRules({ ...scopeParams, status: ruleStatus, activityType, keyword: keyword.trim() || undefined }),
    enabled: view === 'rules',
  })

  const recordsQuery = useQuery({
    queryKey: ['activity-reward-records', scopeParams.tenantID, scopeParams.brandID, recordStatus, keyword],
    queryFn: () => apiClient.listActivityRewardRecords({ ...scopeParams, status: recordStatus, keyword: keyword.trim() || undefined }),
    enabled: view === 'records',
  })

  const createRuleMutation = useMutation({
    mutationFn: apiClient.createActivityRewardRule,
    onSuccess: () => {
      message.success(t('activityRewardCenter.createSuccess'))
      setIsCreateOpen(false)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['activity-reward-rules'] })
    },
  })

  const statusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: ActivityRewardRuleStatus }) =>
      apiClient.updateActivityRewardRuleStatus(id, { status }),
    onSuccess: () => {
      message.success(t('activityRewardCenter.statusSuccess'))
      queryClient.invalidateQueries({ queryKey: ['activity-reward-rules'] })
    },
  })

  const generateRecordMutation = useMutation({
    mutationFn: ({ ruleID, payload }: { ruleID: number; payload: GenerateRecordFormValues }) =>
      apiClient.generateActivityRewardRecord(ruleID, payload),
    onSuccess: () => {
      message.success(t('activityRewardCenter.generateSuccess'))
      setIsGenerateOpen(false)
      setSelectedRule(null)
      generateForm.resetFields()
      navigate('/activity-reward-center/records')
      queryClient.invalidateQueries({ queryKey: ['activity-reward-records'] })
    },
  })

  const reverseRecordMutation = useMutation({
    mutationFn: ({ recordID, payload }: { recordID: number; payload: ReverseRecordFormValues }) =>
      apiClient.reverseActivityRewardRecord(recordID, payload),
    onSuccess: () => {
      message.success(t('activityRewardCenter.reverseSuccess'))
      setIsReverseOpen(false)
      setSelectedRecord(null)
      reverseForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['activity-reward-records'] })
    },
  })

  const rules = rulesQuery.data?.items ?? []
  const records = recordsQuery.data?.items ?? []

  const stats = useMemo(() => {
    const activeRules = rules.filter((item) => item.status === 'active').length
    const draftRules = rules.filter((item) => item.status === 'draft').length
    const grantedRecords = records.filter((item) => item.status === 'granted').length

    return {
      totalRules: rules.length,
      activeRules,
      draftRules,
      totalRecords: records.length,
      grantedRecords,
    }
  }, [records, rules])

  const openGenerateModal = (rule: ActivityRewardRule) => {
    setSelectedRule(rule)
    generateForm.setFieldsValue({
      referenceType: 'manual',
    })
    setIsGenerateOpen(true)
  }

  const openReverseModal = (record: ActivityRewardRecord) => {
    setSelectedRecord(record)
    reverseForm.setFieldsValue({
      remark: record.remark,
    })
    setIsReverseOpen(true)
  }

  const ruleColumns: ColumnsType<ActivityRewardRule> = [
    {
      title: t('activityRewardCenter.rules.columns.rule'),
      dataIndex: 'name',
      render: (value: string, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{value}</Typography.Text>
          <Typography.Text type="secondary">{record.remark || t('common.none')}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.rules.columns.activity'),
      key: 'activityType',
      render: (_, record) => (
        <Space size={[6, 6]} wrap>
          <Tag color="blue">{t(`activityRewardCenter.activityType.${record.activityType}`)}</Tag>
          <Tag>{t(`activityRewardCenter.rewardType.${record.rewardType}`)}</Tag>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.rules.columns.reward'),
      key: 'reward',
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{`${record.rewardValue} ${record.currency}`}</Typography.Text>
          <Typography.Text type="secondary">
            {`${t('activityRewardCenter.rules.triggerValue')}: ${formatNumber(record.triggerValue)}`}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.rules.columns.limits'),
      key: 'limits',
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{`${t('activityRewardCenter.rules.dailyLimit')}: ${formatNumber(record.dailyLimit)}`}</Typography.Text>
          <Typography.Text type="secondary">{`${t('activityRewardCenter.rules.totalLimit')}: ${formatNumber(record.totalLimit)}`}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.rules.columns.period'),
      key: 'period',
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{record.startAt ? dayjs(record.startAt).format('YYYY-MM-DD HH:mm') : t('common.none')}</Typography.Text>
          <Typography.Text type="secondary">{record.endAt ? dayjs(record.endAt).format('YYYY-MM-DD HH:mm') : t('common.none')}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.rules.columns.status'),
      dataIndex: 'status',
      render: (value: ActivityRewardRuleStatus) => <Tag color={ruleStatusColorMap[value]}>{t(`activityRewardCenter.status.${value}`)}</Tag>,
    },
    {
      title: t('activityRewardCenter.rules.columns.actions'),
      key: 'actions',
      render: (_, record) => {
        if (!canManageStatus && !canCreate) {
          return null
        }

        const nextStatus = record.status === 'active' ? 'disabled' : 'active'
        return (
          <Space size="small" wrap>
            {canManageStatus ? (
              <Button
                type="link"
                size="small"
                disabled={statusMutation.isPending}
                onClick={() => statusMutation.mutate({ id: record.id, status: nextStatus })}
              >
                {record.status === 'active' ? t('activityRewardCenter.actions.disable') : t('activityRewardCenter.actions.activate')}
              </Button>
            ) : null}
            {canCreate ? (
              <Button
                type="link"
                size="small"
                disabled={record.status !== 'active'}
                onClick={() => openGenerateModal(record)}
              >
                {t('activityRewardCenter.actions.generate')}
              </Button>
            ) : null}
          </Space>
        )
      },
    },
  ]

  const recordColumns: ColumnsType<ActivityRewardRecord> = [
    {
      title: t('activityRewardCenter.records.columns.recordNo'),
      dataIndex: 'recordNo',
      render: (value: string, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text strong>{value}</Typography.Text>
          <Typography.Text type="secondary">{record.ruleName || t('common.none')}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.records.columns.user'),
      key: 'user',
      render: (_, record) => (
        <Space direction="vertical" size={2}>
          <Typography.Text>{`${t('activityRewardCenter.records.userId')}: ${record.userID ?? record.playerID ?? '-'}`}</Typography.Text>
          <Typography.Text type="secondary">{`${t('activityRewardCenter.records.agentId')}: ${record.agentID ?? '-'}`}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.records.columns.activity'),
      key: 'activity',
      render: (_, record) => (
        <Space size={[6, 6]} wrap>
          <Tag color="geekblue">{record.activityType ? t(`activityRewardCenter.activityType.${record.activityType}`) : t('common.none')}</Tag>
          <Tag>{t(`activityRewardCenter.rewardType.${record.rewardType}`)}</Tag>
        </Space>
      ),
    },
    {
      title: t('activityRewardCenter.records.columns.reward'),
      key: 'reward',
      render: (_, record) => `${record.rewardValue} ${record.currency}`,
    },
    {
      title: t('activityRewardCenter.records.columns.status'),
      dataIndex: 'status',
      render: (value: ActivityRewardRecordStatus) => <Tag color={recordStatusColorMap[value] ?? 'default'}>{t(`activityRewardCenter.recordStatus.${value}`)}</Tag>,
    },
    {
      title: t('activityRewardCenter.records.columns.grantedAt'),
      key: 'grantedAt',
      render: (_, record) => (record.grantedAt ? dayjs(record.grantedAt).format('YYYY-MM-DD HH:mm') : t('common.none')),
    },
    {
      title: t('activityRewardCenter.records.columns.remark'),
      dataIndex: 'remark',
      render: (value?: string) => value || t('common.none'),
    },
    {
      title: t('activityRewardCenter.records.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        canManageStatus ? (
          <Button type="link" size="small" disabled={record.status !== 'granted'} onClick={() => openReverseModal(record)}>
            {t('activityRewardCenter.actions.reverse')}
          </Button>
        ) : null
      ),
    },
  ]

  const handleCreate = async () => {
    const values = await form.validateFields()
    createRuleMutation.mutate({
      ...scopeParams,
      name: values.name,
      activityType: values.activityType,
      rewardType: values.rewardType,
      status: values.status,
      rewardValue: values.rewardValue,
      currency: values.currency,
      triggerValue: values.triggerValue,
      dailyLimit: values.dailyLimit,
      totalLimit: values.totalLimit,
      startAt: values.schedule?.[0]?.toISOString(),
      endAt: values.schedule?.[1]?.toISOString(),
      remark: values.remark,
    })
  }

  const handleGenerate = async () => {
    if (!selectedRule) {
      return
    }

    const values = await generateForm.validateFields()
    generateRecordMutation.mutate({
      ruleID: selectedRule.id,
      payload: values,
    })
  }

  const handleReverse = async () => {
    if (!selectedRecord) {
      return
    }

    const values = await reverseForm.validateFields()
    reverseRecordMutation.mutate({
      recordID: selectedRecord.id,
      payload: values,
    })
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="middle" className="app-page__hero" style={{ width: '100%' }}>
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {view === 'rules' ? t('activityRewardCenter.rules.title') : t('activityRewardCenter.records.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {view === 'rules' ? t('activityRewardCenter.section.rulesSubtitle') : t('activityRewardCenter.section.recordsSubtitle')}
            </Typography.Paragraph>
          </div>
          <PlatformScopeSummary />

          <Row gutter={[16, 16]}>
            <Col xs={24} sm={12} lg={6}>
              <Card bordered={false} className="app-card">
                <Statistic title={t('activityRewardCenter.stats.totalRules')} value={stats.totalRules} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card bordered={false} className="app-card">
                <Statistic title={t('activityRewardCenter.stats.activeRules')} value={stats.activeRules} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card bordered={false} className="app-card">
                <Statistic title={t('activityRewardCenter.stats.draftRules')} value={stats.draftRules} />
              </Card>
            </Col>
            <Col xs={24} sm={12} lg={6}>
              <Card bordered={false} className="app-card">
                <Statistic title={t('activityRewardCenter.stats.grantedRecords')} value={stats.grantedRecords} />
              </Card>
            </Col>
          </Row>

          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('activityRewardCenter.filters.keywordPlaceholder')}
                value={keyword}
                onChange={(event) => setParam('keyword', event.target.value)}
                onSearch={(value) => setParam('keyword', value)}
                style={{ width: 320 }}
              />
              {view === 'rules' ? (
                <>
                  <Select<ActivityRewardRuleStatus | undefined>
                    allowClear
                    placeholder={t('activityRewardCenter.filters.ruleStatus')}
                    value={ruleStatus}
                    onChange={(value) => setParam('ruleStatus', value)}
                    style={{ width: 180 }}
                    options={ruleStatuses.map((item) => ({ value: item, label: t(`activityRewardCenter.status.${item}`) }))}
                  />
                  <Select<string | undefined>
                    allowClear
                    placeholder={t('activityRewardCenter.filters.activityType')}
                    value={activityType}
                    onChange={(value) => setParam('activityType', value)}
                    style={{ width: 180 }}
                    options={activityTypeOptions.map((item) => ({ value: item, label: t(`activityRewardCenter.activityType.${item}`) }))}
                  />
                </>
              ) : (
                <Select<ActivityRewardRecordStatus | undefined>
                  allowClear
                  placeholder={t('activityRewardCenter.filters.recordStatus')}
                  value={recordStatus}
                  onChange={(value) => setParam('recordStatus', value)}
                  style={{ width: 180 }}
                  options={recordStatusOptions.map((item) => ({ value: item, label: t(`activityRewardCenter.recordStatus.${item}`) }))}
                />
              )}
            </div>
            <div className="app-toolbar__actions">
              {view === 'rules' && canCreate ? (
                <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsCreateOpen(true)}>
                  {t('activityRewardCenter.create')}
                </Button>
              ) : null}
              <Button
                icon={<ReloadOutlined />}
                onClick={() => (view === 'rules' ? rulesQuery.refetch() : recordsQuery.refetch())}
                loading={view === 'rules' ? rulesQuery.isFetching : recordsQuery.isFetching}
              >
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {rulesQuery.error ? <Alert type="error" showIcon message={t('activityRewardCenter.loadRulesError')} description={(rulesQuery.error as Error).message} /> : null}
      {recordsQuery.error ? <Alert type="error" showIcon message={t('activityRewardCenter.loadRecordsError')} description={(recordsQuery.error as Error).message} /> : null}
      {createRuleMutation.error ? <Alert type="error" showIcon message={t('activityRewardCenter.createError')} description={(createRuleMutation.error as Error).message} /> : null}
      {statusMutation.error ? <Alert type="error" showIcon message={t('activityRewardCenter.statusError')} description={(statusMutation.error as Error).message} /> : null}
      {generateRecordMutation.error ? <Alert type="error" showIcon message={t('activityRewardCenter.generateError')} description={(generateRecordMutation.error as Error).message} /> : null}
      {reverseRecordMutation.error ? <Alert type="error" showIcon message={t('activityRewardCenter.reverseError')} description={(reverseRecordMutation.error as Error).message} /> : null}

      <Card
        className="app-table-card"
        title={view === 'rules' ? t('activityRewardCenter.rules.title') : t('activityRewardCenter.records.title')}
        extra={
          <Typography.Text type="secondary">
            {view === 'rules'
              ? t('activityRewardCenter.rules.count', { count: rules.length })
              : t('activityRewardCenter.records.count', { count: records.length })}
          </Typography.Text>
        }
      >
        <div className="app-table app-table--compact">
          {view === 'rules' ? (
            <Table<ActivityRewardRule>
              rowKey="id"
              loading={rulesQuery.isLoading || rulesQuery.isFetching}
              dataSource={rules}
              columns={ruleColumns}
              pagination={{ pageSize: 10, showSizeChanger: false }}
              locale={{ emptyText: <Empty description={t('activityRewardCenter.rules.empty')} /> }}
            />
          ) : (
            <Table<ActivityRewardRecord>
              rowKey="id"
              loading={recordsQuery.isLoading || recordsQuery.isFetching}
              dataSource={records}
              columns={recordColumns}
              pagination={{ pageSize: 10, showSizeChanger: false }}
              locale={{ emptyText: <Empty description={t('activityRewardCenter.records.empty')} /> }}
            />
          )}
        </div>
      </Card>

      <Modal
        open={isCreateOpen}
        title={t('activityRewardCenter.createModalTitle')}
        onCancel={() => setIsCreateOpen(false)}
        onOk={() => void handleCreate()}
        confirmLoading={createRuleMutation.isPending}
        destroyOnHidden
        width={720}
      >
        <Form<RewardRuleFormValues>
          form={form}
          layout="vertical"
          initialValues={{
            status: 'draft',
            activityType: 'login',
            rewardType: 'cash',
            currency: 'CNY',
          }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="name" label={t('activityRewardCenter.form.name')} rules={[{ required: true, message: t('activityRewardCenter.form.nameRequired') }]}>
                <Input placeholder={t('activityRewardCenter.form.namePlaceholder')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="status" label={t('activityRewardCenter.form.status')} rules={[{ required: true, message: t('activityRewardCenter.form.statusRequired') }]}>
                <Select options={ruleStatuses.map((item) => ({ value: item, label: t(`activityRewardCenter.status.${item}`) }))} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="activityType" label={t('activityRewardCenter.form.activityType')} rules={[{ required: true, message: t('activityRewardCenter.form.activityTypeRequired') }]}>
                <Select options={activityTypeOptions.map((item) => ({ value: item, label: t(`activityRewardCenter.activityType.${item}`) }))} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="rewardType" label={t('activityRewardCenter.form.rewardType')} rules={[{ required: true, message: t('activityRewardCenter.form.rewardTypeRequired') }]}>
                <Select options={rewardTypeOptions.map((item) => ({ value: item, label: t(`activityRewardCenter.rewardType.${item}`) }))} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="rewardValue" label={t('activityRewardCenter.form.rewardValue')} rules={[{ required: true, message: t('activityRewardCenter.form.rewardValueRequired') }]}>
                <InputNumber min={0} precision={2} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="currency" label={t('activityRewardCenter.form.currency')} rules={[{ required: true, message: t('activityRewardCenter.form.currencyRequired') }]}>
                <Input />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="triggerValue" label={t('activityRewardCenter.form.triggerValue')}>
                <InputNumber min={0} precision={2} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="dailyLimit" label={t('activityRewardCenter.form.dailyLimit')}>
                <InputNumber min={0} precision={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="totalLimit" label={t('activityRewardCenter.form.totalLimit')}>
                <InputNumber min={0} precision={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="schedule" label={t('activityRewardCenter.form.schedule')}>
                <DatePicker.RangePicker showTime style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item name="remark" label={t('activityRewardCenter.form.remark')}>
                <Input.TextArea rows={4} placeholder={t('activityRewardCenter.form.remarkPlaceholder')} />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      <Modal
        open={isGenerateOpen && !!selectedRule}
        title={t('activityRewardCenter.generateModalTitle')}
        onCancel={() => {
          setIsGenerateOpen(false)
          setSelectedRule(null)
          generateForm.resetFields()
        }}
        onOk={() => void handleGenerate()}
        confirmLoading={generateRecordMutation.isPending}
        destroyOnHidden
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {selectedRule ? (
            <Card size="small">
              <Descriptions size="small" column={1}>
                <Descriptions.Item label={t('activityRewardCenter.rules.columns.rule')}>{selectedRule.name}</Descriptions.Item>
                <Descriptions.Item label={t('activityRewardCenter.rules.columns.reward')}>{selectedRule.rewardValue} {selectedRule.currency}</Descriptions.Item>
                <Descriptions.Item label={t('activityRewardCenter.rules.columns.status')}>{t(`activityRewardCenter.status.${selectedRule.status}`)}</Descriptions.Item>
              </Descriptions>
            </Card>
          ) : null}
          <Form<GenerateRecordFormValues>
            form={generateForm}
            layout="vertical"
            initialValues={{ referenceType: 'manual' }}
          >
            <Form.Item name="playerID" label={t('activityRewardCenter.generateForm.playerId')} rules={[{ required: true, message: t('activityRewardCenter.generateForm.playerIdRequired') }]}>
              <InputNumber min={1} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="referenceID" label={t('activityRewardCenter.generateForm.referenceId')} rules={[{ required: true, message: t('activityRewardCenter.generateForm.referenceIdRequired') }]}>
              <Input placeholder={t('activityRewardCenter.generateForm.referenceIdPlaceholder')} />
            </Form.Item>
            <Form.Item name="agentID" label={t('activityRewardCenter.generateForm.agentId')}>
              <AgentSelect tenantID={scopeParams.tenantID} brandID={scopeParams.brandID} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item name="referenceType" label={t('activityRewardCenter.generateForm.referenceType')}>
              <Input placeholder={t('activityRewardCenter.generateForm.referenceTypePlaceholder')} />
            </Form.Item>
            <Form.Item name="remark" label={t('activityRewardCenter.generateForm.remark')}>
              <Input.TextArea rows={3} placeholder={t('activityRewardCenter.generateForm.remarkPlaceholder')} />
            </Form.Item>
          </Form>
        </Space>
      </Modal>

      <Modal
        open={isReverseOpen && !!selectedRecord}
        title={t('activityRewardCenter.reverseModalTitle')}
        onCancel={() => {
          setIsReverseOpen(false)
          setSelectedRecord(null)
          reverseForm.resetFields()
        }}
        onOk={() => void handleReverse()}
        confirmLoading={reverseRecordMutation.isPending}
        destroyOnHidden
      >
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {selectedRecord ? (
            <Card size="small">
              <Descriptions size="small" column={1}>
                <Descriptions.Item label={t('activityRewardCenter.records.columns.recordNo')}>{selectedRecord.recordNo}</Descriptions.Item>
                <Descriptions.Item label={t('activityRewardCenter.records.columns.reward')}>{selectedRecord.rewardValue} {selectedRecord.currency}</Descriptions.Item>
                <Descriptions.Item label={t('activityRewardCenter.records.columns.status')}>{t(`activityRewardCenter.recordStatus.${selectedRecord.status}`)}</Descriptions.Item>
              </Descriptions>
            </Card>
          ) : null}
          <Form<ReverseRecordFormValues> form={reverseForm} layout="vertical">
            <Form.Item name="remark" label={t('activityRewardCenter.reverseForm.remark')}>
              <Input.TextArea rows={3} placeholder={t('activityRewardCenter.reverseForm.remarkPlaceholder')} />
            </Form.Item>
          </Form>
        </Space>
      </Modal>
    </Space>
  )
}
