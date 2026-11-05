import { Button, Card, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { Experiment } from '../types'

interface ExperimentForm {
  key: string
  name: string
  variant_a: string
  variant_b: string
  traffic_percent: number
  status: string
}

export default function ExperimentsPage() {
  const [items, setItems] = useState<Experiment[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<Experiment | null>(null)
  const [open, setOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [form] = Form.useForm<ExperimentForm>()

  const load = async () => {
    setLoading(true)
    try {
      setItems((await api.experiments()).items || [])
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const openForm = (experiment?: Experiment) => {
    setEditing(experiment ?? null)
    form.setFieldsValue(
      experiment
        ? {
            key: experiment.key,
            name: experiment.name,
            variant_a: experiment.variant_a,
            variant_b: experiment.variant_b,
            traffic_percent: experiment.traffic_percent,
            status: experiment.status,
          }
        : {
            key: '',
            name: '',
            variant_a: 'control',
            variant_b: 'treatment',
            traffic_percent: 50,
            status: 'active',
          },
    )
    setOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    setSaving(true)
    try {
      if (editing) {
        await api.updateExperiment(editing.id, values)
      } else {
        await api.createExperiment(values)
      }
      message.success(editing ? '实验已更新' : '实验已创建')
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
          A/B 实验
        </Typography.Title>
        <Button type="primary" onClick={() => openForm()}>
          新建实验
        </Button>
      </Space>
      <Table
        rowKey="id"
        size="small"
        dataSource={items}
        pagination={false}
        columns={[
          { title: 'Key', dataIndex: 'key' },
          { title: '名称', dataIndex: 'name' },
          {
            title: '变体',
            render: (_, row) => (
              <Space>
                <Tag>{row.variant_a}</Tag>
                <Tag color="blue">{row.variant_b}</Tag>
              </Space>
            ),
          },
          { title: 'B 流量', dataIndex: 'traffic_percent', render: (v) => `${v}%` },
          {
            title: '曝光',
            render: (_, row) => `${row.exposure_a} / ${row.exposure_b}`,
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
        title={editing ? `编辑实验 · ${editing.key}` : '新建实验'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={submit}
        confirmLoading={saving}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="key" label="实验 Key" rules={[{ required: true }]}>
            <Input placeholder="home-cta" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Space size={12}>
            <Form.Item name="variant_a" label="变体 A" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="variant_b" label="变体 B" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
          </Space>
          <Space size={12}>
            <Form.Item name="traffic_percent" label="B 流量 %">
              <InputNumber min={1} max={100} style={{ width: 140 }} />
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
