import { Card, Input, Space, Table, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { api } from '../api'
import type { AuditEntry } from '../types'

export default function AuditPage() {
  const [rows, setRows] = useState<AuditEntry[]>([])
  const [loading, setLoading] = useState(false)

  const load = async (targetId: string) => {
    setLoading(true)
    try {
      setRows(await api.audit(targetId || undefined))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load('')
  }, [])

  return (
    <Card>
      <Space style={{ marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          审计日志
        </Typography.Title>
        <Input.Search
          placeholder="按题目 ID 过滤"
          allowClear
          style={{ width: 360 }}
          onSearch={(value) => {
            void load(value)
          }}
        />
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
