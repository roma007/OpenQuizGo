import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useState } from 'react'
import { api, ApiError } from '../api'
import type { QuizPreviewResult, QuizSelected } from '../types'

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

const initialValues = {
  child_id: '',
  stage: 'low',
  grade_tag: 'g2',
  unlock_minutes: 20,
  pass_threshold: 3,
  math: 2,
  chinese: 1,
  english: 1,
  science: 1,
  fun: 0.5,
  fun_enabled: true,
  fun_ratio_cap: 2,
  daily_limit: 60,
  timezone: 'Asia/Shanghai',
  max_questions: 0,
  seed: 0,
}

interface PreviewError {
  code?: string
  message: string
  warnings?: string[]
}

export default function QuizPreviewPage() {
  const [form] = Form.useForm()
  const funEnabled = Form.useWatch('fun_enabled', form)
  const [result, setResult] = useState<QuizPreviewResult | null>(null)
  const [error, setError] = useState<PreviewError | null>(null)
  const [loading, setLoading] = useState(false)

  const submit = async () => {
    const values = await form.validateFields()
    const payload = {
      child_id: values.child_id?.trim() || undefined,
      stage: values.stage,
      grade_tag: values.grade_tag?.trim() || undefined,
      max_questions: values.max_questions || 0,
      seed: values.seed || 0,
      rule: {
        unlock_minutes: values.unlock_minutes,
        pass_threshold: values.pass_threshold,
        subject_weight: {
          math: values.math,
          chinese: values.chinese,
          english: values.english,
          science: values.science,
          fun: values.fun,
        },
        fun_enabled: Boolean(values.fun_enabled),
        fun_ratio_cap: values.fun_ratio_cap,
        daily_limit: values.daily_limit,
        timezone: values.timezone,
      },
    }
    setLoading(true)
    setError(null)
    try {
      const data = await api.quizPreview(payload)
      setResult(data)
      message.success(`组卷成功，共 ${data.total} 题`)
    } catch (err) {
      setResult(null)
      if (err instanceof ApiError) {
        const details = err.details as { warnings?: string[] } | undefined
        setError({ code: err.code, message: err.message, warnings: details?.warnings })
      } else {
        setError({ message: '预览失败' })
      }
    } finally {
      setLoading(false)
    }
  }

  const columns: ColumnsType<QuizSelected> = [
    {
      title: '#',
      width: 50,
      render: (_, __, index) => index + 1,
    },
    {
      title: '题目 ID',
      dataIndex: 'question_id',
      ellipsis: true,
      render: (value: string) => <Typography.Text copyable>{value}</Typography.Text>,
    },
    {
      title: '学科',
      dataIndex: 'subject',
      width: 90,
      render: (value: string) => subjectLabel[value] || value,
    },
    {
      title: '学段',
      dataIndex: 'stage',
      width: 90,
      render: (value: string) => stageLabel[value] || value,
    },
    { title: '类型', dataIndex: 'type', width: 90 },
    { title: '难度', dataIndex: 'difficulty', width: 70 },
    {
      title: '重复',
      dataIndex: 'repeated',
      width: 80,
      render: (value: boolean) => (value ? <Tag color="orange">近 30 天已做</Tag> : '—'),
    },
  ]

  return (
    <Space orientation="vertical" size={16} style={{ display: 'flex' }}>
      <Card>
        <Space style={{ marginBottom: 8 }} wrap>
          <Typography.Title level={4} style={{ margin: 0 }}>
            组卷预览
          </Typography.Title>
          <Typography.Text type="secondary">仅预览，不生成答题会话</Typography.Text>
        </Space>
        <Form form={form} layout="vertical" initialValues={initialValues}>
          <Row gutter={16}>
            <Col xs={24} md={8}>
              <Form.Item name="child_id" label="孩子 ID（可选）">
                <Input placeholder="填写后按孩子学段抽题" allowClear />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="stage" label="学段" rules={[{ required: true }]}>
                <Select
                  options={Object.entries(stageLabel).map(([value, label]) => ({ value, label }))}
                />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="grade_tag" label="年级标签">
                <Input placeholder="如 g2" allowClear />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="max_questions" label="最大题数">
                <InputNumber min={0} max={20} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="seed" label="随机种子">
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col xs={12} md={4}>
              <Form.Item name="unlock_minutes" label="解锁时长（分钟）">
                <InputNumber min={1} max={240} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="pass_threshold" label="加权门槛">
                <InputNumber min={0.5} step={0.5} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="daily_limit" label="每日上限（分钟）">
                <InputNumber min={1} max={720} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="timezone" label="时区">
                <Input />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="fun_ratio_cap" label="趣味题上限">
                <InputNumber min={0} max={10} style={{ width: '100%' }} disabled={!funEnabled} />
              </Form.Item>
            </Col>
            <Col xs={12} md={4}>
              <Form.Item name="fun_enabled" label="启用趣味题" valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            {['math', 'chinese', 'english', 'science', 'fun'].map((key) => (
              <Col xs={12} md={4} key={key}>
                <Form.Item name={key} label={`${subjectLabel[key]}权重`}>
                  <InputNumber min={0} step={0.5} style={{ width: '100%' }} />
                </Form.Item>
              </Col>
            ))}
          </Row>
          <Space>
            <Button type="primary" loading={loading} onClick={() => void submit()}>
              开始预览
            </Button>
            <Button
              onClick={() => {
                form.resetFields()
                setResult(null)
                setError(null)
              }}
            >
              重置
            </Button>
          </Space>
        </Form>
      </Card>

      {error && (
        <Alert
          type="error"
          showIcon
          title={`预览失败${error.code ? `（${error.code}）` : ''}：${error.message}`}
          description={
            error.warnings && error.warnings.length > 0 ? (
              <ul style={{ margin: 0, paddingLeft: 20 }}>
                {error.warnings.map((warning) => (
                  <li key={warning}>{warning}</li>
                ))}
              </ul>
            ) : undefined
          }
        />
      )}

      {result && (
        <Card
          title={result.topic_name ? `专题模式：${result.topic_name}` : '普通组卷'}
          extra={<Typography.Text type="secondary">种子 {result.seed}</Typography.Text>}
        >
          <Descriptions column={{ xs: 1, md: 3 }} style={{ marginBottom: 16 }}>
            <Descriptions.Item label="题数">{result.total}</Descriptions.Item>
            <Descriptions.Item label="最高加权分">{result.max_score}</Descriptions.Item>
            <Descriptions.Item label="目标难度">{result.difficulty_target}</Descriptions.Item>
            <Descriptions.Item label="专题 ID">{result.topic_id || '—'}</Descriptions.Item>
            <Descriptions.Item label="学科分布">
              <Space wrap>
                {Object.entries(result.quota || {}).map(([subject, count]) => (
                  <Tag key={subject}>
                    {subjectLabel[subject] || subject}: {count}
                  </Tag>
                ))}
              </Space>
            </Descriptions.Item>
          </Descriptions>
          {result.warnings && result.warnings.length > 0 && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
              title="组卷警告"
              description={
                <ul style={{ margin: 0, paddingLeft: 20 }}>
                  {result.warnings.map((warning) => (
                    <li key={warning}>{warning}</li>
                  ))}
                </ul>
              }
            />
          )}
          <Table
            rowKey="question_id"
            size="small"
            columns={columns}
            dataSource={result.selected}
            pagination={false}
          />
        </Card>
      )}
    </Space>
  )
}
