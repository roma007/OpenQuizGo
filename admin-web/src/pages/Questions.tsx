import {
  Button,
  Card,
  Input,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, ApiError } from '../api'
import { useAuth } from '../auth'
import type { Question } from '../types'

const statusColor: Record<string, string> = {
  draft: 'default',
  review: 'processing',
  trial: 'cyan',
  online: 'success',
  offline: 'orange',
}
const statusLabel: Record<string, string> = {
  draft: '草稿',
  review: '待审核',
  trial: '已通过',
  online: '已上线',
  offline: '已下线',
}
const subjectLabel: Record<string, string> = {
  math: '数学',
  chinese: '语文',
  english: '英语',
  science: '科学',
  logic: '逻辑',
  safety: '安全',
  fun: '趣味',
}
const stageLabel: Record<string, string> = {
  preschool: '学龄前',
  low: '小学低',
  mid: '小学中',
  high: '小学高',
}

export default function Questions() {
  const navigate = useNavigate()
  const { can } = useAuth()
  const [data, setData] = useState<Question[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [page, setPage] = useState(1)
  const [filters, setFilters] = useState<Record<string, string>>({})
  const pageSize = 20

  const load = async (nextPage = page, nextFilters = filters) => {
    setLoading(true)
    try {
      const result = await api.questions({
        ...nextFilters,
        limit: pageSize,
        offset: (nextPage - 1) * pageSize,
      })
      setData(result.items || [])
      setTotal(result.total || 0)
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const applyFilter = (key: string, value?: string) => {
    const next = { ...filters }
    if (value) next[key] = value
    else delete next[key]
    setFilters(next)
    setPage(1)
    void load(1, next)
  }

  const columns: ColumnsType<Question> = [
    {
      title: '题干',
      dataIndex: 'stem',
      ellipsis: true,
      width: 360,
    },
    {
      title: '学科/学段',
      width: 150,
      render: (_, row) =>
        `${subjectLabel[row.subject] || row.subject} / ${stageLabel[row.stage] || row.stage}`,
    },
    { title: '类型', dataIndex: 'type', width: 80 },
    { title: '难度', dataIndex: 'difficulty', width: 70 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value: string) => (
        <Tag color={statusColor[value] || 'default'}>
          {statusLabel[value] || value}
        </Tag>
      ),
    },
    {
      title: '更新时间',
      dataIndex: 'updated_at',
      width: 180,
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: '操作',
      width: 100,
      render: (_, row) => (
        <Button type="link" onClick={() => navigate(`/questions/${row.id}`)}>
          {can('question.write') && ['draft', 'offline'].includes(row.status)
            ? '编辑'
            : '查看'}
        </Button>
      ),
    },
  ]

  return (
    <Card>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          题目管理
        </Typography.Title>
        <Button
          type="primary"
          disabled={!can('question.write')}
          onClick={() => navigate('/questions/new')}
        >
          新建题目
        </Button>
      </Space>
      <Space wrap style={{ marginBottom: 16 }}>
        <Select
          allowClear
          placeholder="学科"
          style={{ width: 110 }}
          options={Object.entries(subjectLabel).map(([value, label]) => ({
            value,
            label,
          }))}
          onChange={(value) => applyFilter('subject', value)}
        />
        <Select
          allowClear
          placeholder="学段"
          style={{ width: 110 }}
          options={Object.entries(stageLabel).map(([value, label]) => ({
            value,
            label,
          }))}
          onChange={(value) => applyFilter('stage', value)}
        />
        <Select
          allowClear
          placeholder="状态"
          style={{ width: 120 }}
          options={Object.entries(statusLabel).map(([value, label]) => ({
            value,
            label,
          }))}
          onChange={(value) => applyFilter('status', value)}
        />
        <Select
          allowClear
          placeholder="难度"
          style={{ width: 90 }}
          options={[1, 2, 3, 4, 5].map((value) => ({
            value: String(value),
            label: `${value} 星`,
          }))}
          onChange={(value) => applyFilter('difficulty', value)}
        />
        <Input.Search
          allowClear
          placeholder="关键词"
          style={{ width: 240 }}
          onSearch={(value) => applyFilter('q', value)}
        />
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        scroll={{ x: 1200 }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: false,
          onChange: (next) => {
            setPage(next)
            void load(next)
          },
        }}
      />
    </Card>
  )
}
