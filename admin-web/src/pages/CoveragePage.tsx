import { Alert, Button, Card, Col, InputNumber, Row, Space, Statistic, Table, Typography, message } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { api, ApiError } from '../api'
import type { CoverageCell, CoverageSnapshot, TopicCoverageCell } from '../types'

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
  const [warnings, setWarnings] = useState<string[]>([])
  const [topicCells, setTopicCells] = useState<TopicCoverageCell[]>([])
  const [topicTotal, setTopicTotal] = useState(0)
  const [topicWarnings, setTopicWarnings] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [threshold, setThreshold] = useState(50)
  const [trend, setTrend] = useState<CoverageSnapshot[]>([])
  const [snapshotting, setSnapshotting] = useState(false)

  const load = async (next = threshold) => {
    setLoading(true)
    try {
      const [result, topics, snapshots] = await Promise.all([
        api.coverage(next),
        api.coverageTopics(next),
        api.coverageTrend(30),
      ])
      setCells(result.cells || [])
      setTotal(result.total || 0)
      setWarnings(result.warnings || [])
      setTopicCells(topics.cells || [])
      setTopicTotal(topics.total || 0)
      setTopicWarnings(topics.warnings || [])
      setTrend(snapshots || [])
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

  const byStatus = useMemo(() => {
    const map: Record<string, number> = {}
    cells.forEach((cell) => {
      map[cell.status] = (map[cell.status] || 0) + cell.count
    })
    return map
  }, [cells])

  const maxCount = Math.max(1, ...cells.map((cell) => cell.count))
  const topicMaxCount = Math.max(1, ...topicCells.map((cell) => cell.count))

  return (
    <Card loading={loading}>
      <Space style={{ width: '100%', justifyContent: 'space-between' }} wrap>
        <Space>
          <Typography.Title level={4} style={{ marginTop: 0 }}>
            题库覆盖率
          </Typography.Title>
          <span>
            告警阈值
            <InputNumber
              style={{ width: 110, marginInline: 8 }}
              min={1}
              max={10000}
              value={threshold}
              onChange={(value) => setThreshold(value || 50)}
            />
            <Button onClick={() => void load(threshold)}>刷新</Button>
          </span>
        </Space>
        <Space>
          <Button
            loading={snapshotting}
            onClick={async () => {
              setSnapshotting(true)
              try {
                const snapshot = await api.coverageSnapshot()
                message.success(`已记录 ${snapshot.captured_on} 快照`)
                await load(threshold)
              } catch (error) {
                message.error(error instanceof ApiError ? error.message : '快照失败')
              } finally {
                setSnapshotting(false)
              }
            }}
          >
            记录今日快照
          </Button>
          <Button
            onClick={async () => {
              try {
                const blob = await api.coverageExport(threshold)
                const url = URL.createObjectURL(blob)
                const link = document.createElement('a')
                link.href = url
                link.download = 'coverage.csv'
                link.click()
                URL.revokeObjectURL(url)
              } catch (error) {
                message.error(error instanceof ApiError ? error.message : '导出失败')
              }
            }}
          >
            导出 CSV
          </Button>
          <Button
            onClick={async () => {
              try {
                const blob = await api.coverageTopicsExport(threshold)
                const url = URL.createObjectURL(blob)
                const link = document.createElement('a')
                link.href = url
                link.download = 'coverage-topics.csv'
                link.click()
                URL.revokeObjectURL(url)
              } catch (error) {
                message.error(error instanceof ApiError ? error.message : '导出失败')
              }
            }}
          >
            导出专题 CSV
          </Button>
        </Space>
      </Space>
      {warnings.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          title={`覆盖不足告警（${warnings.length} 项，阈值 ${threshold}）`}
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {warnings.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          }
        />
      )}
      {topicWarnings.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          title={`专题覆盖不足告警（${topicWarnings.length} 项，阈值 ${threshold}）`}
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {topicWarnings.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          }
        />
      )}
      <Row gutter={16} style={{ marginBottom: 20 }}>
        <Col span={4}><Statistic title="总题量" value={total} /></Col>
        <Col span={4}><Statistic title="已上线" value={byStatus.online || 0} /></Col>
        <Col span={4}><Statistic title="草稿" value={byStatus.draft || 0} /></Col>
        <Col span={4}><Statistic title="待审核" value={byStatus.in_review || 0} /></Col>
        <Col span={4}><Statistic title="已下线" value={byStatus.offline || 0} /></Col>
        <Col span={4}><Statistic title="聚合单元" value={cells.length} /></Col>
      </Row>
      <Typography.Title level={5} style={{ marginTop: 8 }}>
        覆盖率趋势（近 30 天）
      </Typography.Title>
      {trend.length === 0 ? (
        <span style={{ color: '#999', display: 'inline-block', marginBottom: 16 }}>
          暂无快照，可点击「记录今日快照」开始积累
        </span>
      ) : (
        <Space direction="vertical" size={6} style={{ width: '100%', marginBottom: 16 }}>
          {trend.map((point) => {
            const max = Math.max(...trend.map((item) => item.total), 1)
            return (
              <Space key={point.captured_on} style={{ width: '100%' }}>
                <span style={{ width: 96, display: 'inline-block' }}>{point.captured_on}</span>
                <span
                  style={{
                    display: 'inline-block',
                    width: `${Math.max(4, (point.total / max) * 320)}px`,
                    height: 10,
                    borderRadius: 4,
                    background: '#52c41a',
                  }}
                />
                <span>
                  总量 {point.total} / 上线 {point.online}
                </span>
              </Space>
            )
          })}
        </Space>
      )}
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
      <Typography.Title level={5} style={{ marginTop: 24 }}>
        专题覆盖（去重题量 {topicTotal}）
      </Typography.Title>
      <Table
        rowKey={(row) => `${row.topic_id || 'unassigned'}-${row.status}`}
        size="small"
        dataSource={topicCells}
        pagination={{ pageSize: 20, showSizeChanger: false }}
        columns={[
          {
            title: '专题',
            dataIndex: 'topic_name',
            render: (value: string) => value || '未分专题',
          },
          { title: '状态', dataIndex: 'status', render: (v) => statusLabel[v] || v },
          {
            title: '题量',
            dataIndex: 'count',
            render: (value: number) => {
              const ratio = value / topicMaxCount
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
