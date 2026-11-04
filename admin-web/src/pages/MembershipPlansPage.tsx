import {
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { useAuth } from '../auth'
import type { MembershipPlan } from '../types'

const statusLabel: Record<string, string> = {
  draft: '草稿',
  online: '已上线',
  offline: '已下线',
}
const entitlementLabel: Record<string, string> = {
  topic_pack: '付费专题题包',
  report_advanced: '进阶报告',
  customer_service: '专属客服',
}
const entitlementOptions = Object.entries(entitlementLabel).map(([value, label]) => ({
  value,
  label,
}))
const statusColor: Record<string, string> = {
  draft: 'default',
  online: 'success',
  offline: 'orange',
}

function yuan(cents: number) {
  return `¥${(cents / 100).toFixed(2)}`
}

export default function MembershipPlansPage() {
  const { can } = useAuth()
  const [plans, setPlans] = useState<MembershipPlan[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<MembershipPlan | null>(null)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm()

  const load = async () => {
    setLoading(true)
    try {
      const result = await api.membershipPlans()
      setPlans(result.items || [])
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({
      duration_days: 30,
      price_yuan: 19.9,
      status: 'draft',
      sort: 0,
      features_text: '',
      entitlement_codes: [],
    })
    setOpen(true)
  }

  const openEdit = (plan: MembershipPlan) => {
    setEditing(plan)
    form.setFieldsValue({
      code: plan.code,
      name: plan.name,
      description: plan.description,
      duration_days: plan.duration_days,
      price_yuan: plan.price_cents / 100,
      status: plan.status,
      sort: plan.sort,
      features_text: (plan.features || []).join('\n'),
      entitlement_codes: plan.entitlement_codes || [],
    })
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    const payload = {
      code: values.code,
      name: values.name,
      description: values.description || '',
      duration_days: values.duration_days,
      price_cents: Math.round(values.price_yuan * 100),
      status: values.status,
      sort: values.sort || 0,
      entitlement_codes: values.entitlement_codes || [],
      features: (values.features_text || '')
        .split('\n')
        .map((item: string) => item.trim())
        .filter(Boolean),
    }
    setSaving(true)
    try {
      if (editing) {
        await api.updateMembershipPlan(editing.id, payload)
        message.success('套餐已更新')
      } else {
        await api.createMembershipPlan(payload)
        message.success('套餐已创建')
      }
      setOpen(false)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const remove = async (plan: MembershipPlan) => {
    try {
      await api.deleteMembershipPlan(plan.id)
      message.success('套餐已删除')
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '删除失败')
    }
  }

  const columns: ColumnsType<MembershipPlan> = [
    { title: '排序', dataIndex: 'sort', width: 70 },
    { title: '名称', dataIndex: 'name', width: 160 },
    { title: '编码', dataIndex: 'code', width: 140 },
    {
      title: '时长',
      dataIndex: 'duration_days',
      width: 90,
      render: (value: number) => `${value} 天`,
    },
    {
      title: '价格',
      dataIndex: 'price_cents',
      width: 100,
      render: (value: number) => yuan(value),
    },
    {
      title: '权益',
      dataIndex: 'features',
      render: (features: string[]) =>
        features && features.length > 0 ? (
          <Space wrap>
            {features.map((item) => (
              <Tag key={item}>{item}</Tag>
            ))}
          </Space>
        ) : (
          '—'
        ),
    },
    {
      title: '功能权益',
      dataIndex: 'entitlement_codes',
      width: 200,
      render: (codes: string[]) =>
        codes && codes.length > 0 ? (
          <Space wrap>
            {codes.map((code) => (
              <Tag color="green" key={code}>
                {entitlementLabel[code] || code}
              </Tag>
            ))}
          </Space>
        ) : (
          '—'
        ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: string) => (
        <Tag color={statusColor[value] || 'default'}>{statusLabel[value] || value}</Tag>
      ),
    },
    {
      title: '操作',
      width: 150,
      render: (_, plan) => (
        <Space size={0}>
          <Button
            type="link"
            disabled={!can('membership.write')}
            onClick={() => openEdit(plan)}
          >
            编辑
          </Button>
          <Popconfirm
            title={`删除套餐「${plan.name}」？`}
            description="已有订单引用的套餐无法删除，请改为下线。"
            okText="删除"
            cancelText="取消"
            onConfirm={() => void remove(plan)}
            disabled={!can('membership.write')}
          >
            <Button type="link" danger disabled={!can('membership.write')}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Card>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          会员套餐
        </Typography.Title>
        <Button
          type="primary"
          disabled={!can('membership.write')}
          onClick={openCreate}
        >
          新建套餐
        </Button>
        <Typography.Text type="secondary">
          价格按元展示，服务端以分存储
        </Typography.Text>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={plans}
        pagination={false}
        locale={{ emptyText: '还没有会员套餐' }}
      />

      <Modal
        open={open}
        title={editing ? `编辑套餐 · ${editing.name}` : '新建套餐'}
        onCancel={() => setOpen(false)}
        onOk={() => void submit()}
        confirmLoading={saving}
        okText="保存"
        forceRender
      >
        <Form form={form} layout="vertical">
          <Space size="large" wrap>
            <Form.Item
              name="code"
              label="编码"
              rules={[
                { required: true, message: '请填写编码' },
                { pattern: /^[a-z][a-z0-9-]{1,31}$/, message: '2–32 位小写字母/数字/连字符' },
              ]}
            >
              <Input placeholder="monthly" style={{ width: 200 }} />
            </Form.Item>
            <Form.Item name="status" label="状态" rules={[{ required: true }]}>
              <Select
                style={{ width: 140 }}
                options={Object.entries(statusLabel).map(([value, label]) => ({ value, label }))}
              />
            </Form.Item>
            <Form.Item name="sort" label="排序">
              <InputNumber min={0} max={10000} style={{ width: 120 }} />
            </Form.Item>
          </Space>
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请填写名称' }, { max: 50 }]}
          >
            <Input placeholder="月度会员" />
          </Form.Item>
          <Space size="large" wrap>
            <Form.Item
              name="duration_days"
              label="时长（天）"
              rules={[{ required: true }]}
            >
              <InputNumber min={1} max={3650} style={{ width: 140 }} />
            </Form.Item>
            <Form.Item
              name="price_yuan"
              label="价格（元）"
              rules={[{ required: true }]}
            >
              <InputNumber min={0} step={0.1} style={{ width: 140 }} />
            </Form.Item>
          </Space>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} placeholder="套餐说明" />
          </Form.Item>
          <Form.Item name="entitlement_codes" label="功能权益（实际生效）">
            <Select
              mode="multiple"
              placeholder="选择会员实际解锁的功能"
              options={entitlementOptions}
            />
          </Form.Item>
          <Form.Item
            name="features_text"
            label="权益文案（每行一条，仅展示）"
            extra="权益暂只展示，后续接入具体功能开关"
          >
            <Input.TextArea rows={4} placeholder={'专题题包优先体验\n成长报告进阶分析'} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}
