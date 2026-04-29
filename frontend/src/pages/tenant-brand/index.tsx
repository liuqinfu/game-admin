import { DeleteOutlined, ReloadOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Col,
  Empty,
  Form,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
} from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../../auth'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { apiClient, type Brand, type BrandStatus, type Tenant, type TenantStatus } from '../../lib/api'
import { PlatformScopeSummary } from '../../platform-scope/PlatformScopeSummary'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

const tenantStatusOptions: TenantStatus[] = ['active', 'inactive', 'disabled']
const brandStatusOptions: BrandStatus[] = ['active', 'inactive', 'disabled']

const statusColorMap: Record<TenantStatus | BrandStatus, string> = {
  active: 'success',
  inactive: 'warning',
  disabled: 'default',
}

type TenantFormValues = {
  code: string
  name: string
  displayName?: string
  status: TenantStatus
  remark?: string
}

type BrandFormValues = {
  tenantID: number
  code: string
  name: string
  displayName?: string
  status: BrandStatus
  domain?: string
  isDefault: boolean
  remark?: string
}

function resolveTenantDeleteError(t: (key: string) => string, error: Error) {
  switch (error.message) {
    case 'tenant has active brands':
      return t('tenantBrand.tenants.deleteBlockedBrands')
    case 'tenant has active agents':
      return t('tenantBrand.tenants.deleteBlockedAgents')
    case 'tenant has active players':
      return t('tenantBrand.tenants.deleteBlockedPlayers')
    case 'tenant has active games':
      return t('tenantBrand.tenants.deleteBlockedGames')
    case 'tenant has active platform configs':
      return t('tenantBrand.tenants.deleteBlockedConfigs')
    case 'tenant has active commission rules':
      return t('tenantBrand.tenants.deleteBlockedRules')
    case 'tenant has active withdrawal requests':
      return t('tenantBrand.tenants.deleteBlockedWithdrawals')
    case 'tenant has active recharge orders':
      return t('tenantBrand.tenants.deleteBlockedOrders')
    case 'tenant has active commission records':
      return t('tenantBrand.tenants.deleteBlockedCommissions')
    default:
      return error.message || t('tenantBrand.tenants.deleteError')
  }
}

function resolveBrandDeleteError(t: (key: string) => string, error: Error) {
  switch (error.message) {
    case 'brand is tenant default':
      return t('tenantBrand.brands.deleteBlockedDefault')
    case 'brand has active agents':
      return t('tenantBrand.brands.deleteBlockedAgents')
    case 'brand has active players':
      return t('tenantBrand.brands.deleteBlockedPlayers')
    case 'brand has active games':
      return t('tenantBrand.brands.deleteBlockedGames')
    case 'brand has active platform configs':
      return t('tenantBrand.brands.deleteBlockedConfigs')
    case 'brand has active invite codes':
      return t('tenantBrand.brands.deleteBlockedInvites')
    case 'brand has active bindings':
      return t('tenantBrand.brands.deleteBlockedBindings')
    case 'brand has active withdrawal requests':
      return t('tenantBrand.brands.deleteBlockedWithdrawals')
    case 'brand has active recharge orders':
      return t('tenantBrand.brands.deleteBlockedOrders')
    case 'brand has active commission records':
      return t('tenantBrand.brands.deleteBlockedCommissions')
    case 'brand has active commission rules':
      return t('tenantBrand.brands.deleteBlockedRules')
    default:
      return error.message || t('tenantBrand.brands.deleteError')
  }
}

export default function TenantBrandManagementPage() {
  const { t } = useI18n()
  const location = useLocation()
  const { hasPermission } = useAuth()
  const {
    tenants: scopedTenants,
    brands: scopedBrands,
    activeTenantId,
    activeBrandId,
    setActiveTenantId,
    setActiveBrandId,
    scopeLevel,
    isTenantLocked,
    isBrandLocked,
  } = usePlatformScope()
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const [selectedTenantID, setSelectedTenantID] = useState<number | undefined>(activeTenantId)
  const [tenantModalOpen, setTenantModalOpen] = useState(false)
  const [brandModalOpen, setBrandModalOpen] = useState(false)
  const [tenantForm] = Form.useForm<TenantFormValues>()
  const [brandForm] = Form.useForm<BrandFormValues>()

  const canCreateTenant = scopeLevel === 'platform' && hasPermission('tenants:create')
  const canUpdateTenantStatus = scopeLevel === 'platform' && (hasPermission('tenants:status:update') || hasPermission('tenants:create'))
  const canDeleteTenant = scopeLevel === 'platform' && hasPermission('tenants:delete')
  const canCreateBrand = scopeLevel !== 'agent' && hasPermission('brands:create')
  const canUpdateBrandStatus = hasPermission('brands:status:update') || hasPermission('brands:create')
  const canDeleteBrand = scopeLevel !== 'agent' && hasPermission('brands:delete')
  const canViewTenants = hasPermission('tenants:view')
  const canViewBrands = hasPermission('brands:view')
  const section = useMemo<'tenants' | 'brands'>(() => (location.pathname.endsWith('/brands') ? 'brands' : 'tenants'), [location.pathname])

  const tenantsQuery = useQuery({
    queryKey: ['tenants'],
    queryFn: () => apiClient.listTenants(),
    enabled: canViewTenants && (section === 'tenants' || canCreateBrand || canViewBrands),
  })

  const brandsQuery = useQuery({
    queryKey: ['brands', selectedTenantID],
    queryFn: () => apiClient.listBrands({ tenantID: selectedTenantID }),
    enabled: canViewBrands && section === 'brands',
  })

  const tenants = useMemo(() => canViewTenants ? (tenantsQuery.data?.items ?? []) : scopedTenants, [canViewTenants, scopedTenants, tenantsQuery.data?.items])
  const brands = useMemo(() => {
    const items = canViewBrands ? (brandsQuery.data?.items ?? []) : scopedBrands
    return selectedTenantID ? items.filter((brand) => brand.tenantID === selectedTenantID) : items
  }, [canViewBrands, brandsQuery.data?.items, scopedBrands, selectedTenantID])

  useEffect(() => {
    setSelectedTenantID(activeTenantId)
  }, [activeTenantId])

  const tenantMap = useMemo(
    () => tenants.reduce<Record<number, Tenant>>((acc, tenant) => {
      acc[tenant.id] = tenant
      return acc
    }, {}),
    [tenants],
  )

  const summary = useMemo(() => ({
    tenants: tenants.length,
    activeTenants: tenants.filter((item) => item.status === 'active').length,
    brands: brands.length,
    defaultBrands: brands.filter((item) => item.isDefault).length,
  }), [brands, tenants])

  const createTenantMutation = useMutation({
    mutationFn: apiClient.createTenant,
    onSuccess: () => {
      message.success(t('tenantBrand.tenants.createSuccess'))
      setTenantModalOpen(false)
      tenantForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['tenants'] })
    },
    onError: (error) => {
      message.error((error as Error).message || t('tenantBrand.tenants.createError'))
    },
  })

  const updateTenantStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: TenantStatus }) => apiClient.updateTenantStatus(id, status),
    onSuccess: () => {
      message.success(t('tenantBrand.tenants.statusSuccess'))
      queryClient.invalidateQueries({ queryKey: ['tenants'] })
    },
    onError: (error) => {
      message.error((error as Error).message || t('tenantBrand.brands.createError'))
    },
  })

  const deleteTenantMutation = useMutation({
    mutationFn: apiClient.deleteTenant,
    onSuccess: (_, id) => {
      message.success(t('tenantBrand.tenants.deleteSuccess'))
      if (activeTenantId === id) {
        setActiveTenantId(undefined)
        setActiveBrandId(undefined)
        setSelectedTenantID(undefined)
      }
      queryClient.invalidateQueries({ queryKey: ['tenants'] })
      queryClient.invalidateQueries({ queryKey: ['brands'] })
    },
    onError: (error) => {
      message.error(resolveTenantDeleteError(t, error as Error))
    },
  })

  const createBrandMutation = useMutation({
    mutationFn: apiClient.createBrand,
    onSuccess: () => {
      message.success(t('tenantBrand.brands.createSuccess'))
      setBrandModalOpen(false)
      brandForm.resetFields()
      setActiveTenantId(undefined)
      setActiveBrandId(undefined)
      queryClient.invalidateQueries({ queryKey: ['brands'] })
      queryClient.invalidateQueries({ queryKey: ['tenants'] })
    },
  })

  const updateBrandStatusMutation = useMutation({
    mutationFn: ({ id, status }: { id: number; status: BrandStatus }) => apiClient.updateBrandStatus(id, status),
    onSuccess: () => {
      message.success(t('tenantBrand.brands.statusSuccess'))
      queryClient.invalidateQueries({ queryKey: ['brands'] })
    },
  })

  const deleteBrandMutation = useMutation({
    mutationFn: apiClient.deleteBrand,
    onSuccess: (_, id) => {
      message.success(t('tenantBrand.brands.deleteSuccess'))
      if (activeBrandId === id) {
        setActiveBrandId(undefined)
      }
      queryClient.invalidateQueries({ queryKey: ['brands'] })
      queryClient.invalidateQueries({ queryKey: ['tenants'] })
    },
    onError: (error) => {
      message.error(resolveBrandDeleteError(t, error as Error))
    },
  })

  const confirmDeleteTenant = (tenant: Tenant) => {
    Modal.confirm({
      title: t('tenantBrand.tenants.deleteConfirmTitle'),
      content: t('tenantBrand.tenants.deleteConfirmContent', { name: tenant.name }),
      okText: t('common.delete'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: () => deleteTenantMutation.mutateAsync(tenant.id),
    })
  }

  const confirmDeleteBrand = (brand: Brand) => {
    Modal.confirm({
      title: t('tenantBrand.brands.deleteConfirmTitle'),
      content: t('tenantBrand.brands.deleteConfirmContent', { name: brand.name }),
      okText: t('common.delete'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: () => deleteBrandMutation.mutateAsync(brand.id),
    })
  }

  const tenantColumns: ColumnsType<Tenant> = [
    {
      title: t('tenantBrand.tenants.columns.tenant'),
      key: 'tenant',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text type="secondary">{record.code}</Typography.Text>
        </Space>
      ),
    },
    { title: t('tenantBrand.tenants.columns.displayName'), dataIndex: 'displayName', render: (value) => value || t('common.none') },
    {
      title: t('tenantBrand.tenants.columns.status'),
      dataIndex: 'status',
      render: (value: TenantStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    {
      title: t('tenantBrand.tenants.columns.defaultBrandId'),
      dataIndex: 'defaultBrandID',
      render: (value) => value ?? t('common.none'),
    },
    { title: t('tenantBrand.tenants.columns.remark'), dataIndex: 'remark', render: (value) => value || t('common.none') },
    { title: t('tenantBrand.tenants.columns.createdAt'), dataIndex: 'createdAt' },
    {
      title: t('tenantBrand.tenants.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Space wrap className="app-actions-inline">
          <Select<TenantStatus>
            value={record.status}
            style={{ width: 150 }}
            disabled={!canUpdateTenantStatus}
            options={tenantStatusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
            loading={updateTenantStatusMutation.isPending}
            onChange={(nextStatus) => {
              if (nextStatus !== record.status) {
                updateTenantStatusMutation.mutate({ id: record.id, status: nextStatus })
              }
            }}
          />
          {canDeleteTenant ? (
            <Button danger icon={<DeleteOutlined />} loading={deleteTenantMutation.isPending} onClick={() => confirmDeleteTenant(record)}>
              {t('common.delete')}
            </Button>
          ) : null}
        </Space>
      ),
    },
  ]

  const brandColumns: ColumnsType<Brand> = [
    {
      title: t('tenantBrand.brands.columns.brand'),
      key: 'brand',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text type="secondary">{record.code}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('tenantBrand.brands.columns.tenant'),
      dataIndex: 'tenantID',
      render: (value: number) => tenantMap[value]?.name || `#${value}`,
    },
    { title: t('tenantBrand.brands.columns.displayName'), dataIndex: 'displayName', render: (value) => value || t('common.none') },
    {
      title: t('tenantBrand.brands.columns.status'),
      dataIndex: 'status',
      render: (value: BrandStatus) => <Tag color={statusColorMap[value]}>{t(`status.${value}`)}</Tag>,
    },
    { title: t('tenantBrand.brands.columns.domain'), dataIndex: 'domain', render: (value) => value || t('common.none') },
    {
      title: t('tenantBrand.brands.columns.default'),
      dataIndex: 'isDefault',
      render: (value: boolean) => <Tag color={value ? 'success' : 'default'}>{value ? t('common.yes') : t('common.no')}</Tag>,
    },
    { title: t('tenantBrand.brands.columns.createdAt'), dataIndex: 'createdAt' },
    {
      title: t('tenantBrand.brands.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Space wrap className="app-actions-inline">
          <Select<BrandStatus>
            value={record.status}
            style={{ width: 150 }}
            disabled={!canUpdateBrandStatus}
            options={brandStatusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))}
            loading={updateBrandStatusMutation.isPending}
            onChange={(nextStatus) => {
              if (nextStatus !== record.status) {
                updateBrandStatusMutation.mutate({ id: record.id, status: nextStatus })
              }
            }}
          />
          {canDeleteBrand ? (
            <Button danger icon={<DeleteOutlined />} loading={deleteBrandMutation.isPending} onClick={() => confirmDeleteBrand(record)}>
              {t('common.delete')}
            </Button>
          ) : null}
        </Space>
      ),
    },
  ]

  const handleCreateTenant = async () => {
    const values = await tenantForm.validateFields()
    createTenantMutation.mutate({
      code: values.code.trim(),
      name: values.name.trim(),
      displayName: values.displayName?.trim(),
      status: values.status,
      remark: values.remark?.trim(),
    })
  }

  const handleCreateBrand = async () => {
    const values = await brandForm.validateFields()
    createBrandMutation.mutate({
      tenantID: values.tenantID,
      code: values.code.trim(),
      name: values.name.trim(),
      displayName: values.displayName?.trim(),
      status: values.status,
      domain: values.domain?.trim(),
      isDefault: values.isDefault,
      remark: values.remark?.trim(),
    })
  }

  const sectionTitle = section === 'tenants' ? t('tenantBrand.tenants.cardTitle') : t('tenantBrand.brands.cardTitle')
  const sectionSubtitle = section === 'tenants' ? t('tenantBrand.section.tenantsSubtitle') : t('tenantBrand.section.brandsSubtitle')

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {sectionTitle}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {sectionSubtitle}
            </Typography.Paragraph>
          </div>
          <PlatformScopeSummary />
          <Alert
            type={scopeLevel === 'platform' ? 'info' : 'warning'}
            showIcon
            message={t(`tenantBrand.saasGuide.${scopeLevel}`)}
            description={t('tenantBrand.saasGuide.description')}
          />
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('tenantBrand.stats.tenants')} value={summary.tenants} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('tenantBrand.stats.activeTenants')} value={summary.activeTenants} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('tenantBrand.stats.brands')} value={summary.brands} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('tenantBrand.stats.defaultBrands')} value={summary.defaultBrands} /></Card></Col>
          </Row>
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Select<number | undefined>
                allowClear
                style={{ width: 260 }}
                placeholder={t('tenantBrand.brands.filterTenantPlaceholder')}
                value={selectedTenantID}
                onChange={(value) => {
                  setSelectedTenantID(value)
                  setActiveTenantId(value)
                }}
                options={tenants.map((tenant) => ({ label: `${tenant.name} (${tenant.code})`, value: tenant.id }))}
              />
            </div>
            <div className="app-toolbar__actions">
              {section === 'tenants' && canCreateTenant ? (
                <Button
                  type="primary"
                  onClick={() => {
                    tenantForm.setFieldsValue({ status: 'active' })
                    setTenantModalOpen(true)
                  }}
                >
                  {t('tenantBrand.tenants.create')}
                </Button>
              ) : null}
              {section === 'brands' && canCreateBrand ? (
                <Button
                  onClick={() => {
                    brandForm.setFieldsValue({ tenantID: selectedTenantID ?? activeTenantId, status: 'active', isDefault: false })
                    setBrandModalOpen(true)
                  }}
                >
                  {t('tenantBrand.brands.create')}
                </Button>
              ) : null}
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  if (canViewTenants) {
                    void tenantsQuery.refetch()
                  }
                  if (canViewBrands) {
                    void brandsQuery.refetch()
                  }
                }}
                loading={(canViewTenants && tenantsQuery.isFetching) || (canViewBrands && brandsQuery.isFetching)}
              >
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {canViewTenants && tenantsQuery.error ? <Alert type="error" showIcon message={t('tenantBrand.tenants.loadError')} description={(tenantsQuery.error as Error).message} /> : null}
      {canViewBrands && brandsQuery.error ? <Alert type="error" showIcon message={t('tenantBrand.brands.loadError')} description={(brandsQuery.error as Error).message} /> : null}
      {createTenantMutation.error ? <Alert type="error" showIcon message={t('tenantBrand.tenants.createError')} description={(createTenantMutation.error as Error).message} /> : null}
      {updateTenantStatusMutation.error ? <Alert type="error" showIcon message={t('tenantBrand.tenants.statusError')} description={(updateTenantStatusMutation.error as Error).message} /> : null}
      {deleteTenantMutation.error ? <Alert type="error" showIcon message={t('tenantBrand.tenants.deleteError')} description={resolveTenantDeleteError(t, deleteTenantMutation.error as Error)} /> : null}
      {createBrandMutation.error ? <Alert type="error" showIcon message={t('tenantBrand.brands.createError')} description={(createBrandMutation.error as Error).message} /> : null}
      {updateBrandStatusMutation.error ? <Alert type="error" showIcon message={t('tenantBrand.brands.statusError')} description={(updateBrandStatusMutation.error as Error).message} /> : null}
      {deleteBrandMutation.error ? <Alert type="error" showIcon message={t('tenantBrand.brands.deleteError')} description={resolveBrandDeleteError(t, deleteBrandMutation.error as Error)} /> : null}

      {section === 'tenants' ? <Card className="app-table-card" title={t('tenantBrand.tenants.cardTitle')} extra={<Typography.Text type="secondary">{tenants.length} {t('tenantBrand.tenants.results')}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<Tenant>
            rowKey="id"
            loading={canViewTenants && (tenantsQuery.isLoading || tenantsQuery.isFetching)}
            dataSource={tenants}
            columns={tenantColumns}
            pagination={{ pageSize: 8, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('tenantBrand.tenants.empty')} /> }}
          />
        </div>
      </Card> : null}

      {section === 'brands' ? <Card className="app-table-card" title={t('tenantBrand.brands.cardTitle')} extra={<Typography.Text type="secondary">{brands.length} {t('tenantBrand.brands.results')}</Typography.Text>}>
        <div className="app-table app-table--compact">
          {activeTenantId || activeBrandId ? (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 16 }}
              message={t('tenantBrand.scopeHint')}
            />
          ) : null}
          <Table<Brand>
            rowKey="id"
            loading={canViewBrands && (brandsQuery.isLoading || brandsQuery.isFetching)}
            dataSource={brands}
            columns={brandColumns}
            pagination={{ pageSize: 8, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('tenantBrand.brands.empty')} /> }}
          />
        </div>
      </Card> : null}

      <Modal
        className="app-modal-form"
        title={t('tenantBrand.tenants.modalTitle')}
        open={canCreateTenant && tenantModalOpen}
        onCancel={() => {
          setTenantModalOpen(false)
          tenantForm.resetFields()
        }}
        onOk={() => void handleCreateTenant()}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
        confirmLoading={createTenantMutation.isPending}
        destroyOnClose
      >
        <Form<TenantFormValues> className="app-form" form={tenantForm} layout="vertical" initialValues={{ status: 'active' }}>
          <Form.Item label={t('tenantBrand.tenants.form.code')} name="code" rules={[{ required: true, message: t('tenantBrand.tenants.form.codeRequired') }]}>
            <Input placeholder={t('tenantBrand.tenants.form.codePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.tenants.form.name')} name="name" rules={[{ required: true, message: t('tenantBrand.tenants.form.nameRequired') }]}>
            <Input placeholder={t('tenantBrand.tenants.form.namePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.tenants.form.displayName')} name="displayName">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.tenants.form.status')} name="status" rules={[{ required: true, message: t('tenantBrand.tenants.form.statusRequired') }]}>
            <Select options={tenantStatusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.tenants.form.remark')} name="remark">
            <Input.TextArea rows={3} placeholder={t('common.optional')} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        className="app-modal-form"
        title={t('tenantBrand.brands.modalTitle')}
        open={canCreateBrand && brandModalOpen}
        onCancel={() => {
          setBrandModalOpen(false)
          brandForm.resetFields()
        }}
        onOk={() => void handleCreateBrand()}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
        confirmLoading={createBrandMutation.isPending}
        destroyOnClose
      >
        <Form<BrandFormValues> className="app-form" form={brandForm} layout="vertical" initialValues={{ status: 'active', isDefault: false }}>
          <Form.Item label={t('tenantBrand.brands.form.tenantId')} name="tenantID" rules={[{ required: true, message: t('tenantBrand.brands.form.tenantIdRequired') }]}>
            <Select options={tenants.map((tenant) => ({ label: `${tenant.name} (${tenant.code})`, value: tenant.id }))} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.code')} name="code" rules={[{ required: true, message: t('tenantBrand.brands.form.codeRequired') }]}>
            <Input placeholder={t('tenantBrand.brands.form.codePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.name')} name="name" rules={[{ required: true, message: t('tenantBrand.brands.form.nameRequired') }]}>
            <Input placeholder={t('tenantBrand.brands.form.namePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.displayName')} name="displayName">
            <Input placeholder={t('common.optional')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.status')} name="status" rules={[{ required: true, message: t('tenantBrand.brands.form.statusRequired') }]}>
            <Select options={brandStatusOptions.map((item) => ({ label: t(`status.${item}`), value: item }))} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.domain')} name="domain">
            <Input placeholder={t('tenantBrand.brands.form.domainPlaceholder')} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.isDefault')} name="isDefault" rules={[{ required: true, message: t('tenantBrand.brands.form.isDefaultRequired') }]}>
            <Select options={[{ label: t('common.no'), value: false }, { label: t('common.yes'), value: true }]} />
          </Form.Item>
          <Form.Item label={t('tenantBrand.brands.form.remark')} name="remark">
            <Input.TextArea rows={3} placeholder={t('common.optional')} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
