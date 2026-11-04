import { Button, Card, Form, Input, Typography, message } from 'antd'
import { useState } from 'react'
import { api, ApiError } from '../api'
import type { AdminPrincipal } from '../types'

interface Props {
  onLogin: (principal: AdminPrincipal) => void
}

export default function Login({ onLogin }: Props) {
  const [loading, setLoading] = useState(false)
  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f0f5f3',
      }}
    >
      <Card style={{ width: 380 }}>
        <Typography.Title level={3} style={{ marginBottom: 4 }}>
          OpenQuizGo
        </Typography.Title>
        <Typography.Text type="secondary">题库治理后台</Typography.Text>
        <Form
          layout="vertical"
          style={{ marginTop: 24 }}
          onFinish={async (values) => {
            setLoading(true)
            try {
              const result = await api.login(values.username, values.password)
              onLogin(result.admin)
            } catch (error) {
              message.error(error instanceof ApiError ? error.message : '登录失败')
            } finally {
              setLoading(false)
            }
          }}
        >
          <Form.Item name="username" label="账号" rules={[{ required: true }]}>
            <Input autoFocus placeholder="admin" />
          </Form.Item>
          <Form.Item name="password" label="密码" rules={[{ required: true }]}>
            <Input.Password placeholder="密码" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            登录
          </Button>
        </Form>
      </Card>
    </div>
  )
}
