import {
  Alert,
  Button,
  Card,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { CalibrationHistoryItem, CalibrationItem } from '../types'

const subjectLabel: Record<string, string> = {
  math: '数学', chinese: '语文', english: '英语', science: '科学',
  logic: '逻辑', safety: '安全', fun: '趣味',
}
const stageLabel: Record<string, string> = {
  preschool: '学龄前', low: '小学低', mid: '小学中', high: '小学高',
}

export default function CalibrationPage() {
  const [minAnswers, setMinAnswers] = useState(20)
  const [items, setItems] = useState<CalibrationItem[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [applying, setApplying] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  const [historyLoading, setHistoryLoading] = useState(false)
  const [historyItems, setHistoryItems] = useState<CalibrationHistoryItem[]>([])
  const [historyQuestion, setHistoryQuestion] = useState<CalibrationItem | null>(null)
  const [rollingBack, setRollingBack] = useState(false)

  const load = async (threshold = minAnswers) => {
    setLoading(true)
    try {
      const result = await api.calibrationPreview(threshold)
      setItems(result.items || [])
      setSelected(
        (result.items || [])
          .filter((item) => item.suggested_difficulty !== item.difficulty)
          .map((item) => item.question_id),
      )
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

  const apply = async () => {
    const chosen = items.filter((item) => selected.includes(item.question_id))
    if (chosen.length === 0) return
    setApplying(true)
    try {
      const result = await api.calibrationApply(
        chosen.map((item) => ({
          question_id: item.question_id,
          difficulty: item.suggested_difficulty,
        })),
      )
      message.success(`已应用 ${result.applied} 题`)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '应用失败')
    } finally {
      setApplying(false)
    }
  }

  const changeCount = items.filter(
    (item) => item.suggested_difficulty !== item.difficulty,
  ).length

  const openHistory = async (row: CalibrationItem) => {
    setHistoryQuestion(row)
    setHistoryOpen(true)
    setHistoryLoading(true)
    setHistoryItems([])
    try {
      const result = await api.calibrationHistory(row.question_id)
      setHistoryItems(result || [])
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载历史失败')
      setHistoryItems([])
    } finally {
      setHistoryLoading(false)
    }
  }

  const rollback = async (entry: CalibrationHistoryItem) => {
    if (!historyQuestion) return
    setRollingBack(true)
    try {
      const result = await api.calibrationApply([
        { question_id: historyQuestion.question_id, difficulty: entry.from },
      ])
      message.success(`已回滚 ${result.applied} 题`)
      setHistoryOpen(false)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '回滚失败')
    } finally {
      setRollingBack(false)
    }
  }

  return (
    <Card loading={loading}>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          难度校准
        </Typography.Title>
        <span>
          样本门槛
          <InputNumber
            style={{ width: 90, marginInline: 8 }}
            min={5}
            max={1000}
            value={minAnswers}
            onChange={(value) => setMinAnswers(value || 20)}
          />
          <Button onClick={() => load(minAnswers)}>刷新</Button>
        </span>
        <Popconfirm
          title={`确认应用 ${selected.length} 题的难度调整？`}
          onConfirm={apply}
          disabled={selected.length === 0}
        >
          <Button type="primary" disabled={selected.length === 0} loading={applying}>
            应用选中（{selected.length}）
          </Button>
        </Popconfirm>
      </Space>
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message={`正确率 ≥ 85% 时难度 +1，≤ 40% 时难度 -1；当前 ${items.length} 题达样本门槛，${changeCount} 题有建议。`}
      />
      <Table
        rowKey="question_id"
        size="small"
        dataSource={items}
        scroll={{ x: 1000 }}
        rowSelection={{
          selectedRowKeys: selected,
          onChange: (keys) => setSelected(keys as string[]),
          getCheckboxProps: (row) => ({
            disabled: row.suggested_difficulty === row.difficulty,
          }),
        }}
        columns={[
          { title: '题干', dataIndex: 'stem', ellipsis: true, width: 320 },
          {
            title: '学科/学段',
            width: 130,
            render: (_, row) =>
              `${subjectLabel[row.subject] || row.subject} / ${stageLabel[row.stage] || row.stage}`,
          },
          {
            title: '难度',
            width: 140,
            render: (_, row) => (
              <Space>
                <span>{row.difficulty}</span>
                <span>→</span>
                <Tag color={row.suggested_difficulty === row.difficulty ? 'default' : 'blue'}>
                  {row.suggested_difficulty}
                </Tag>
              </Space>
            ),
          },
          { title: '作答', dataIndex: 'answered', width: 80 },
          {
            title: '正确率',
            width: 100,
            render: (_, row) => `${(row.accuracy * 100).toFixed(0)}%`,
          },
          { title: '建议', dataIndex: 'reason' },
          {
            title: '历史',
            width: 80,
            render: (_, row) => (
              <Button size="small" onClick={() => openHistory(row)}>
                历史
              </Button>
            ),
          },
        ]}
      />
      <Modal
        title={`校准历史${historyQuestion ? `：${historyQuestion.stem}` : ''}`}
        open={historyOpen}
        onCancel={() => setHistoryOpen(false)}
        footer={null}
        width={720}
      >
        <Table
          rowKey="id"
          size="small"
          loading={historyLoading}
          dataSource={historyItems}
          pagination={false}
          locale={{ emptyText: '暂无校准记录' }}
          columns={[
            {
              title: '难度变化',
              width: 120,
              render: (_, row) => `${row.from} → ${row.to}`,
            },
            {
              title: '时间',
              width: 180,
              render: (_, row) => new Date(row.created_at).toLocaleString(),
            },
            { title: '操作人', dataIndex: 'actor_id', ellipsis: true },
            {
              title: '操作',
              width: 150,
              render: (_, row) => (
                <Popconfirm
                  title={`确认回滚到难度 ${row.from}？`}
                  description="回滚会生成一条新的校准记录。"
                  onConfirm={() => rollback(row)}
                  disabled={rollingBack}
                >
                  <Button size="small" loading={rollingBack} disabled={rollingBack}>
                    回滚到此难度
                  </Button>
                </Popconfirm>
              ),
            },
          ]}
        />
      </Modal>
    </Card>
  )
}
