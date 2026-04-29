import { ReloadOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Empty, Input, Space, Table, Tag, Typography } from 'antd'
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { Player, apiClient } from '../../lib/api'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

export default function UsersPage() {
  const { t } = useI18n()
  const { activeTenantId, activeBrandId } = usePlatformScope()
  const [keyword, setKeyword] = useState('')
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])

  const query = useQuery({
    queryKey: ['players', scopeParams.tenantID, scopeParams.brandID, keyword],
    queryFn: () => apiClient.listPlayers({ ...scopeParams, keyword: keyword || undefined }),
  })

  const data = useMemo(() => query.data?.items ?? [], [query.data?.items])
  const registeredCount = useMemo(() => data.filter((item) => Boolean(item.registeredAt)).length, [data])
  const activeCount = useMemo(() => data.filter((item) => item.status === 'active').length, [data])

  const columns: ColumnsType<Player> = [
    {
      title: t('users.columns.player'),
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.nickname || t('common.none')}</Typography.Text>
          <Typography.Text type="secondary">{record.playerNo}</Typography.Text>
        </Space>
      ),
    },
    { title: t('users.columns.platformUserId'), dataIndex: 'platformUserID' },
    { title: t('users.columns.phone'), dataIndex: 'phone', render: (value) => value || t('common.none') },
    { title: t('users.columns.country'), dataIndex: 'countryCode', render: (value) => value || t('common.none') },
    { title: t('users.columns.currency'), dataIndex: 'currency' },
    { title: t('users.columns.status'), dataIndex: 'status', render: (value: string) => <Tag color={value === 'active' ? 'success' : 'default'}>{t(`status.${value}`)}</Tag> },
    { title: t('users.columns.registeredAt'), dataIndex: 'registeredAt' },
    { title: t('users.columns.createdAt'), dataIndex: 'createdAt' },
  ]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card" bordered={false}>
        <Space direction="vertical" size="middle" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('users.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('users.subtitle')}
            </Typography.Paragraph>
          </div>
          <Space wrap size={[12, 12]}>
            <Tag color="blue">{t('users.summary.accounts', { count: data.length })}</Tag>
            <Tag color="success">{t('users.summary.active', { count: activeCount })}</Tag>
            <Tag>{t('users.summary.registered', { count: registeredCount })}</Tag>
          </Space>
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('users.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
            </div>
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={() => query.refetch()} loading={query.isFetching}>{t('common.refresh')}</Button>
            </div>
          </div>
        </Space>
      </Card>

      {query.error ? <Alert type="error" showIcon message={t('users.loadError')} description={(query.error as Error).message} /> : null}

      <Card className="app-table-card" title={t('users.tableTitle')} extra={<Typography.Text type="secondary">{t('users.results', { count: data.length })}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Player>
            rowKey="id"
            loading={query.isLoading || query.isFetching}
            dataSource={data}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('users.empty')} /> }}
          />
        </div>
      </Card>
    </Space>
  )
}
