import { ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Empty, InputNumber, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'
import { BindingHistory, BindingStatus, apiClient } from '../../lib/api'

const statusColorMap: Record<BindingStatus, string> = {
  bound: 'success',
  pending_change: 'processing',
  changed: 'warning',
  released: 'default',
}

export default function BindingHistoryPage() {
  const { t } = useI18n()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const [playerID, setPlayerID] = useState<number | undefined>()
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['binding-history', scopeParams.tenantID, scopeParams.brandID, playerID],
    queryFn: () => apiClient.listBindingHistory({ ...scopeParams, playerID }),
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])

  const columns: ColumnsType<BindingHistory> = [
    {
      title: t('bindingHistory.columns.bindingId'),
      dataIndex: 'bindingID',
    },
    {
      title: t('bindingHistory.columns.playerId'),
      dataIndex: 'playerID',
    },
    {
      title: t('bindingHistory.columns.fromAgentId'),
      dataIndex: 'fromAgentID',
      render: (value) => value ?? t('common.none'),
    },
    {
      title: t('bindingHistory.columns.toAgentId'),
      dataIndex: 'toAgentID',
    },
    {
      title: t('bindingHistory.columns.status'),
      dataIndex: 'status',
      render: (value: BindingStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    {
      title: t('bindingHistory.columns.source'),
      dataIndex: 'source',
      render: (value) => t(`bindings.source.${value}`),
    },
    {
      title: t('bindingHistory.columns.changedAt'),
      dataIndex: 'changedAt',
    },
    {
      title: t('bindingHistory.columns.changedBy'),
      dataIndex: 'changedBy',
      render: (value) => value || t('common.none'),
    },
    {
      title: t('bindingHistory.columns.reason'),
      dataIndex: 'changeReason',
      render: (value) => value || t('common.none'),
    },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('bindingHistory.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('bindingHistory.subtitle')}
            </Typography.Paragraph>
          </div>
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <InputNumber
                min={1}
                value={playerID}
                onChange={(value) => setPlayerID(value ?? undefined)}
                placeholder={t('bindingHistory.filterPlaceholder')}
                style={{ width: 240 }}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('bindingHistory.loadError')} description={(query.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('bindingHistory.tableTitle')} extra={<Typography.Text type="secondary">{data.length} {t('bindingHistory.results')}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<BindingHistory>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('bindingHistory.empty')} /> }}
          />
        </div>
      </Card>
    </Space>
  )
}
