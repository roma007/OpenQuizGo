import { Card, Col, Row, Statistic, Table, Typography, message } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { api, ApiError } from '../api'
import type { CoverageCell } from '../types'

const subjectLabel: Record<string, string> = {
  math: '数学', chinese: '语文', english: '英语', science: '科学',
  logic: '逻辑', safety: '安全', fun: '趣味',
}
const stageLabel: Record<string, string> = {
  preschool: '学龄前', low: '小学低', mid: '小学中', high: '小学高',
}
const statusLabel: Record<string, string> = {
  draft: '草稿', in_review: '待审核', approved: '已通过',
  online: '已上线', offline: '已下线', rejected: '已驳回',
}

export default function CoveragePage() {
  const [cells, setCells] = useState<CoverageCell[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    void (async () => {
      setLoading(true)
      try {
        const result = await api.coverage()
        setCells(result.cells || [])
        setTotal(result.total || 0)
      } catch (error) {
        message.error(error instanceof ApiError ? error.message : '加载失败')
      } finally {
        setLoading(false)
      }
    })()
  }, [])

  const byStatus = useMemo(() => {
    const map: Record<string, number> = {}
    cells.forEach((cell) => {
      map[cell.status] = (map[cell.status] || 0) + cell.count
    })
    return map
  }, [cells])

  const maxCount = Math.max(1, ...cells.map((cell) => cell.count))

  return (
    <Card loading={loading}>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        题库覆盖率
      </Typography.Title>
      <Row gutter={16} style={{ marginBottom: 20 }}>
        <Col span={4}><Statistic title="总题量" value={total} /></Col>
        <Col span={4}><Statistic title="已上线" value={byStatus.online || 0} /></Col>
        <Col span={4}><Statistic title="草稿" value={byStatus.draft || 0} /></Col>
        <Col span={4}><Statistic title="待审核" value={byStatus.in_review || 0} /></Col>
        <Col span={4}><Statistic title="已下线" value={byStatus.offline || 0} /></Col>
        <Col span={4}><Statistic title="聚合单元" value={cells.length} /></Col>
      </Row>
      <Table
        rowKey={(row) => `${row.subject}-${row.stage}-${row.type}-${row.status}-${row.difficulty}`}
        size="small"
        dataSource={cells}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        columns={[
          { title: '学科', dataIndex: 'subject', render: (v) => subjectLabel[v] || v },
          { title: '学段', dataIndex: 'stage', render: (v) => stageLabel[v] || v },
          { title: '题型', dataIndex: 'type' },
          { title: '状态', dataIndex: 'status', render: (v) => statusLabel[v] || v },
          { title: '难度', dataIndex: 'difficulty' },
          {
            title: '题量',
            dataIndex: 'count',
            render: (value: number) => {
              const ratio = value / maxCount
              return (
                <span
                  style={{
                    display: 'inline-block',
                    minWidth: 80,
                    padding: '2px 8px',
                    borderRadius: 4,
                    background: `rgba(24, 144, 255, ${0.08 + ratio * 0.5})`,
                  }}
                >
                  {value}
                </span>
              )
            },
          },
        ]}
      />
    </Card>
  )
}
