import { Button, Card, Input, Select, Space, Table, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { api } from '../api'
import type { AuditEntry } from '../types'

export default function AuditPage() {
  const [rows, setRows] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [filters, setFilters] = useState({
    target_id: '',
    action: '',
    target_type: '',
    actor_id: '',
  })

  const load = async (next = filters) => {
    setLoading(true)
    try {
      setRows(
        await api.audit({
          target_id: next.target_id || undefined,
          action: next.action || undefined,
          target_type: next.target_type || undefined,
          actor_id: next.actor_id || undefined,
        }),
      )
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const update = (key: keyof typeof filters, value?: string) => {
    const next = { ...filters, [key]: value || '' }
    setFilters(next)
    void load(next)
  }

  return (
    <Card>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          审计日志
        </Typography.Title>
        <Input.Search
          placeholder="对象 ID"
          allowClear
          style={{ width: 280 }}
          onSearch={(value) => update('target_id', value)}
        />
        <Select
          allowClear
          placeholder="动作"
          style={{ width: 220 }}
          value={filters.action || undefined}
          options={[
            'question.create',
            'question.update',
            'question.submit_review',
            'question.review',
            'question.publish',
            'question.offline',
            'question.import',
            'question.calibrate',
            'question.delete',
            'membership.order.refund',
            'device.unlock',
            'rule.update',
          ].map((value) => ({ value, label: value }))}
          onChange={(value) => update('action', value)}
        />
        <Select
          allowClear
          placeholder="对象类型"
          style={{ width: 160 }}
          value={filters.target_type || undefined}
          options={[
            { value: 'question', label: 'question' },
            { value: 'membership_order', label: 'membership_order' },
          ]}
          onChange={(value) => update('target_type', value)}
        />
        <Input
          placeholder="操作人 ID"
          allowClear
          style={{ width: 260 }}
          value={filters.actor_id}
          onChange={(event) => update('actor_id', event.target.value)}
        />
        <Button
          onClick={() => {
            const empty = { target_id: '', action: '', target_type: '', actor_id: '' }
            setFilters(empty)
            void load(empty)
          }}
        >
          重置
        </Button>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        dataSource={rows}
        columns={[
          { title: '动作', dataIndex: 'action', width: 180 },
          {
            title: '对象',
            dataIndex: 'target_id',
            ellipsis: true,
          },
          { title: '操作人类型', dataIndex: 'actor_type', width: 120 },
          {
            title: '时间',
            dataIndex: 'created_at',
            width: 200,
            render: (value: string) => new Date(value).toLocaleString(),
          },
        ]}
      />
    </Card>
  )
}
