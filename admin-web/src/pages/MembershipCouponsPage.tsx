import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { MembershipCoupon } from '../types'

interface CouponForm {
  code: string
  name: string
  amount: number
  min_amount: number
  max_uses: number
  status: string
}

export default function MembershipCouponsPage() {
  const [items, setItems] = useState<MembershipCoupon[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<MembershipCoupon | null>(null)
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<CouponForm>()

  const load = async () => {
    setLoading(true)
    try {
      const result = await api.membershipCoupons()
      setItems(result.items || [])
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const openForm = (coupon?: MembershipCoupon) => {
    setEditing(coupon ?? null)
    form.setFieldsValue(
      coupon
        ? {
            code: coupon.code,
            name: coupon.name,
            amount: coupon.amount_cents / 100,
            min_amount: coupon.min_amount_cents / 100,
            max_uses: coupon.max_uses,
            status: coupon.status,
          }
        : {
            code: '',
            name: '',
            amount: 1,
            min_amount: 0,
            max_uses: 0,
            status: 'active',
          },
    )
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    setSaving(true)
    const payload = {
      code: values.code.trim().toUpperCase(),
      name: values.name.trim(),
      amount_cents: Math.round(values.amount * 100),
      min_amount_cents: Math.round((values.min_amount || 0) * 100),
      max_uses: values.max_uses || 0,
      status: values.status,
    }
    try {
      if (editing) {
        await api.updateMembershipCoupon(editing.id, payload)
      } else {
        await api.createMembershipCoupon(payload)
      }
      message.success(editing ? '优惠券已更新' : '优惠券已创建')
      setOpen(false)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card loading={loading}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          优惠券
        </Typography.Title>
        <Button type="primary" onClick={() => openForm()}>
          新建优惠券
        </Button>
      </Space>
      <Table
        rowKey="id"
        size="small"
        dataSource={items}
        pagination={false}
        columns={[
          { title: '优惠码', dataIndex: 'code' },
          { title: '名称', dataIndex: 'name' },
          {
            title: '优惠',
            render: (_, row) => `¥${(row.amount_cents / 100).toFixed(2)}`,
          },
          {
            title: '门槛',
            render: (_, row) =>
              row.min_amount_cents > 0
                ? `满 ¥${(row.min_amount_cents / 100).toFixed(2)}`
                : '无门槛',
          },
          {
            title: '使用',
            render: (_, row) =>
              `${row.used_count}/${row.max_uses === 0 ? '不限' : row.max_uses}`,
          },
          {
            title: '状态',
            dataIndex: 'status',
            render: (value) => (
              <Tag color={value === 'active' ? 'green' : 'default'}>
                {value === 'active' ? '启用' : '停用'}
              </Tag>
            ),
          },
          {
            title: '操作',
            width: 100,
            render: (_, row) => (
              <Button size="small" onClick={() => openForm(row)}>
                编辑
              </Button>
            ),
          },
        ]}
      />
      <Modal
        title={editing ? `编辑优惠券 · ${editing.code}` : '新建优惠券'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="code" label="优惠码（大写字母/数字）" rules={[{ required: true }]}>
            <Input placeholder="SAVE100" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Space style={{ width: '100%' }} size={12}>
            <Form.Item name="amount" label="优惠金额（元）" rules={[{ required: true }]}>
              <InputNumber min={0.01} precision={2} style={{ width: 160 }} />
            </Form.Item>
            <Form.Item name="min_amount" label="最低订单（元）">
              <InputNumber min={0} precision={2} style={{ width: 160 }} />
            </Form.Item>
          </Space>
          <Space style={{ width: '100%' }} size={12}>
            <Form.Item name="max_uses" label="最多使用次数（0 不限）">
              <InputNumber min={0} style={{ width: 180 }} />
            </Form.Item>
            <Form.Item name="status" label="状态">
              <Select
                style={{ width: 120 }}
                options={[
                  { value: 'active', label: '启用' },
                  { value: 'disabled', label: '停用' },
                ]}
              />
            </Form.Item>
          </Space>
        </Form>
      </Modal>
    </Card>
  )
}
