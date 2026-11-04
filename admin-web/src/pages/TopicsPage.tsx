import {
  Button,
  Card,
  DatePicker,
  Form,
  Input,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { useAuth } from '../auth'
import type { Question, TopicPack } from '../types'

const typeLabel: Record<string, string> = {
  holiday: '节日',
  ip: 'IP 联名',
  vacation: '假期',
  daily: '每日',
}
const statusLabel: Record<string, string> = {
  draft: '草稿',
  online: '已上线',
  offline: '已下线',
}
const statusColor: Record<string, string> = {
  draft: 'default',
  online: 'success',
  offline: 'orange',
}
const subjectOptions = [
  { value: 'math', label: '数学' },
  { value: 'chinese', label: '语文' },
  { value: 'english', label: '英语' },
  { value: 'science', label: '科学' },
  { value: 'logic', label: '逻辑' },
  { value: 'safety', label: '安全' },
  { value: 'fun', label: '趣味' },
]
const stageOptions = [
  { value: 'preschool', label: '学龄前' },
  { value: 'low', label: '小学低' },
  { value: 'mid', label: '小学中' },
  { value: 'high', label: '小学高' },
]

export default function TopicsPage() {
  const { can } = useAuth()
  const [packs, setPacks] = useState<TopicPack[]>([])
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<TopicPack | null>(null)
  const [formOpen, setFormOpen] = useState(false)
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)

  const [pickerPack, setPickerPack] = useState<TopicPack | null>(null)
  const [selected, setSelected] = useState<Question[]>([])
  const [results, setResults] = useState<Question[]>([])
  const [searching, setSearching] = useState(false)
  const [pickerLoading, setPickerLoading] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      const result = await api.topics()
      setPacks(result.items || [])
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
      type: 'holiday',
      status: 'draft',
      is_paid: false,
      subject_scope: [],
      stage_scope: [],
    })
    setFormOpen(true)
  }

  const openEdit = (pack: TopicPack) => {
    setEditing(pack)
    form.setFieldsValue({
      name: pack.name,
      type: pack.type,
      status: pack.status,
      is_paid: pack.is_paid,
      subject_scope: pack.subject_scope || [],
      stage_scope: pack.stage_scope || [],
      range:
        pack.start_at || pack.end_at
          ? [
              pack.start_at ? dayjs(pack.start_at) : null,
              pack.end_at ? dayjs(pack.end_at) : null,
            ]
          : undefined,
    })
    setFormOpen(true)
  }

  const submit = async () => {
    const values = await form.validateFields()
    const range = values.range as [dayjs.Dayjs | null, dayjs.Dayjs | null] | undefined
    const payload = {
      name: values.name,
      type: values.type,
      status: values.status,
      is_paid: Boolean(values.is_paid),
      subject_scope: values.subject_scope || [],
      stage_scope: values.stage_scope || [],
      start_at: range?.[0] ? range[0].toISOString() : null,
      end_at: range?.[1] ? range[1].toISOString() : null,
    }
    setSaving(true)
    try {
      if (editing) {
        await api.updateTopic(editing.id, payload)
        message.success('专题已更新')
      } else {
        await api.createTopic(payload)
        message.success('专题已创建')
      }
      setFormOpen(false)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const remove = async (pack: TopicPack) => {
    try {
      await api.deleteTopic(pack.id)
      message.success('专题已删除')
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '删除失败')
    }
  }

  const openPicker = async (pack: TopicPack) => {
    setPickerPack(pack)
    setResults([])
    setPickerLoading(true)
    try {
      const detail = await api.topic(pack.id)
      const ids = detail.question_ids || []
      const questions = await Promise.all(
        ids.map(async (id) => {
          try {
            return await api.question(id)
          } catch {
            return null
          }
        }),
      )
      setSelected(questions.filter((item): item is Question => Boolean(item)))
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载题目失败')
    } finally {
      setPickerLoading(false)
    }
  }

  const search = async (keyword: string) => {
    const value = keyword.trim()
    if (!value) {
      setResults([])
      return
    }
    setSearching(true)
    try {
      const result = await api.questions({ q: value, limit: 20 })
      const chosen = new Set(selected.map((item) => item.id))
      setResults((result.items || []).filter((item) => !chosen.has(item.id)))
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '搜索失败')
    } finally {
      setSearching(false)
    }
  }

  const addQuestion = (question: Question) => {
    setSelected((prev) => [...prev, question])
    setResults((prev) => prev.filter((item) => item.id !== question.id))
  }

  const removeQuestion = (id: string) => {
    setSelected((prev) => prev.filter((item) => item.id !== id))
  }

  const saveQuestions = async () => {
    if (!pickerPack) return
    setPickerLoading(true)
    try {
      await api.setTopicQuestions(
        pickerPack.id,
        selected.map((item) => item.id),
      )
      message.success(`已保存 ${selected.length} 道题目`)
      setPickerPack(null)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '保存题目失败')
    } finally {
      setPickerLoading(false)
    }
  }

  const columns: ColumnsType<TopicPack> = [
    { title: '名称', dataIndex: 'name', ellipsis: true },
    {
      title: '类型',
      dataIndex: 'type',
      width: 90,
      render: (value: string) => typeLabel[value] || value,
    },
    {
      title: '范围',
      width: 220,
      render: (_, row) => {
        const subjects = (row.subject_scope || [])
          .map((v) => subjectOptions.find((o) => o.value === v)?.label || v)
          .join('/')
        const stages = (row.stage_scope || [])
          .map((v) => stageOptions.find((o) => o.value === v)?.label || v)
          .join('/')
        return subjects || stages ? `${subjects || '全学科'} · ${stages || '全学段'}` : '全学科 · 全学段'
      },
    },
    {
      title: '时间',
      width: 200,
      render: (_, row) =>
        row.start_at || row.end_at
          ? `${row.start_at ? dayjs(row.start_at).format('MM-DD HH:mm') : '不限'} ~ ${
              row.end_at ? dayjs(row.end_at).format('MM-DD HH:mm') : '不限'
            }`
          : '长期',
    },
    {
      title: '付费',
      dataIndex: 'is_paid',
      width: 70,
      render: (value: boolean) => (value ? '付费' : '免费'),
    },
    { title: '题目', dataIndex: 'question_count', width: 70 },
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
      width: 190,
      render: (_, row) => (
        <Space size={0}>
          <Button type="link" disabled={!can('question.write')} onClick={() => openEdit(row)}>
            编辑
          </Button>
          <Button
            type="link"
            disabled={!can('question.write')}
            onClick={() => void openPicker(row)}
          >
            选题
          </Button>
          <Popconfirm
            title={`删除专题「${row.name}」？`}
            description="题目本身不会被删除。"
            okText="删除"
            cancelText="取消"
            onConfirm={() => void remove(row)}
            disabled={!can('question.write')}
          >
            <Button type="link" danger disabled={!can('question.write')}>
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
          专题题包
        </Typography.Title>
        <Button type="primary" disabled={!can('question.write')} onClick={openCreate}>
          新建专题
        </Button>
      </Space>

      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={packs}
        pagination={false}
        locale={{ emptyText: '还没有专题题包' }}
      />

      <Modal
        open={formOpen}
        title={editing ? `编辑专题 · ${editing.name}` : '新建专题'}
        onCancel={() => setFormOpen(false)}
        onOk={() => void submit()}
        confirmLoading={saving}
        okText="保存"
        forceRender
      >
        <Form form={form} layout="vertical">
          <Form.Item
            name="name"
            label="名称"
            rules={[{ required: true, message: '请填写名称' }, { max: 100 }]}
          >
            <Input placeholder="如：寒假数学闯关" />
          </Form.Item>
          <Space size="large" wrap>
            <Form.Item name="type" label="类型" rules={[{ required: true }]}>
              <Select
                style={{ width: 130 }}
                options={Object.entries(typeLabel).map(([value, label]) => ({ value, label }))}
              />
            </Form.Item>
            <Form.Item name="status" label="状态" rules={[{ required: true }]}>
              <Select
                style={{ width: 130 }}
                options={Object.entries(statusLabel).map(([value, label]) => ({ value, label }))}
              />
            </Form.Item>
            <Form.Item name="is_paid" label="付费专题" valuePropName="checked">
              <Switch />
            </Form.Item>
          </Space>
          <Form.Item name="subject_scope" label="学科范围（空为全部）">
            <Select mode="multiple" options={subjectOptions} placeholder="全部学科" />
          </Form.Item>
          <Form.Item name="stage_scope" label="学段范围（空为全部）">
            <Select mode="multiple" options={stageOptions} placeholder="全部学段" />
          </Form.Item>
          <Form.Item name="range" label="有效期（留空为长期）">
            <DatePicker.RangePicker showTime style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={Boolean(pickerPack)}
        title={`选题 · ${pickerPack?.name || ''}`}
        onCancel={() => setPickerPack(null)}
        onOk={() => void saveQuestions()}
        okText={`保存 ${selected.length} 题`}
        confirmLoading={pickerLoading}
        width={760}
        destroyOnHidden
      >
        <Input.Search
          allowClear
          placeholder="搜索题干关键词后回车"
          loading={searching}
          onSearch={(value) => void search(value)}
          style={{ marginBottom: 16 }}
        />
        {results.length > 0 && (
          <Card size="small" title="搜索结果" style={{ marginBottom: 16 }}>
            {results.map((item) => (
              <div
                key={item.id}
                style={{ display: 'flex', alignItems: 'center', gap: 12, padding: '4px 0' }}
              >
                <Typography.Text ellipsis style={{ flex: 1 }}>
                  {item.stem}
                </Typography.Text>
                <Button size="small" onClick={() => addQuestion(item)}>
                  添加
                </Button>
              </div>
            ))}
          </Card>
        )}
        <Typography.Text type="secondary">已选 {selected.length} 题</Typography.Text>
        <div style={{ maxHeight: 360, overflow: 'auto', marginTop: 8 }}>
          {selected.length === 0 && (
            <Typography.Text type="secondary">尚未选择题目，请先搜索并添加。</Typography.Text>
          )}
          {selected.map((item, index) => (
            <div
              key={item.id}
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: 12,
                padding: '6px 0',
                borderBottom: '1px solid #f0f0f0',
              }}
            >
              <Typography.Text type="secondary" style={{ width: 28 }}>
                {index + 1}
              </Typography.Text>
              <Typography.Text ellipsis style={{ flex: 1 }}>
                {item.stem}
              </Typography.Text>
              <Button size="small" danger onClick={() => removeQuestion(item.id)}>
                移除
              </Button>
            </div>
          ))}
        </div>
      </Modal>
    </Card>
  )
}
