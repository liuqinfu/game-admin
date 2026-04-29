import { ReloadOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, DatePicker, Form, Input, Select, Space, Table, Tag, Typography } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery } from '@tanstack/react-query'
import dayjs from 'dayjs'
import type { ColumnsType } from 'antd/es/table'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../../auth'
import { AgentSelect } from '../../components/entity-selects'
import { useI18n } from '../../i18n'
import { CreateRecalculationTaskPayload, RecalculationScope, RecalculationTask, RecalculationTaskStatus, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const taskScopeOptions: RecalculationScope[] = ['all', 'agent', 'settlement_bill', 'period']
const taskTypeOptions: Array<NonNullable<CreateRecalculationTaskPayload['taskType']>> = ['settlement_bill', 'commission']

const statusColorMap: Record<string, string> = {
  pending: 'default',
  processing: 'processing',
  completed: 'success',
  failed: 'error',
}

const statusOptions: RecalculationTaskStatus[] = ['pending', 'processing', 'completed', 'failed']

type RecalculationTaskDraftState = {
  createTaskDraft?: CreateRecalculationTaskPayload
}

export default function RecalculationTasksPage() {
  const { t } = useI18n()
  const location = useLocation()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const [form] = Form.useForm<CreateRecalculationTaskPayload>()
  const [status, setStatus] = useState<RecalculationTaskStatus | undefined>()
  const [taskNoKeyword, setTaskNoKeyword] = useState('')
  const canCreateTask = hasPermission('recalculation-tasks:create')
  const scope = Form.useWatch('scope', form)
  const taskType = Form.useWatch('taskType', form)
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['recalculation-tasks', scopeParams.tenantID, scopeParams.brandID, status, taskNoKeyword],
    queryFn: () => apiClient.listRecalculationTasks({ ...scopeParams, status, taskNo: taskNoKeyword || undefined }),
  })

  useEffect(() => {
    const draft = (location.state as RecalculationTaskDraftState | null)?.createTaskDraft
    if (!draft || !canCreateTask) {
      return
    }
    form.setFieldsValue({
      taskType: draft.taskType ?? 'settlement_bill',
      scope: draft.scope ?? 'settlement_bill',
      agentID: draft.agentID,
      settlementBillID: draft.settlementBillID,
      periodStart: draft.periodStart,
      periodEnd: draft.periodEnd,
      remark: draft.remark,
    })
  }, [canCreateTask, form, location.state])

  const createMutation = useMutation({
    mutationFn: (payload: CreateRecalculationTaskPayload) => apiClient.createRecalculationTask(payload),
    onSuccess: () => {
      message.success(t('recalculationTasks.createSuccess'))
      form.resetFields()
      void query.refetch()
    },
  })

  const columns: ColumnsType<RecalculationTask> = useMemo(() => [
    { title: t('recalculationTasks.columns.id'), dataIndex: 'id', width: 80 },
    { title: t('recalculationTasks.columns.scope'), dataIndex: 'scope', width: 140, render: (value) => value ? <Tag>{t(`recalculationTasks.scope.${value}`)}</Tag> : t('common.none') },
    { title: t('recalculationTasks.columns.status'), dataIndex: 'status', width: 120, render: (value) => <Tag color={statusColorMap[value] ?? 'default'}>{t(`recalculationTasks.status.${value}`)}</Tag> },
    { title: t('recalculationTasks.columns.taskType'), dataIndex: 'taskType', width: 140, render: (value) => <Tag color="blue">{t(`recalculationTasks.taskType.${value}`)}</Tag> },
    { title: t('recalculationTasks.columns.taskNo'), dataIndex: 'taskNo', width: 180 },
    {
      title: t('recalculationTasks.columns.agent'),
      key: 'agent',
      width: 160,
      render: (_, record) => record.agentName || (record.agentID ? `#${record.agentID}` : t('common.none')),
    },
    { title: t('recalculationTasks.columns.agentId'), dataIndex: 'agentID', width: 120, render: (value) => value || t('common.none') },
    {
      title: t('recalculationTasks.columns.settlementBill'),
      key: 'settlementBill',
      width: 180,
      render: (_, record) => record.billNo || (record.settlementBillID ? `#${record.settlementBillID}` : t('common.none')),
    },
    { title: t('recalculationTasks.columns.settlementBillId'), dataIndex: 'settlementBillID', width: 150, render: (value) => value || t('common.none') },
    {
      title: t('recalculationTasks.columns.period'),
      key: 'period',
      width: 240,
      render: (_, record) => record.periodStart && record.periodEnd ? `${record.periodStart} ~ ${record.periodEnd}` : t('common.none'),
    },
    { title: t('recalculationTasks.columns.periodStart'), dataIndex: 'periodStart', width: 140, render: (value) => value || t('common.none') },
    { title: t('recalculationTasks.columns.periodEnd'), dataIndex: 'periodEnd', width: 140, render: (value) => value || t('common.none') },
    { title: t('recalculationTasks.columns.requestedBy'), dataIndex: 'requestedBy', width: 140, render: (value) => value || t('common.none') },
    { title: t('recalculationTasks.columns.startedAt'), dataIndex: 'startedAt', width: 180, render: (value) => value || t('common.none') },
    { title: t('recalculationTasks.columns.completedAt'), dataIndex: 'completedAt', width: 180, render: (value) => value || t('common.none') },
    { title: t('recalculationTasks.columns.remark'), dataIndex: 'remark', render: (value) => value || t('common.none') },
    { title: t('recalculationTasks.columns.createdAt'), dataIndex: 'createdAt', width: 180 },
    { title: t('recalculationTasks.columns.updatedAt'), dataIndex: 'updatedAt', width: 180 },
  ], [t])

  const submit = async () => {
    if (!scopeParams.tenantID || !scopeParams.brandID) {
      message.warning(t('recalculationTasks.scopeRequiredHint'))
      return
    }
    const values = await form.validateFields()
    const payload: CreateRecalculationTaskPayload = {
      taskType: values.taskType,
      scope: values.scope,
      agentID: values.agentID ? Number(values.agentID) : undefined,
      settlementBillID: values.settlementBillID ? Number(values.settlementBillID) : undefined,
      reason: values.reason?.trim() || undefined,
      remark: values.remark?.trim() || undefined,
      periodStart: values.periodStart,
      periodEnd: values.periodEnd,
    }
    createMutation.mutate(payload)
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">{t('recalculationTasks.title')}</Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">{t('recalculationTasks.subtitle')}</Typography.Paragraph>
          </div>
          <ScopeNotice requireBrand />
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('recalculationTasks.loadError')} description={(query.error as Error).message} /> : null}
      {createMutation.error ? <Alert type="error" showIcon message={t('recalculationTasks.createError')} description={(createMutation.error as Error).message} /> : null}

      <Card title={t('recalculationTasks.createTitle')}>
        {!canCreateTask ? <Alert type="info" showIcon message={t('recalculationTasks.permissionHint')} style={{ marginBottom: 16 }} /> : null}
        <Form form={form} layout="vertical" initialValues={{ scope: 'all', taskType: 'settlement_bill' }} disabled={!canCreateTask}>
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space wrap align="start">
              <Form.Item label={t('recalculationTasks.form.taskType')} name="taskType" rules={[{ required: true }]} style={{ minWidth: 220, marginBottom: 0 }}>
                <Select options={taskTypeOptions.map((item) => ({ label: t(`recalculationTasks.taskType.${item}`), value: item }))} />
              </Form.Item>
              <Form.Item label={t('recalculationTasks.form.scope')} name="scope" rules={[{ required: true }]} style={{ minWidth: 220, marginBottom: 0 }}>
                <Select options={taskScopeOptions.map((item) => ({ label: t(`recalculationTasks.scope.${item}`), value: item }))} />
              </Form.Item>

              {scope === 'agent' ? (
                <Form.Item label={t('recalculationTasks.form.agentId')} name="agentID" rules={[{ required: true }]} style={{ minWidth: 180, marginBottom: 0 }}>
                  <AgentSelect tenantID={scopeParams.tenantID} brandID={scopeParams.brandID} style={{ width: '100%' }} placeholder={t('recalculationTasks.form.agentIdPlaceholder')} />
                </Form.Item>
              ) : null}

              {scope === 'settlement_bill' ? (
                <Form.Item label={t('recalculationTasks.form.settlementBillId')} name="settlementBillID" rules={[{ required: true }]} style={{ minWidth: 180, marginBottom: 0 }}>
                  <Input placeholder={t('recalculationTasks.form.settlementBillIdPlaceholder')} />
                </Form.Item>
              ) : null}

              {scope === 'period' ? (
                <>
                  <Form.Item label={t('recalculationTasks.form.periodStart')} name="periodStart" rules={[{ required: true }]} style={{ minWidth: 180, marginBottom: 0 }}>
                    <DatePicker style={{ width: '100%' }} format="YYYY-MM-DD" onChange={(value) => form.setFieldValue('periodStart', value ? dayjs(value).format('YYYY-MM-DD') : undefined)} />
                  </Form.Item>
                  <Form.Item label={t('recalculationTasks.form.periodEnd')} name="periodEnd" rules={[{ required: true }]} style={{ minWidth: 180, marginBottom: 0 }}>
                    <DatePicker style={{ width: '100%' }} format="YYYY-MM-DD" onChange={(value) => form.setFieldValue('periodEnd', value ? dayjs(value).format('YYYY-MM-DD') : undefined)} />
                  </Form.Item>
                </>
              ) : null}
            </Space>

            <Form.Item label={t('recalculationTasks.form.reason')} name="reason" rules={[{ required: taskType === 'settlement_bill', whitespace: true }]} style={{ marginBottom: 0 }}>
              <Input.TextArea rows={2} placeholder={t('recalculationTasks.form.reasonPlaceholder')} />
            </Form.Item>

            <Form.Item label={t('recalculationTasks.form.remark')} name="remark" style={{ marginBottom: 0 }}>
              <Input.TextArea rows={3} placeholder={t('recalculationTasks.form.remarkPlaceholder')} />
            </Form.Item>

            <Space>
              <Button type="primary" onClick={() => void submit()} loading={createMutation.isPending} disabled={!canCreateTask}>{t('recalculationTasks.create')}</Button>
              <Button onClick={() => form.resetFields()} disabled={!canCreateTask}>{t('common.reset')}</Button>
            </Space>
          </Space>
        </Form>
      </Card>

      <Card title={t('recalculationTasks.filtersTitle')}>
        <Space wrap>
          <Input
            allowClear
            value={taskNoKeyword}
            onChange={(event) => setTaskNoKeyword(event.target.value)}
            placeholder={t('recalculationTasks.form.taskNoPlaceholder')}
            style={{ width: 240 }}
          />
          <Select
            allowClear
            value={status}
            onChange={(value) => setStatus(value)}
            placeholder={t('recalculationTasks.form.statusPlaceholder')}
            style={{ width: 220 }}
            options={statusOptions.map((item) => ({ label: t(`recalculationTasks.status.${item}`), value: item }))}
          />
          <Button onClick={() => { setStatus(undefined); setTaskNoKeyword('') }}>{t('common.reset')}</Button>
        </Space>
      </Card>

      <Card title={t('recalculationTasks.tableTitle')} extra={<Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>}>
        <Table<RecalculationTask>
          rowKey="id"
          loading={query.isLoading || query.isFetching}
          columns={columns}
          dataSource={query.data?.items ?? []}
          pagination={{ pageSize: 10, showSizeChanger: false }}
          scroll={{ x: 1200 }}
        />
      </Card>
    </Space>
  )
}
