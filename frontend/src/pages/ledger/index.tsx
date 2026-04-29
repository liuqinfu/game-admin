import { ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Empty, Input, Select, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useI18n } from '../../i18n'
import { LedgerEntry, LedgerType, apiClient } from '../../lib/api'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const typeOptions: LedgerType[] = ['income', 'freeze', 'unfreeze', 'debit', 'reverse', 'adjust', 'commission_income', 'commission_reverse', 'manual_adjust']

const typeColorMap: Record<LedgerType, string> = {
  income: 'success',
  freeze: 'warning',
  unfreeze: 'processing',
  debit: 'error',
  reverse: 'purple',
  adjust: 'default',
  commission_income: 'success',
  commission_reverse: 'warning',
  manual_adjust: 'processing',
}

export default function LedgerPage() {
  const { t } = useI18n()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const [type, setType] = useState<LedgerType | undefined>()
  const [keyword, setKeyword] = useState('')
  const orderNo = keyword.trim() || undefined
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['ledger', scopeParams.tenantID, scopeParams.brandID, type, orderNo],
    queryFn: () => apiClient.listLedger({ ...scopeParams, type, orderNo }),
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])

  const columns: ColumnsType<LedgerEntry> = [
    { title: t('ledger.columns.agentId'), dataIndex: 'agentID' },
    { title: t('ledger.columns.reference'), dataIndex: 'referenceID' },
    {
      title: t('ledger.columns.type'),
      dataIndex: 'ledgerType',
      render: (value: LedgerType) => <Tag color={typeColorMap[value] ?? 'default'}>{t(`ledger.type.${value}`)}</Tag>,
    },
    { title: t('ledger.columns.amount'), render: (_, record) => `${record.amount} ${record.currency}` },
    { title: t('ledger.columns.balanceAfter'), render: (_, record) => `${record.balanceAfter} ${record.currency}` },
    { title: t('ledger.columns.remark'), dataIndex: 'remark', render: (value) => value || t('common.none') },
    { title: t('ledger.columns.createdAt'), dataIndex: 'createdAt' },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <Card className="app-card" bordered={false}>
        <Space direction="vertical" size="middle" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('ledger.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('ledger.subtitle')}
            </Typography.Paragraph>
          </div>
          <ScopeNotice />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('ledger.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
              <Select<LedgerType | undefined>
                allowClear
                placeholder={t('ledger.typePlaceholder')}
                value={type}
                onChange={(value) => setType(value)}
                style={{ width: 220 }}
                options={typeOptions.map((item) => ({ label: t(`ledger.type.${item}`), value: item }))}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('ledger.loadError')} description={(query.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('ledger.tableTitle')} extra={<Typography.Text type="secondary">{t('ledger.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table">
          <Table<LedgerEntry>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('ledger.empty')} /> }}
          />
        </div>
      </Card>
    </Space>
  )
}
