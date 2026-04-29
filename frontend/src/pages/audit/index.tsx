import { ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Empty, Input, Select, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useI18n } from '../../i18n'
import { AuditLog, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const moduleOptions = ['agent', 'binding', 'game', 'rule', 'settlement', 'commission', 'account', 'tenant', 'brand', 'platform_config', 'risk', 'rbac', 'withdrawal', 'activity_reward'] as const
const actionOptions = [
  'create',
  'update',
  'publish',
  'invite_audit',
  'tenant_create',
  'tenant_update_status',
  'brand_create',
  'brand_update_status',
  'platform_config_create',
  'withdrawal_review',
  'withdrawal_payout',
  'settlement_bill_confirm',
  'settlement_bill_export',
  'risk_case_review',
] as const

export default function AuditPage() {
  const { t } = useI18n()
  const { scopeLevel } = usePlatformScope()
  const [module, setModule] = useState<string | undefined>()
  const [action, setAction] = useState<string | undefined>()
  const [keyword, setKeyword] = useState('')
  const targetID = keyword.trim() || undefined

  const query = useQuery({
    queryKey: ['audit', scopeLevel, module, action, targetID],
    queryFn: () => apiClient.listAuditLogs({ module, action, targetID }),
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])

  const getAuditModuleLabel = (value: string) => {
    const translated = t(`audit.module.${value}`)
    return translated === `audit.module.${value}` ? value : translated
  }

  const getAuditActionLabel = (value: string) => {
    const translated = t(`audit.action.${value}`)
    return translated === `audit.action.${value}` ? value : translated
  }

  const getAuditTargetTypeLabel = (value: string) => {
    const translated = t(`audit.targetType.${value}`)
    return translated === `audit.targetType.${value}` ? value : translated
  }

  const columns: ColumnsType<AuditLog> = [
    { title: t('audit.columns.operator'), dataIndex: 'operatorName', render: (value, record) => value || record.operatorID },
    { title: t('audit.columns.module'), dataIndex: 'module', render: (value: string) => getAuditModuleLabel(value) },
    {
      title: t('audit.columns.action'),
      dataIndex: 'action',
      render: (value: string) => <Tag>{getAuditActionLabel(value)}</Tag>,
    },
    { title: t('audit.columns.target'), render: (_, record) => `${getAuditTargetTypeLabel(record.targetType)} #${record.targetID}` },
    { title: t('audit.columns.result'), dataIndex: 'result', render: (value: string) => <Tag color={value === 'success' ? 'success' : 'error'}>{t(`audit.result.${value}`)}</Tag> },
    { title: t('audit.columns.requestId'), dataIndex: 'requestID', render: (value) => value || t('common.none') },
    { title: t('audit.columns.createdAt'), dataIndex: 'createdAt' },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card" bordered={false}>
        <Space direction="vertical" size="middle" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('audit.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('audit.subtitle')}
            </Typography.Paragraph>
          </div>
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('audit.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<string | undefined>
                allowClear
                placeholder={t('audit.modulePlaceholder')}
                value={module}
                onChange={(value) => setModule(value)}
                style={{ width: 180 }}
                options={moduleOptions.map((item) => ({ label: t(`audit.module.${item}`), value: item }))}
              />
              <Select<string | undefined>
                allowClear
                placeholder={t('audit.actionPlaceholder')}
                value={action}
                onChange={(value) => setAction(value)}
                style={{ width: 180 }}
                options={actionOptions.map((item) => ({ label: t(`audit.action.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('audit.loadError')} description={(query.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('audit.tableTitle')} extra={<Typography.Text type="secondary">{t('audit.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table">
          <Table<AuditLog>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('audit.empty')} /> }}
          />
        </div>
      </Card>
    </Space>
  )
}
