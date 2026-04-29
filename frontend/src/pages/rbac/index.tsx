import { EditOutlined, ReloadOutlined, TeamOutlined, UserAddOutlined } from '@ant-design/icons'
import {
  Alert,
  App,
  Button,
  Card,
  Checkbox,
  Col,
  Collapse,
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
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ColumnsType } from 'antd/es/table'
import { useLocation } from 'react-router-dom'
import { useAuth } from '../../auth'
import { AgentSelect, BrandSelect, TenantSelect } from '../../components/entity-selects'
import { labelKeyForBackendPermission } from '../../config/permission-meta'
import { permissionGroups } from '../../config/permission-taxonomy'
import { useI18n } from '../../i18n'
import { ScopeNotice } from '../../platform-scope/ScopeNotice'
import { apiClient, type RbacPermission, type RbacRole, type RbacRolePayload, type RbacUser, type RbacUserPayload } from '../../lib/api'
import type { PlatformScopeLevel } from '../../platform-scope/PlatformScopeProvider'
import { usePlatformScope } from '../../platform-scope/usePlatformScope'

type RoleFormValues = {
  scopeLevel: PlatformScopeLevel
  tenantID?: number | null
  brandID?: number | null
  code: string
  name: string
  permissions: string[]
}

type UserFormValues = {
  scopeLevel: PlatformScopeLevel
  username: string
  password?: string
  displayName?: string
  status: 'active' | 'disabled'
  tenantID?: number | null
  brandID?: number | null
  agentID?: number | null
  roles: string[]
}


function getScopeLevelFromValues(values: { tenantID?: number | null; brandID?: number | null; agentID?: number | null }): PlatformScopeLevel {
  if (values.agentID) {
    return 'agent'
  }
  if (values.brandID) {
    return 'brand'
  }
  if (values.tenantID) {
    return 'tenant'
  }
  return 'platform'
}

function getScopePayload(values: { scopeLevel?: PlatformScopeLevel; tenantID?: number | null; brandID?: number | null; agentID?: number | null }) {
  const scopeLevel = values.scopeLevel ?? getScopeLevelFromValues(values)
  return {
    tenantID: scopeLevel === 'platform' ? null : (values.tenantID ?? null),
    brandID: scopeLevel === 'platform' || scopeLevel === 'tenant' ? null : (values.brandID ?? null),
    agentID: scopeLevel === 'agent' ? (values.agentID ?? null) : null,
  }
}

function roleMatchesUserScope(role: RbacRole, tenantID?: number | null, brandID?: number | null) {
  if (!tenantID) {
    return !role.tenantID && !role.brandID
  }
  if (role.tenantID !== tenantID) {
    return false
  }
  if (!brandID) {
    return !role.brandID
  }
  return !role.brandID || role.brandID === brandID
}

function getPermissionLabel(permission: RbacPermission, t: (key: string) => string) {
  const key = labelKeyForBackendPermission(permission.code)
  if (key) {
    return t(key)
  }
  return permission.name?.trim() || permission.code
}

function getRoleLabel(role: RbacRole) {
  return `${role.name} (${role.code})`
}

function getPermissionCountText(selected: number, total: number, t: (key: string) => string) {
  return t('rbac.permissions.selectedCount').replace('{selected}', String(selected)).replace('{total}', String(total))
}

export default function RbacPage() {
  const { t } = useI18n()
  const location = useLocation()
  const { message } = App.useApp()
  const { hasPermission } = useAuth()
  const { activeTenantId, activeBrandId, scopeLevel: currentScopeLevel, isTenantLocked, isBrandLocked } = usePlatformScope()
  const queryClient = useQueryClient()
  const [roleForm] = Form.useForm<RoleFormValues>()
  const [userForm] = Form.useForm<UserFormValues>()
  const [editingRole, setEditingRole] = useState<RbacRole | null>(null)
  const [editingUser, setEditingUser] = useState<RbacUser | null>(null)
  const [roleModalOpen, setRoleModalOpen] = useState(false)
  const [userModalOpen, setUserModalOpen] = useState(false)
  const section = useMemo<'users' | 'roles' | 'permissions'>(() => {
    const value = location.pathname.split('/')[2]
    if (value === 'roles' || value === 'permissions') {
      return value
    }
    return 'users'
  }, [location.pathname])

  const permissionsQuery = useQuery({
    queryKey: ['rbac', 'permissions'],
    queryFn: () => apiClient.listRbacPermissions(),
  })

  const rolesQuery = useQuery({
    queryKey: ['rbac', 'roles'],
    queryFn: () => apiClient.listRbacRoles(),
  })

  const usersQuery = useQuery({
    queryKey: ['rbac', 'users'],
    queryFn: () => apiClient.listRbacUsers(),
    enabled: hasPermission('rbac:users:view'),
  })

  const saveRoleMutation = useMutation({
    mutationFn: (payload: { id?: number; body: RbacRolePayload }) => {
      if (payload.id) {
        return apiClient.updateRbacRole(payload.id, payload.body)
      }
      return apiClient.createRbacRole(payload.body)
    },
    onSuccess: () => {
      message.success(editingRole ? t('rbac.roles.updateSuccess') : t('rbac.roles.createSuccess'))
      setRoleModalOpen(false)
      setEditingRole(null)
      roleForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['rbac', 'roles'] })
    },
  })

  const saveUserMutation = useMutation({
    mutationFn: (payload: { id?: number; body: RbacUserPayload }) => {
      if (payload.id) {
        return apiClient.updateRbacUser(payload.id, payload.body)
      }
      return apiClient.createRbacUser(payload.body)
    },
    onSuccess: () => {
      message.success(editingUser ? t('rbac.users.updateSuccess') : t('rbac.users.createSuccess'))
      setUserModalOpen(false)
      setEditingUser(null)
      userForm.resetFields()
      queryClient.invalidateQueries({ queryKey: ['rbac', 'users'] })
    },
  })

  const deleteRoleMutation = useMutation({
    mutationFn: apiClient.deleteRbacRole,
    onSuccess: () => {
      message.success(t('rbac.roles.deleteSuccess'))
      queryClient.invalidateQueries({ queryKey: ['rbac', 'roles'] })
    },
  })

  const deleteUserMutation = useMutation({
    mutationFn: apiClient.deleteRbacUser,
    onSuccess: () => {
      message.success(t('rbac.users.deleteSuccess'))
      queryClient.invalidateQueries({ queryKey: ['rbac', 'users'] })
    },
  })

  const permissions = useMemo(() => permissionsQuery.data?.items ?? [], [permissionsQuery.data?.items])
  const roles = useMemo(() => rolesQuery.data?.items ?? [], [rolesQuery.data?.items])
  const users = useMemo(() => [...(usersQuery.data?.items ?? [])].sort((left, right) => new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime()), [usersQuery.data?.items])
  const scopeParams = useMemo(() => ({ tenantID: activeTenantId, brandID: activeBrandId }), [activeBrandId, activeTenantId])
  const rolePermissionCount = useMemo(() => roles.reduce((acc, role) => acc + role.permissions.length, 0), [roles])
  const assignedRoleCount = useMemo(() => users.reduce((acc, user) => acc + user.roles.length, 0), [users])
  const selectedRolePermissions = Form.useWatch('permissions', roleForm) ?? []
  const selectedRoleScopeLevel = Form.useWatch('scopeLevel', roleForm) ?? currentScopeLevel
  const selectedRoleTenantID = Form.useWatch('tenantID', roleForm)
  const selectedUserScopeLevel = Form.useWatch('scopeLevel', userForm) ?? currentScopeLevel
  const selectedUserTenantID = Form.useWatch('tenantID', userForm)
  const selectedUserBrandID = Form.useWatch('brandID', userForm)
  const roleOptions = useMemo(() => roles.filter((role) => roleMatchesUserScope(role, selectedUserTenantID, selectedUserBrandID)).map((role) => ({ label: `${getRoleLabel(role)} · ${role.tenantID ? `T${role.tenantID} / B${role.brandID ?? '-'}` : t('rbac.scope.platform')}`, value: role.code })), [roles, selectedUserBrandID, selectedUserTenantID, t])
  const permissionMap = useMemo(
    () => permissions.reduce<Record<string, RbacPermission>>((acc, permission) => {
      acc[permission.code] = permission
      return acc
    }, {}),
    [permissions],
  )
  const groupedPermissions = useMemo(
    () => {
      const groupedCodes = new Set(permissionGroups.flatMap((group) => group.menus.flatMap((menu) => menu.codes)))
      const knownGroups = permissionGroups
        .map((group) => ({
          ...group,
          menus: group.menus
            .map((menu) => ({
              ...menu,
              permissions: menu.codes.map((code) => permissionMap[code]).filter((permission): permission is RbacPermission => Boolean(permission)),
            }))
            .filter((menu) => menu.permissions.length > 0),
        }))
        .filter((group) => group.menus.length > 0)
      const uncategorizedPermissions = permissions.filter((permission) => !groupedCodes.has(permission.code))

      if (uncategorizedPermissions.length === 0) {
        return knownGroups
      }

      return [
        ...knownGroups,
        {
          key: 'uncategorized',
          labelKey: 'businessGroup.uncategorized',
          menus: [{ labelKey: 'rbac.permissions.uncategorizedMenu', codes: uncategorizedPermissions.map((permission) => permission.code), permissions: uncategorizedPermissions }],
        },
      ]
    },
    [permissionMap, permissions],
  )
  const roleMap = useMemo(
    () => roles.reduce<Record<string, RbacRole>>((acc, role) => {
      acc[role.code] = role
      return acc
    }, {}),
    [roles],
  )

  const refreshAll = () => {
    void permissionsQuery.refetch()
    void rolesQuery.refetch()
    if (hasPermission('rbac:users:view')) {
      void usersQuery.refetch()
    }
  }

  const openCreateRoleModal = () => {
    setEditingRole(null)
    roleForm.setFieldsValue({ scopeLevel: currentScopeLevel, tenantID: activeTenantId ?? null, brandID: activeBrandId ?? null, code: '', name: '', permissions: [] })
    setRoleModalOpen(true)
  }

  const openEditRoleModal = (role: RbacRole) => {
    setEditingRole(role)
    roleForm.setFieldsValue({ scopeLevel: getScopeLevelFromValues(role), tenantID: role.tenantID ?? null, brandID: role.brandID ?? null, code: role.code, name: role.name, permissions: role.permissions })
    setRoleModalOpen(true)
  }

  const openCreateUserModal = () => {
    setEditingUser(null)
    userForm.setFieldsValue({ scopeLevel: currentScopeLevel, username: '', password: '', displayName: '', status: 'active', tenantID: activeTenantId ?? null, brandID: activeBrandId ?? null, agentID: null, roles: [] })
    setUserModalOpen(true)
  }

  const openEditUserModal = (user: RbacUser) => {
    setEditingUser(user)
    userForm.setFieldsValue({ scopeLevel: getScopeLevelFromValues(user), username: user.username, password: '', displayName: user.displayName, status: user.status, tenantID: user.tenantID ?? null, brandID: user.brandID ?? null, agentID: user.agentID, roles: user.roles })
    setUserModalOpen(true)
  }

  const closeRoleModal = () => {
    setRoleModalOpen(false)
    setEditingRole(null)
    roleForm.resetFields()
  }

  const closeUserModal = () => {
    setUserModalOpen(false)
    setEditingUser(null)
    userForm.resetFields()
  }

  const confirmDeleteRole = (role: RbacRole) => {
    Modal.confirm({
      title: t('rbac.roles.deleteConfirmTitle'),
      content: t('rbac.roles.deleteConfirmContent'),
      okText: t('common.delete'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: () => deleteRoleMutation.mutateAsync(role.id),
    })
  }

  const confirmDeleteUser = (user: RbacUser) => {
    Modal.confirm({
      title: t('rbac.users.deleteConfirmTitle'),
      content: t('rbac.users.deleteConfirmContent'),
      okText: t('common.delete'),
      cancelText: t('common.cancel'),
      okButtonProps: { danger: true },
      onOk: () => deleteUserMutation.mutateAsync(user.id),
    })
  }

  const handleRoleSubmit = async () => {
    const values = await roleForm.validateFields()
    const scopePayload = getScopePayload(values)
    saveRoleMutation.mutate({
      id: editingRole?.id,
      body: { tenantID: scopePayload.tenantID, brandID: scopePayload.brandID, code: values.code.trim(), name: values.name.trim(), permissions: values.permissions ?? [] },
    })
  }

  const handleUserSubmit = async () => {
    const values = await userForm.validateFields()
    const scopePayload = getScopePayload(values)
    const agentIDValue = scopePayload.agentID ? Number(scopePayload.agentID) : null
    saveUserMutation.mutate({
      id: editingUser?.id,
      body: {
        username: values.username.trim(),
        password: values.password?.trim(),
        displayName: values.displayName?.trim(),
        status: values.status,
        tenantID: scopePayload.tenantID,
        brandID: scopePayload.brandID,
        agentID: agentIDValue && Number.isFinite(agentIDValue) ? agentIDValue : null,
        roles: values.roles ?? [],
      },
    })
  }

  const userColumns: ColumnsType<RbacUser> = [
    {
      title: t('rbac.users.columns.user'),
      key: 'user',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.displayName || record.username}</Typography.Text>
          <Typography.Text type="secondary">{record.username}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('rbac.users.columns.status'),
      dataIndex: 'status',
      render: (value: RbacUser['status']) => <Tag color={value === 'active' ? 'green' : 'default'}>{t(`rbac.users.status.${value}`)}</Tag>,
    },
    {
      title: t('rbac.users.columns.roles'),
      dataIndex: 'roles',
      render: (value: string[]) => (
        <Space size={[4, 4]} wrap>
          {value.length > 0 ? value.map((code) => <Tag key={code} color="geekblue">{roleMap[code]?.name ?? code}</Tag>) : <Typography.Text type="secondary">{t('common.none')}</Typography.Text>}
        </Space>
      ),
    },
    {
      title: t('rbac.users.columns.scope'),
      key: 'scope',
      render: (_, record) => record.agentID ? <Tag color="purple">Agent #{record.agentID}</Tag> : <Tag color="blue">T#{record.tenantID ?? '-'} / B#{record.brandID ?? '-'}</Tag>,
    },
    { title: t('rbac.users.columns.lastLoginAt'), dataIndex: 'lastLoginAt', render: (value?: string | null) => value || t('common.none') },
    {
      title: t('rbac.users.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button icon={<EditOutlined />} onClick={() => openEditUserModal(record)} disabled={!hasPermission('rbac:users:write')}>
            {t('rbac.users.edit')}
          </Button>
          <Button danger onClick={() => confirmDeleteUser(record)} disabled={!hasPermission('rbac:users:write')} loading={deleteUserMutation.isPending}>
            {t('common.delete')}
          </Button>
        </Space>
      ),
    },
  ]

  const roleColumns: ColumnsType<RbacRole> = [
    {
      title: t('rbac.roles.columns.role'),
      key: 'role',
      render: (_, record) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{record.name}</Typography.Text>
          <Typography.Text type="secondary">{record.code}</Typography.Text>
        </Space>
      ),
    },
    {
      title: t('rbac.roles.columns.permissions'),
      dataIndex: 'permissions',
      render: (value: string[]) => (
        <Space size={[4, 4]} wrap>
          {value.length > 0 ? value.map((code) => (
            <Tag key={code}>{permissionMap[code] ? getPermissionLabel(permissionMap[code], t) : code}</Tag>
          )) : <Typography.Text type="secondary">{t('common.none')}</Typography.Text>}
        </Space>
      ),
    },
    {
      title: t('rbac.roles.columns.scope'),
      key: 'scope',
      render: (_, record) => <Tag color={record.tenantID ? 'blue' : 'default'}>{record.tenantID ? `T${record.tenantID} / B${record.brandID ?? '-'}` : t('rbac.scope.platform')}</Tag>,
    },
    { title: t('rbac.roles.columns.updatedAt'), dataIndex: 'updatedAt' },
    {
      title: t('rbac.roles.columns.actions'),
      key: 'actions',
      render: (_, record) => (
        <Space>
          <Button icon={<EditOutlined />} onClick={() => openEditRoleModal(record)} disabled={!hasPermission('rbac:roles:write')}>
            {t('rbac.roles.edit')}
          </Button>
          <Button danger onClick={() => confirmDeleteRole(record)} disabled={!hasPermission('rbac:roles:write')} loading={deleteRoleMutation.isPending}>
            {t('common.delete')}
          </Button>
        </Space>
      ),
    },
  ]

  const sectionTitle = {
    users: t('rbac.users.cardTitle'),
    roles: t('rbac.roles.cardTitle'),
    permissions: t('rbac.permissions.cardTitle'),
  }[section]

  const sectionSubtitle = {
    users: t('rbac.section.usersSubtitle'),
    roles: t('rbac.section.rolesSubtitle'),
    permissions: t('rbac.section.permissionsSubtitle'),
  }[section]

  return (
    <Space direction="vertical" size="large" className="app-page">
      <ScopeNotice />
      <Card className="app-card app-card--compact-hero" bordered={false}>
        <Space direction="vertical" size="small" className="app-page__hero">
          <div className="app-page__heading">
            <Typography.Title level={3} className="app-page__title">{sectionTitle}</Typography.Title>
            <Typography.Paragraph type="secondary" className="app-page__subtitle">{sectionSubtitle}</Typography.Paragraph>
          </div>
          <Row gutter={[12, 12]}>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('rbac.permissions.total')} value={permissions.length} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('rbac.users.total')} value={users.length} prefix={<TeamOutlined />} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('rbac.roles.total')} value={roles.length} /></Card></Col>
            <Col xs={24} sm={12} lg={6}><Card className="app-stat-card"><Statistic title={t('rbac.roles.assignedPermissions')} value={rolePermissionCount + assignedRoleCount} /></Card></Col>
          </Row>
          <div className="app-toolbar">
            <div className="app-toolbar__actions">
              <Button icon={<ReloadOutlined />} onClick={refreshAll} loading={permissionsQuery.isFetching || rolesQuery.isFetching || usersQuery.isFetching}>{t('common.refresh')}</Button>
              {section === 'users' ? <Button icon={<UserAddOutlined />} onClick={openCreateUserModal} disabled={!hasPermission('rbac:users:write')}>{t('rbac.users.create')}</Button> : null}
              {section === 'roles' ? <Button type="primary" onClick={openCreateRoleModal} disabled={!hasPermission('rbac:roles:write')}>{t('rbac.roles.create')}</Button> : null}
            </div>
          </div>
        </Space>
      </Card>

      {permissionsQuery.error ? <Alert type="error" showIcon message={t('rbac.permissions.loadError')} description={(permissionsQuery.error as Error).message} /> : null}
      {rolesQuery.error ? <Alert type="error" showIcon message={t('rbac.roles.loadError')} description={(rolesQuery.error as Error).message} /> : null}
      {usersQuery.error ? <Alert type="error" showIcon message={t('rbac.users.loadError')} description={(usersQuery.error as Error).message} /> : null}
      {saveRoleMutation.error ? <Alert type="error" showIcon message={editingRole ? t('rbac.roles.updateError') : t('rbac.roles.createError')} description={(saveRoleMutation.error as Error).message} /> : null}

      {section === 'users' && hasPermission('rbac:users:view') ? (
        <Card className="app-table-card" title={t('rbac.users.cardTitle')} extra={<Typography.Text type="secondary">{users.length} {t('rbac.users.results')}</Typography.Text>}>
          <div className="app-table app-table--compact">
            <Table<RbacUser> rowKey="id" loading={usersQuery.isLoading || usersQuery.isFetching} dataSource={users} columns={userColumns} pagination={{ pageSize: 10, showSizeChanger: false, showTotal: (total) => `${total} ${t('rbac.users.results')}` }} />
          </div>
        </Card>
      ) : null}

      {section === 'roles' ? <Card className="app-table-card" title={t('rbac.roles.cardTitle')} extra={<Typography.Text type="secondary">{roles.length} {t('rbac.roles.results')}</Typography.Text>}>
        <div className="app-table app-table--compact">
          <Table<RbacRole> rowKey="id" loading={rolesQuery.isLoading || rolesQuery.isFetching} dataSource={roles} columns={roleColumns} pagination={{ pageSize: 10, showSizeChanger: false }} />
        </div>
      </Card> : null}

      {section === 'permissions' ? <Card className="app-table-card" title={t('rbac.permissions.cardTitle')} extra={<Typography.Text type="secondary">{permissions.length} {t('rbac.permissions.results')}</Typography.Text>}>
        {permissions.length > 0 ? (
          <Collapse
            ghost
            className="rbac-permission-catalog"
            defaultActiveKey={groupedPermissions.map((group) => group.key)}
            items={groupedPermissions.map((group) => ({
              key: group.key,
              label: (
                <Space size={8}>
                  <Typography.Text strong>{t(group.labelKey)}</Typography.Text>
                  <Tag>{group.menus.reduce((count, menu) => count + menu.permissions.length, 0)}</Tag>
                </Space>
              ),
              children: (
                <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                  {group.menus.map((menu) => (
                    <div key={menu.labelKey} className="rbac-permission-menu">
                      <Typography.Text className="rbac-permission-menu__title">{t(menu.labelKey)}</Typography.Text>
                      <Space size={[8, 8]} wrap>
                        {menu.permissions.map((permission) => (
                          <Tag key={permission.code} color="blue">{getPermissionLabel(permission, t)} ({permission.code})</Tag>
                        ))}
                      </Space>
                    </div>
                  ))}
                </Space>
              ),
            }))}
          />
        ) : <Typography.Text type="secondary">{t('rbac.permissions.empty')}</Typography.Text>}
      </Card> : null}

      <Modal width={860} title={editingRole ? t('rbac.roles.modalEditTitle') : t('rbac.roles.modalCreateTitle')} open={roleModalOpen} onCancel={closeRoleModal} onOk={() => void handleRoleSubmit()} okText={t('common.save')} cancelText={t('common.cancel')} confirmLoading={saveRoleMutation.isPending} destroyOnClose>
        <Form<RoleFormValues> form={roleForm} layout="vertical">
          <Alert type="info" showIcon message={t('rbac.scope.formHint')} style={{ marginBottom: 16 }} />
          <Form.Item label={t('rbac.scope.level')} name="scopeLevel" rules={[{ required: true, message: t('rbac.scope.levelRequired') }]}>
            <Select
              disabled={Boolean(editingRole)}
              options={[
                { label: t('rbac.scope.platform'), value: 'platform', disabled: currentScopeLevel !== 'platform' },
                { label: t('rbac.scope.tenantLevel'), value: 'tenant', disabled: currentScopeLevel === 'brand' || currentScopeLevel === 'agent' },
                { label: t('rbac.scope.brandLevel'), value: 'brand', disabled: currentScopeLevel === 'agent' },
              ]}
              onChange={(level) => {
                if (level === 'platform') {
                  roleForm.setFieldsValue({ tenantID: null, brandID: null })
                }
                if (level === 'tenant') {
                  roleForm.setFieldValue('brandID', null)
                }
              }}
            />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label={t('rbac.scope.tenant')} name="tenantID">
                <TenantSelect disabled={Boolean(editingRole) || selectedRoleScopeLevel === 'platform' || isTenantLocked} onChange={() => roleForm.setFieldValue('brandID', null)} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label={t('rbac.scope.brand')} name="brandID">
                <BrandSelect tenantID={selectedRoleTenantID ?? undefined} disabled={Boolean(editingRole) || selectedRoleScopeLevel !== 'brand' || isBrandLocked} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label={t('rbac.roles.form.code')} name="code" rules={[{ required: true, message: t('rbac.roles.form.codeRequired') }]}> 
            <Input placeholder={t('rbac.roles.form.codePlaceholder')} disabled={Boolean(editingRole)} />
          </Form.Item>
          <Form.Item label={t('rbac.roles.form.name')} name="name" rules={[{ required: true, message: t('rbac.roles.form.nameRequired') }]}>
            <Input placeholder={t('rbac.roles.form.namePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('rbac.roles.form.permissions')} name="permissions">
            <Checkbox.Group style={{ width: '100%' }}>
              <Collapse
                className="rbac-permission-picker"
                defaultActiveKey={groupedPermissions.map((group) => group.key)}
                items={groupedPermissions.map((group) => {
                  const groupPermissionCodes = group.menus.flatMap((menu) => menu.permissions.map((permission) => permission.code))
                  const selectedCount = groupPermissionCodes.filter((code) => selectedRolePermissions.includes(code)).length

                  return {
                    key: group.key,
                    label: (
                      <Space size={8}>
                        <Typography.Text strong>{t(group.labelKey)}</Typography.Text>
                        <Tag color={selectedCount > 0 ? 'blue' : 'default'}>{getPermissionCountText(selectedCount, groupPermissionCodes.length, t)}</Tag>
                      </Space>
                    ),
                    children: (
                      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
                        {group.menus.map((menu) => (
                          <div key={menu.labelKey} className="rbac-permission-menu">
                            <Typography.Text className="rbac-permission-menu__title">{t(menu.labelKey)}</Typography.Text>
                            <Row gutter={[10, 10]}>
                              {menu.permissions.map((permission) => (
                                <Col key={permission.code} xs={24} sm={12} lg={8}>
                                  <Checkbox value={permission.code} className="rbac-permission-checkbox">
                                    <span className="rbac-permission-checkbox__label">{getPermissionLabel(permission, t)}</span>
                                    <span className="rbac-permission-checkbox__code">{permission.code}</span>
                                  </Checkbox>
                                </Col>
                              ))}
                            </Row>
                          </div>
                        ))}
                      </Space>
                    ),
                  }
                })}
              />
            </Checkbox.Group>
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={editingUser ? t('rbac.users.modalEditTitle') : t('rbac.users.modalCreateTitle')} open={userModalOpen} onCancel={closeUserModal} onOk={() => void handleUserSubmit()} okText={t('common.save')} cancelText={t('common.cancel')} confirmLoading={saveUserMutation.isPending} destroyOnClose>
        <Form<UserFormValues> form={userForm} layout="vertical">
          <Alert type="info" showIcon message={t('rbac.scope.userFormHint')} style={{ marginBottom: 16 }} />
          {saveUserMutation.error ? <Alert type="error" showIcon message={editingUser ? t('rbac.users.updateError') : t('rbac.users.createError')} description={(saveUserMutation.error as Error).message} style={{ marginBottom: 16 }} /> : null}
          <Form.Item label={t('rbac.users.form.username')} name="username" rules={[{ required: !editingUser, message: t('rbac.users.form.usernameRequired') }]}> 
            <Input placeholder={t('rbac.users.form.usernamePlaceholder')} disabled={Boolean(editingUser)} />
          </Form.Item>
          <Form.Item label={t('rbac.users.form.displayName')} name="displayName">
            <Input placeholder={t('rbac.users.form.displayNamePlaceholder')} />
          </Form.Item>
          <Form.Item label={t('rbac.users.form.password')} name="password" rules={[{ required: !editingUser, message: t('rbac.users.form.passwordRequired') }]}>
            <Input.Password placeholder={editingUser ? t('rbac.users.form.passwordEditPlaceholder') : t('rbac.users.form.passwordPlaceholder')} />
          </Form.Item>
          <Form.Item label={t('rbac.users.form.status')} name="status" rules={[{ required: true, message: t('rbac.users.form.statusRequired') }]}> 
            <Select options={[{ label: t('rbac.users.status.active'), value: 'active' }, { label: t('rbac.users.status.disabled'), value: 'disabled' }]} />
          </Form.Item>
          <Form.Item label={t('rbac.scope.level')} name="scopeLevel" rules={[{ required: true, message: t('rbac.scope.levelRequired') }]}>
            <Select
              options={[
                { label: t('rbac.scope.platform'), value: 'platform', disabled: currentScopeLevel !== 'platform' },
                { label: t('rbac.scope.tenantLevel'), value: 'tenant', disabled: currentScopeLevel === 'brand' || currentScopeLevel === 'agent' },
                { label: t('rbac.scope.brandLevel'), value: 'brand', disabled: currentScopeLevel === 'agent' },
                { label: t('rbac.scope.agentLevel'), value: 'agent' },
              ]}
              onChange={(level) => {
                if (level === 'platform') {
                  userForm.setFieldsValue({ tenantID: null, brandID: null, agentID: null, roles: [] })
                }
                if (level === 'tenant') {
                  userForm.setFieldsValue({ brandID: null, agentID: null, roles: [] })
                }
                if (level === 'brand') {
                  userForm.setFieldsValue({ agentID: null, roles: [] })
                }
              }}
            />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label={t('rbac.scope.tenant')} name="tenantID">
                <TenantSelect disabled={selectedUserScopeLevel === 'platform' || isTenantLocked} onChange={() => userForm.setFieldsValue({ brandID: null, agentID: null, roles: [] })} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label={t('rbac.scope.brand')} name="brandID">
                <BrandSelect tenantID={selectedUserTenantID ?? undefined} disabled={(selectedUserScopeLevel !== 'brand' && selectedUserScopeLevel !== 'agent') || isBrandLocked} onChange={() => userForm.setFieldValue('roles', [])} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label={t('rbac.users.form.agentID')} name="agentID" hidden={selectedUserScopeLevel !== 'agent'}>
            <AgentSelect
              tenantID={selectedUserTenantID ?? scopeParams.tenantID}
              brandID={selectedUserBrandID ?? scopeParams.brandID}
              placeholder={t('rbac.users.form.agentIDPlaceholder')}
              style={{ width: '100%' }}
            />
          </Form.Item>
          <Form.Item label={t('rbac.users.form.roles')} name="roles" rules={[{ required: true, message: t('rbac.users.form.rolesRequired') }]}>
            <Select mode="multiple" options={roleOptions} placeholder={t('rbac.users.form.rolesPlaceholder')} />
          </Form.Item>
        </Form>
      </Modal>
    </Space>
  )
}
