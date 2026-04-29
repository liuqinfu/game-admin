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
  Modal,
  Progress,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useAuth } from '../../auth'
import {
  buildOperatingParamDefaults,
  findOperatingTemplate,
  findScopedOperatingConfigs,
  operatingParamTemplates,
  validateOperatingConfig,
} from '../../config/operating-params'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import {
  apiClient,
  type Brand,
  type PlatformConfig,
  type PlatformConfigPayload,
  type Tenant,
} from '../../lib/api'
import { PlatformScopeSummary } from '../../platform-scope/PlatformScopeSummary'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

type PlatformConfigFormValues = {
  tenantID?: number
  brandID?: number
  key: string
  valueText: string
  description?: string
  templateKey?: string
}

function safeStringify(value: Record<string, unknown>) {
  return JSON.stringify(value, null, 2)
}

export default function PlatformConfigCenterPage() {
  const { t } = useI18n()
  const { hasPermission } = useAuth()
  const { message } = App.useApp()
  const queryClient = useQueryClient()
  const {
    activeTenantId,
    activeBrandId,
    tenants,
    brands,
    setActiveTenantId,
    setActiveBrandId,
  } = usePlatformScope()

  const [keyword, setKeyword] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editingItem, setEditingItem] = useState<PlatformConfig | null>(null)
  const [form] = Form.useForm<PlatformConfigFormValues>()

  const canWrite = hasPermission('platform-configs:write')

  const platformConfigsQuery = useQuery({
    queryKey: ['platform-configs', activeTenantId, activeBrandId],
    queryFn: async () => {
      const tenantCode = activeTenantId ? tenants.find((item) => item.id === activeTenantId)?.code : undefined
      return apiClient.listPlatformConfigs({ tenantID: activeTenantId, brandID: activeBrandId, tenantCode })
    },
    enabled: !activeTenantId || tenants.length > 0,
  })

  const tenantMap = useMemo(
    () => tenants.reduce<Record<number, Tenant>>((acc, tenant) => {
      acc[tenant.id] = tenant
      return acc
    }, {}),
    [tenants],
  )

  const brandMap = useMemo(
    () => brands.reduce<Record<number, Brand>>((acc, brand) => {
      acc[brand.id] = brand
      return acc
    }, {}),
    [brands],
  )

  const submitMutation = useMutation({
    mutationFn: ({ id, payload }: { id?: number; payload: PlatformConfigPayload }) =>
      id ? apiClient.updatePlatformConfig(id, payload) : apiClient.createPlatformConfig(payload),
    onSuccess: () => {
      message.success(editingItem ? t('platformConfigCenter.updateSuccess') : t('platformConfigCenter.createSuccess'))
      setModalOpen(false)
      setEditingItem(null)
      form.resetFields()
      queryClient.invalidateQueries({ queryKey: ['platform-configs'] })
    },
  })

  const filteredItems = useMemo(() => {
    const items = platformConfigsQuery.data?.items ?? []
    return items.filter((item) => {
      if (!keyword.trim()) {
        return true
      }
      const normalized = keyword.trim().toLowerCase()
      return [item.key, item.description, item.tenantCode, item.brandCode, safeStringify(item.value)]
        .filter(Boolean)
        .some((value) => String(value).toLowerCase().includes(normalized))
    })
  }, [keyword, platformConfigsQuery.data?.items])

  const scopedOperatingConfigs = useMemo(
    () => findScopedOperatingConfigs(platformConfigsQuery.data?.items ?? []),
    [platformConfigsQuery.data?.items],
  )

  const configuredOperatingCount = scopedOperatingConfigs.filter((item) => item.config).length
  const operatingCoverage = operatingParamTemplates.length > 0
    ? Math.round((configuredOperatingCount / operatingParamTemplates.length) * 100)
    : 0

  useEffect(() => {
    if (!modalOpen) {
      return
    }

    if (editingItem) {
      form.setFieldsValue({
        tenantID: editingItem.tenantID ?? undefined,
        brandID: editingItem.brandID ?? undefined,
        key: editingItem.key,
        valueText: safeStringify(editingItem.value),
        description: editingItem.description,
        templateKey: editingItem.key,
      })
      return
    }

    form.setFieldsValue({
      tenantID: activeTenantId,
      brandID: activeBrandId,
      key: form.getFieldValue('key') || '',
      valueText: form.getFieldValue('valueText') || '{\n  "enabled": true\n}',
      description: form.getFieldValue('description') || '',
      templateKey: form.getFieldValue('templateKey'),
    })
  }, [activeBrandId, activeTenantId, editingItem, form, modalOpen])

  const handleCreate = () => {
    setEditingItem(null)
    setModalOpen(true)
  }

  const handleEdit = (item: PlatformConfig) => {
    setEditingItem(item)
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await form.validateFields()
    let parsedValue: Record<string, unknown>

    try {
      parsedValue = JSON.parse(values.valueText)
    } catch {
      message.error(t('platformConfigCenter.form.valueInvalid'))
      return
    }

    const payload: PlatformConfigPayload = {
      tenantID: values.tenantID,
      brandID: values.brandID,
      key: values.key.trim(),
      value: parsedValue,
      description: values.description?.trim(),
    }

    const matchedTemplate = findOperatingTemplate(payload.key)
    if (matchedTemplate) {
      const issues = validateOperatingConfig(matchedTemplate, payload.value)
      if (issues.length > 0) {
        const firstIssue = issues[0]
        const field = matchedTemplate.fields.find((item) => item.key === firstIssue.fieldKey)
        message.error(
          field
            ? t('platformConfigCenter.operating.validationErrorWithField', {
                field: t(field.labelKey),
                reason: t(firstIssue.messageKey),
              })
            : t(firstIssue.messageKey),
        )
        return
      }
    }

    submitMutation.mutate({ id: editingItem?.id, payload })
  }

  const columns: ColumnsType<PlatformConfig> = [
    {
      title: t('platformConfigCenter.columns.scope'),
      key: 'scope',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.tenantCode || t('platformConfigCenter.scope.platform')}</Typography.Text>
          <Typography.Text type="secondary">{record.brandCode || t('platformConfigCenter.scope.allBrands')}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('platformConfigCenter.columns.key'),
      dataIndex: 'key',
      render: (value: string, record) => (
        <Space direction="vertical" size={0}>
          <Space size={[6, 6]} wrap>
            <Typography.Text code>{value}</Typography.Text>
            {findOperatingTemplate(value) ? <Tag color="gold">{t('platformConfigCenter.operating.badge')}</Tag> : null}
          </Space>
          <Typography.Text type="secondary">{record.description || t('common.none')}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('platformConfigCenter.columns.value'),
      dataIndex: 'value',
      render: (value: Record<string, unknown>) => (
        <Typography.Paragraph
          code
          ellipsis={{ rows: 3, expandable: true, symbol: t('platformConfigCenter.expand') }}
          style={{ marginBottom: 0, maxWidth: 420, whiteSpace: 'pre-wrap' }}
        >
          {safeStringify(value)}
        </Typography.Paragraph>
      ),
    },
    {
      title: t('platformConfigCenter.columns.level'),
      key: 'level',
      render: (_, record) => {
        const hasBrand = Boolean(record.brandID)
        const hasTenant = Boolean(record.tenantID)
        const color = hasBrand ? 'purple' : hasTenant ? 'blue' : 'default'
        const label = hasBrand
          ? t('platformConfigCenter.level.brand')
          : hasTenant
            ? t('platformConfigCenter.level.tenant')
            : t('platformConfigCenter.level.platform')
        return <Tag color={color}>{label}</Tag>
      },
    },
    canWrite
      ? {
          title: t('platformConfigCenter.columns.actions'),
          key: 'actions',
          render: (_, record) => (
            <Button type="link" onClick={() => handleEdit(record)}>
              {t('platformConfigCenter.edit')}
            </Button>
          ),
        }
      : {
          title: t('platformConfigCenter.columns.actions'),
          key: 'actions',
          render: () => null,
        },
  ]

  const brandOptions = useMemo(() => {
    const visibleBrands = form.getFieldValue('tenantID')
      ? brands.filter((item) => item.tenantID === form.getFieldValue('tenantID'))
      : brands

    return visibleBrands.map((brand) => ({
      label: `${brand.name} (${brand.code})`,
      value: brand.id,
    }))
  }, [brands, form])

  const handleTemplateChange = (templateKey?: string) => {
    if (!templateKey) {
      form.setFieldsValue({
        key: '',
        valueText: '{\n  "enabled": true\n}',
        description: '',
      })
      return
    }

    const template = findOperatingTemplate(templateKey)
    if (!template) {
      return
    }

    form.setFieldsValue({
      key: template.key,
      valueText: safeStringify(buildOperatingParamDefaults(template)),
      description: t(template.descriptionKey),
    })
  }

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">
              {t('platformConfigCenter.title')}
            </Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">
              {t('platformConfigCenter.subtitle')}
            </Typography.Paragraph>
          </div>
          <PlatformScopeSummary />
          <div className="app-toolbar">
            <div className="app-toolbar__filters">
              <Input.Search
                allowClear
                placeholder={t('platformConfigCenter.searchPlaceholder')}
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                onSearch={(value) => setKeyword(value)}
                style={{ width: 320 }}
              />
            </div>
            <div className="app-toolbar__actions">
              {canWrite ? (
                <Button type="primary" onClick={handleCreate}>
                  {t('platformConfigCenter.create')}
                </Button>
              ) : null}
              <Button icon={<ReloadOutlined />} onClick={() => platformConfigsQuery.refetch()} loading={platformConfigsQuery.isFetching}>
                {t('common.refresh')}
              </Button>
            </div>
          </div>
        </Space>
      </Card>

      {platformConfigsQuery.error ? (
        <Alert type="error" showIcon message={t('platformConfigCenter.loadError')} description={(platformConfigsQuery.error as Error).message} />
      ) : null}
      {submitMutation.error ? (
        <Alert type="error" showIcon message={t('platformConfigCenter.submitError')} description={(submitMutation.error as Error).message} />
      ) : null}

      <Card className="app-card" bordered={false}>
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          <Space direction="vertical" size={2}>
            <Typography.Title level={4} style={{ margin: 0 }}>
              {t('platformConfigCenter.operating.title')}
            </Typography.Title>
            <Typography.Text type="secondary">
              {t('platformConfigCenter.operating.subtitle')}
            </Typography.Text>
          </Space>
          <div>
            <Space align="baseline" size="middle">
              <Typography.Text strong>
                {t('platformConfigCenter.operating.coverage', { count: configuredOperatingCount, total: operatingParamTemplates.length })}
              </Typography.Text>
              <Typography.Text type="secondary">{`${operatingCoverage}%`}</Typography.Text>
            </Space>
            <Progress percent={operatingCoverage} showInfo={false} strokeColor="#1677ff" />
          </div>
          {configuredOperatingCount < operatingParamTemplates.length ? (
            <Alert
              type="warning"
              showIcon
              message={t('platformConfigCenter.operating.missingAlertTitle')}
              description={t('platformConfigCenter.operating.missingAlertDescription', {
                names: scopedOperatingConfigs
                  .filter((item) => !item.config)
                  .map((item) => t(item.template.titleKey))
                  .join(' / '),
              })}
            />
          ) : null}
          <Row gutter={[16, 16]}>
            {scopedOperatingConfigs.map(({ template, config }) => (
              <Col key={template.key} xs={24} xl={12}>
                <Card
                  size="small"
                  title={t(template.titleKey)}
                  extra={<Tag color={config ? 'success' : 'default'}>{config ? t('platformConfigCenter.operating.configured') : t('platformConfigCenter.operating.missing')}</Tag>}
                  style={{ width: '100%' }}
                >
                  <Space direction="vertical" size={8} style={{ width: '100%' }}>
                    <Typography.Text type="secondary">{t(template.descriptionKey)}</Typography.Text>
                    <Typography.Text type="secondary">
                      {template.fields.map((field) => t(field.labelKey)).join(' / ')}
                    </Typography.Text>
                    {canWrite ? (
                      <Button onClick={() => {
                        if (config) {
                          handleEdit(config)
                          return
                        }
                        setEditingItem(null)
                        setModalOpen(true)
                        form.setFieldsValue({ tenantID: activeTenantId, brandID: activeBrandId, templateKey: template.key })
                        handleTemplateChange(template.key)
                      }}
                      >
                        {config ? t('platformConfigCenter.operating.viewTemplate') : t('platformConfigCenter.operating.useTemplate')}
                      </Button>
                    ) : null}
                  </Space>
                </Card>
              </Col>
            ))}
          </Row>
        </Space>
      </Card>

      <Card
        className="app-table-card"
        title={t('platformConfigCenter.tableTitle')}
        extra={<Typography.Text type="secondary">{t('platformConfigCenter.results', { count: filteredItems.length })}</Typography.Text>}
      >
        <div className="app-table app-table--compact">
          <Table<PlatformConfig>
            rowKey="id"
            loading={platformConfigsQuery.isLoading || platformConfigsQuery.isFetching}
            dataSource={filteredItems}
            columns={columns}
            pagination={{ pageSize: 10, showSizeChanger: false }}
            locale={{ emptyText: <Empty description={t('platformConfigCenter.empty')} /> }}
          />
        </div>
      </Card>

      <Modal
        title={editingItem ? t('platformConfigCenter.editModalTitle') : t('platformConfigCenter.createModalTitle')}
        open={modalOpen}
        onCancel={() => {
          setModalOpen(false)
          setEditingItem(null)
        }}
        onOk={handleSubmit}
        okText={t('common.save')}
        cancelText={t('common.cancel')}
        confirmLoading={submitMutation.isPending}
        destroyOnHidden
      >
        <Form form={form} layout="vertical">
          {!editingItem ? (
            <Form.Item name="templateKey" label={t('platformConfigCenter.operating.templateLabel')}>
              <Select
                allowClear
                placeholder={t('platformConfigCenter.operating.templatePlaceholder')}
                options={operatingParamTemplates.map((template) => ({
                  label: t(template.titleKey),
                  value: template.key,
                }))}
                onChange={handleTemplateChange}
              />
            </Form.Item>
          ) : null}
          <Form.Item name="tenantID" label={t('platformConfigCenter.form.tenantId')}>
            <Select
              allowClear
              showSearch
              placeholder={t('platformConfigCenter.form.tenantPlaceholder')}
              options={tenants.map((tenant) => ({
                label: `${tenant.name} (${tenant.code})`,
                value: tenant.id,
              }))}
              optionFilterProp="label"
              disabled={Boolean(editingItem)}
              onChange={(value) => {
                form.setFieldValue('brandID', undefined)
                setActiveTenantId(value)
                if (!value) {
                  setActiveBrandId(undefined)
                }
              }}
            />
          </Form.Item>
          <Form.Item name="brandID" label={t('platformConfigCenter.form.brandId')}>
            <Select
              allowClear
              showSearch
              placeholder={t('platformConfigCenter.form.brandPlaceholder')}
              options={brandOptions}
              optionFilterProp="label"
              disabled={Boolean(editingItem)}
              onChange={(value) => setActiveBrandId(value)}
            />
          </Form.Item>
          <Form.Item
            name="key"
            label={t('platformConfigCenter.form.key')}
            rules={[{ required: true, message: t('platformConfigCenter.form.keyRequired') }]}
          >
            <Input placeholder={t('platformConfigCenter.form.keyPlaceholder')} disabled={Boolean(editingItem)} />
          </Form.Item>
          <Form.Item
            name="valueText"
            label={t('platformConfigCenter.form.value')}
            rules={[{ required: true, message: t('platformConfigCenter.form.valueRequired') }]}
          >
            <Input.TextArea rows={8} placeholder={t('platformConfigCenter.form.valuePlaceholder')} />
          </Form.Item>
          <Form.Item name="description" label={t('platformConfigCenter.form.description')}>
            <Input.TextArea rows={3} placeholder={t('platformConfigCenter.form.descriptionPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
