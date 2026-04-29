import { LockOutlined, UserOutlined } from '@ant-design/icons'
import { Alert, Button, Card, Form, Input, Space, Tag, Typography } from 'antd'
import { useMutation } from '@tanstack/react-query'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../../auth'
import { firstAccessibleNavPath } from '../../config/permission-taxonomy'
import { useI18n } from '../../i18n'
import { apiClient } from '../../lib/api'
import './login.css'

type LoginFormValues = {
  username: string
  password: string
}

export default function LoginPage() {
  const { t } = useI18n()
  const { signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const redirectTo = (location.state as { from?: { pathname?: string } } | undefined)?.from?.pathname ?? '/'

  const loginMutation = useMutation({
    mutationFn: apiClient.login,
    onSuccess: (data) => {
      signIn(data.token, data.user)
      const fallbackPath = firstAccessibleNavPath(data.user?.permissions ?? [])
      navigate(redirectTo === '/' ? fallbackPath : redirectTo, { replace: true })
    },
  })

  const handleSubmit = (values: LoginFormValues) => {
    loginMutation.mutate(values)
  }

  const highlights = [
    t('login.highlight.scope'),
    t('login.highlight.audit'),
    t('login.highlight.rbac'),
  ]

  const metrics = [
    { value: '24/7', label: t('login.metric.availability') },
    { value: '3', label: t('login.metric.layers') },
    { value: 'RBAC', label: t('login.metric.access') },
  ]

  return (
    <div className="login-page">
      <div className="login-page__mesh" aria-hidden="true" />
      <div className="login-page__shell">
        <section className="login-page__hero">
          <div className="login-page__hero-head">
            <Tag bordered={false} className="login-page__badge">
              {t('login.badge')}
            </Tag>
            <Typography.Title level={1} className="login-page__title">
              {t('login.heroTitle')}
            </Typography.Title>
            <Typography.Paragraph className="login-page__hero-copy">
              {t('login.heroDescription')}
            </Typography.Paragraph>
          </div>

          <div className="login-page__metric-grid" aria-label={t('login.metricsAria')}>
            {metrics.map((metric) => (
              <div key={metric.label} className="login-page__metric-card">
                <span className="login-page__metric-value">{metric.value}</span>
                <span className="login-page__metric-label">{metric.label}</span>
              </div>
            ))}
          </div>

          <Card bordered={false} className="login-page__hero-card">
            <Space direction="vertical" size="middle" style={{ width: '100%' }}>
              <Typography.Text className="login-page__panel-kicker">
                {t('login.panelTitle')}
              </Typography.Text>
              <Typography.Paragraph className="login-page__panel-copy">
                {t('login.panelDescription')}
              </Typography.Paragraph>
              <div className="login-page__tag-row">
                {highlights.map((item) => (
                  <span key={item} className="login-page__flat-tag">
                    {item}
                  </span>
                ))}
              </div>
            </Space>
          </Card>
        </section>

        <Card bordered={false} className="login-page__form-card">
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <div>
              <Typography.Text className="login-page__eyebrow">
                {t('login.formEyebrow')}
              </Typography.Text>
              <Typography.Title level={3} className="login-page__form-title">
                {t('login.title')}
              </Typography.Title>
              <Typography.Paragraph className="login-page__form-copy">
                {t('login.subtitle')}
              </Typography.Paragraph>
            </div>

            {loginMutation.error ? (
              <Alert
                type="error"
                showIcon
                message={t('login.failed')}
                description={t('login.failedDescription')}
              />
            ) : null}

            <Form<LoginFormValues>
              layout="vertical"
              initialValues={{ username: '', password: '' }}
              onFinish={handleSubmit}
              autoComplete="off"
              className="login-page__form"
            >
              <Form.Item label={t('login.username')} name="username" rules={[{ required: true, message: t('login.usernameRequired') }]}>
                <Input prefix={<UserOutlined />} placeholder={t('login.usernamePlaceholder')} size="large" />
              </Form.Item>
              <Form.Item label={t('login.password')} name="password" rules={[{ required: true, message: t('login.passwordRequired') }]}>
                <Input.Password prefix={<LockOutlined />} placeholder={t('login.passwordPlaceholder')} size="large" />
              </Form.Item>
              <Button type="primary" htmlType="submit" block loading={loginMutation.isPending} size="large" className="login-page__submit">
                {t('login.submit')}
              </Button>
            </Form>

            <Typography.Text className="login-page__assist">
              {t('login.assist')}
            </Typography.Text>
          </Space>
        </Card>
      </div>
    </div>
  )
}
