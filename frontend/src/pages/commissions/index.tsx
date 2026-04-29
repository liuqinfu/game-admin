import { ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Empty, Input, Select, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useI18n } from '../../i18n'
import { CommissionRecord, CommissionStatus, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const statusOptions: CommissionStatus[] = ['pending', 'frozen', 'settled', 'reversed', 'voided']

const statusColorMap: Record<CommissionStatus, string> = {
  pending: 'processing',
  frozen: 'warning',
  settled: 'success',
  reversed: 'default',
  voided: 'error',
}

export default function CommissionsPage() {
  const { t } = useI18n()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const [status, setStatus] = useState<CommissionStatus | undefined>()
  const [keyword, setKeyword] = useState('')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['commissions', scopeParams.tenantID, scopeParams.brandID, status],
    queryFn: () => apiClient.listCommissions({ ...scopeParams, status }),
  })

  const data = useMemo(() => {
    const items = query.data?.items ?? []
    if (!keyword.trim()) {
      return items
    }

    const normalized = keyword.trim().toLowerCase()
    return items.filter((item) =>
      [item.rechargeOrderID, item.playerID, item.agentID, item.ruleID, item.currency]
        .filter((value) => value !== undefined && value !== null)
        .some((value) => String(value).toLowerCase().includes(normalized)),
    )
  }, [keyword, query.data?.items])

  const columns: ColumnsType<CommissionRecord> = [
    { title: t('commissions.columns.orderId'), dataIndex: 'rechargeOrderID' },
    { title: t('commissions.columns.playerId'), dataIndex: 'playerID' },
    { title: t('commissions.columns.agentId'), dataIndex: 'agentID' },
    { title: t('commissions.columns.ruleId'), dataIndex: 'ruleID' },
    { title: t('commissions.columns.gameId'), dataIndex: 'gameID' },
    { title: t('commissions.columns.level'), dataIndex: 'settlementDepth' },
    { title: t('commissions.columns.baseAmount'), render: (_, record) => `${record.commissionBaseAmount} ${record.currency}` },
    { title: t('commissions.columns.commissionAmount'), render: (_, record) => `${record.commissionAmount} ${record.currency}` },
    {
      title: t('commissions.columns.status'),
      dataIndex: 'status',
      render: (value: CommissionStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    { title: t('commissions.columns.createdAt'), dataIndex: 'createdAt' },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('commissions.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('commissions.subtitle')}
            </Typography.Paragraph>
          </div>
          <ScopeNotice />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('commissions.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<CommissionStatus | undefined>
                allowClear
                placeholder={t('commissions.statusPlaceholder')}
                value={status}
                onChange={(value) => setStatus(value)}
                style={{ width: 180 }}
                options={statusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('commissions.loadError')} description={(query.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('commissions.tableTitle')} extra={<Typography.Text type="secondary">{t('commissions.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<CommissionRecord>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('commissions.empty')} /> }}
          />
        </div>
      </Card>
    </Space>
  )
}
