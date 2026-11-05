import {
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Upload,
  Tag,
  Typography,
  message,
} from 'antd'
import { UploadOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, ApiError } from '../api'
import { useAuth } from '../auth'
import type { AuditEntry, OptionItem, Question } from '../types'

const subjects = [
  ['math', '数学'],
  ['chinese', '语文'],
  ['english', '英语'],
  ['science', '科学'],
  ['logic', '逻辑'],
  ['safety', '安全'],
  ['fun', '趣味'],
]
const stages = [
  ['preschool', '学龄前'],
  ['low', '小学低年级'],
  ['mid', '小学中年级'],
  ['high', '小学高年级'],
]
const categories = [
  ['core', '核心'],
  ['literacy', '素养'],
  ['topic', '专题'],
  ['fun', '趣味'],
]
const types = [
  ['choice', '选择'],
  ['image', '看图'],
  ['fill', '填空'],
  ['voice', '语音'],
  ['sort', '排序'],
  ['match', '连线'],
]
const statusLabel: Record<string, string> = {
  draft: '草稿',
  review: '待审核',
  trial: '已通过',
  online: '已上线',
  offline: '已下线',
}
const MAX_OPTION_IMAGES = 3

// 旧数据可能只有 image，加载时并入 images 以统一展示。
function normalizeOption(option: OptionItem): OptionItem {
  const images = (option.images ?? []).filter((url) => url.trim())
  if (images.length === 0 && option.image?.trim()) {
    images.push(option.image)
  }
  return { ...option, images, image: images[0] }
}

interface QuestionForm {
  subject: string
  category: string
  stage: string
  grade_tag?: string
  difficulty: number
  type: string
  stem: string
  explain?: string
  duration_limit?: number
}

export default function QuestionEdit() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { can } = useAuth()
  const [form] = Form.useForm<QuestionForm>()
  const [question, setQuestion] = useState<Question | null>(null)
  const [options, setOptions] = useState<OptionItem[]>([
    { id: 'a', text: '' },
    { id: 'b', text: '' },
  ])
  const [answerIds, setAnswerIds] = useState<string[]>(['a'])
  const [answerJson, setAnswerJson] = useState('[]')
  const [optionsJson, setOptionsJson] = useState('')
  const [audit, setAudit] = useState<AuditEntry[]>([])
  const [saving, setSaving] = useState(false)
  const [loading, setLoading] = useState(Boolean(id))
  const [stemMediaUrl, setStemMediaUrl] = useState('')
  const [uploading, setUploading] = useState(false)
  const [uploadingOption, setUploadingOption] = useState<number | null>(null)
  const [uploadingOptionAudio, setUploadingOptionAudio] = useState<number | null>(null)

  const loadAudit = async (targetId: string) => {
    try {
      setAudit(await api.audit({ target_id: targetId }))
    } catch {
      // 审计不可用时不影响编辑
    }
  }

  useEffect(() => {
    if (!id) {
      form.setFieldsValue({
        subject: 'math',
        category: 'core',
        stage: 'low',
        difficulty: 2,
        type: 'choice',
        stem: '',
      })
      return
    }
    void (async () => {
      try {
        const result = await api.question(id)
        setQuestion(result)
        form.setFieldsValue({
          subject: result.subject,
          category: result.category,
          stage: result.stage,
          grade_tag: result.grade_tag,
          difficulty: result.difficulty,
          type: result.type,
          stem: result.stem,
          explain: result.explain,
          duration_limit: result.duration_limit || undefined,
        })
        if (result.stem_media && typeof result.stem_media === 'object') {
          setStemMediaUrl(result.stem_media.url || '')
        }
        if (Array.isArray(result.options)) {
          setOptions((result.options as OptionItem[]).map(normalizeOption))
        }
        if (Array.isArray(result.answer)) {
          setAnswerIds(result.answer as string[])
        }
        setAnswerJson(JSON.stringify(result.answer ?? []))
        setOptionsJson(result.options ? JSON.stringify(result.options) : '')
        void loadAudit(id)
      } catch (error) {
        message.error(error instanceof ApiError ? error.message : '加载失败')
      } finally {
        setLoading(false)
      }
    })()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id])

  const isChoice = () =>
    ['choice', 'image'].includes(form.getFieldValue('type'))

  const uploadOptionImage = async (index: number, file: File) => {
    setUploadingOption(index)
    try {
      const result = await api.uploadMedia(file)
      setOptions((prev) => {
        const next = [...prev]
        const current = next[index]
        if (!current) return prev
        const images = [...(current.images ?? []), result.url].slice(0, MAX_OPTION_IMAGES)
        next[index] = { ...current, images, image: images[0] }
        return next
      })
      message.success('选项图片已上传')
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '上传失败')
    } finally {
      setUploadingOption(null)
    }
  }

  const uploadOptionAudio = async (index: number, file: File) => {
    setUploadingOptionAudio(index)
    try {
      const result = await api.uploadMedia(file)
      setOptions((prev) => {
        const next = [...prev]
        const current = next[index]
        if (!current) return prev
        next[index] = { ...current, audio: result.url }
        return next
      })
      message.success('选项音频已上传')
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '上传失败')
    } finally {
      setUploadingOptionAudio(null)
    }
  }

  const removeOptionImage = (index: number, imageIndex: number) => {
    setOptions((prev) => {
      const next = [...prev]
      const current = next[index]
      if (!current) return prev
      const images = (current.images ?? []).filter((_, i) => i !== imageIndex)
      next[index] = { ...current, images, image: images[0] }
      return next
    })
  }

  // 统一写 images，并把首图写入 image 以兼容旧 TV 版本。
  const buildChoiceOptions = () =>
    options
      .filter(
        (item) =>
          item.id.trim() &&
          (item.text.trim() ||
            (item.images ?? []).length > 0 ||
            Boolean(item.image) ||
            Boolean(item.audio?.trim())),
      )
      .map((item) => {
        const images = (item.images ?? [])
          .filter((url) => url.trim())
          .slice(0, MAX_OPTION_IMAGES)
        return {
          id: item.id.trim(),
          text: item.text,
          images,
          image: images[0] ?? item.image,
          audio: item.audio?.trim() || undefined,
        }
      })

  const buildPayload = (values: QuestionForm) => {
    const choice = ['choice', 'image'].includes(values.type)
    return {
      subject: values.subject,
      category: values.category,
      stage: values.stage,
      grade_tag: values.grade_tag || '',
      difficulty: values.difficulty,
      type: values.type,
      stem: values.stem,
      stem_media: stemMediaUrl ? { url: stemMediaUrl } : null,
      options: choice
        ? buildChoiceOptions()
        : optionsJson.trim()
          ? JSON.parse(optionsJson)
          : null,
      answer: choice ? answerIds : JSON.parse(answerJson || 'null'),
      answer_alias: null,
      explain: values.explain || '',
      duration_limit: values.duration_limit || null,
      topic_id: null,
      schema_ver: 1,
    }
  }

  const save = async () => {
    try {
      const values = await form.validateFields()
      const payload = buildPayload(values)
      setSaving(true)
      if (id) {
        const updated = await api.updateQuestion(id, payload)
        setQuestion(updated)
        setAnswerJson(JSON.stringify(updated.answer ?? []))
        setOptionsJson(updated.options ? JSON.stringify(updated.options) : '')
        message.success('已保存')
        void loadAudit(id)
      } else {
        const created = await api.createQuestion(payload)
        message.success('已创建草稿')
        navigate(`/questions/${created.id}`, { replace: true })
      }
    } catch (error) {
      if (error instanceof ApiError) message.error(error.message)
      else if (error instanceof Error && !('errorFields' in error)) {
        message.error('请求体格式不正确')
      }
    } finally {
      setSaving(false)
    }
  }

  const runAction = async (action: string, approve?: boolean) => {
    if (!id) return
    try {
      const result =
        action === 'review'
          ? await api.review(id, Boolean(approve))
          : await api.action(id, action)
      setQuestion(result)
      message.success('操作成功')
      void loadAudit(id)
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '操作失败')
    }
  }

  const status = question?.status || 'draft'
  const editable = !id || ['draft', 'offline'].includes(status)
  const canWrite = can('question.write')
  const canReview = can('question.review')
  const canPublish = can('question.publish')

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card
        loading={loading}
        title={
          <Space>
            <span>{id ? '题目详情' : '新建题目'}</span>
            {question && (
              <Tag color={status === 'online' ? 'success' : status === 'review' ? 'blue' : 'default'}>
                {statusLabel[status] || status}
              </Tag>
            )}
          </Space>
        }
        extra={
          <Space>
            <Button onClick={() => navigate('/questions')}>返回</Button>
            <Button
              type="primary"
              onClick={save}
              loading={saving}
              disabled={!canWrite || !editable}
            >
              保存
            </Button>
          </Space>
        }
      >
        <Form form={form} layout="vertical" disabled={!editable}>
          <Row gutter={16}>
            <Col span={4}>
              <Form.Item name="subject" label="学科" rules={[{ required: true }]}>
                <Select options={subjects.map(([v, l]) => ({ value: v, label: l }))} />
              </Form.Item>
            </Col>
            <Col span={4}>
              <Form.Item name="category" label="分类" rules={[{ required: true }]}>
                <Select options={categories.map(([v, l]) => ({ value: v, label: l }))} />
              </Form.Item>
            </Col>
            <Col span={4}>
              <Form.Item name="stage" label="学段" rules={[{ required: true }]}>
                <Select options={stages.map(([v, l]) => ({ value: v, label: l }))} />
              </Form.Item>
            </Col>
            <Col span={3}>
              <Form.Item name="difficulty" label="难度" rules={[{ required: true }]}>
                <InputNumber min={1} max={5} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={4}>
              <Form.Item name="type" label="题型" rules={[{ required: true }]}>
                <Select options={types.map(([v, l]) => ({ value: v, label: l }))} />
              </Form.Item>
            </Col>
            <Col span={5}>
              <Form.Item name="grade_tag" label="年级标签">
                <Input placeholder="g2" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="stem" label="题干" rules={[{ required: true }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item label="题干图片">
            <Space orientation="vertical" size={8}>
              {stemMediaUrl && (
                <img
                  src={stemMediaUrl}
                  alt="题干图片"
                  style={{ maxWidth: 280, maxHeight: 200, objectFit: 'contain' }}
                />
              )}
              <Space>
                <Upload
                  accept="image/jpeg,image/png,image/webp,image/gif"
                  showUploadList={false}
                  beforeUpload={(file) => {
                    void (async () => {
                      setUploading(true)
                      try {
                        const result = await api.uploadMedia(file as File)
                        setStemMediaUrl(result.url)
                        message.success('图片已上传')
                      } catch (error) {
                        message.error(
                          error instanceof ApiError ? error.message : '上传失败',
                        )
                      } finally {
                        setUploading(false)
                      }
                    })()
                    return false
                  }}
                >
                  <Button icon={<UploadOutlined />} loading={uploading}>
                    上传图片
                  </Button>
                </Upload>
                {stemMediaUrl && (
                  <Button danger onClick={() => setStemMediaUrl('')}>
                    移除
                  </Button>
                )}
              </Space>
            </Space>
          </Form.Item>
          <Form.Item name="explain" label="解析">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Row gutter={16}>
            <Col span={6}>
              <Form.Item name="duration_limit" label="单题限时（秒）">
                <InputNumber min={5} max={600} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
        </Form>

        <Divider titlePlacement="start">选项与答案</Divider>
        {isChoice() ? (
          <Space direction="vertical" style={{ width: '100%' }}>
            {options.map((option, index) => {
              const images = option.images ?? []
              return (
                <Space
                  key={index}
                  direction="vertical"
                  size={8}
                  style={{
                    width: '100%',
                    padding: 12,
                    border: '1px solid #f0f0f0',
                    borderRadius: 8,
                  }}
                >
                  <Space wrap>
                    <Input
                      style={{ width: 80 }}
                      value={option.id}
                      placeholder="id"
                      onChange={(event) => {
                        const next = [...options]
                        next[index] = { ...next[index], id: event.target.value }
                        setOptions(next)
                      }}
                    />
                    <Input
                      style={{ width: 360 }}
                      value={option.text}
                      placeholder="选项内容（可留空，仅图片）"
                      onChange={(event) => {
                        const next = [...options]
                        next[index] = { ...next[index], text: event.target.value }
                        setOptions(next)
                      }}
                    />
                    <Upload
                      accept="image/jpeg,image/png,image/webp,image/gif"
                      showUploadList={false}
                      beforeUpload={(file) => {
                        void uploadOptionImage(index, file as File)
                        return false
                      }}
                    >
                      <Button
                        icon={<UploadOutlined />}
                        loading={uploadingOption === index}
                        disabled={images.length >= MAX_OPTION_IMAGES}
                      >
                        图片 {images.length}/{MAX_OPTION_IMAGES}
                      </Button>
                    </Upload>
                    <Upload
                      accept="audio/mpeg,audio/wav,audio/x-wav,audio/mp4,audio/ogg"
                      showUploadList={false}
                      beforeUpload={(file) => {
                        void uploadOptionAudio(index, file as File)
                        return false
                      }}
                    >
                      <Button
                        icon={<UploadOutlined />}
                        loading={uploadingOptionAudio === index}
                      >
                        音频
                      </Button>
                    </Upload>
                    <Button
                      danger
                      onClick={() => setOptions(options.filter((_, i) => i !== index))}
                    >
                      删除
                    </Button>
                  </Space>
                  {option.audio && (
                    <Space wrap>
                      <audio controls src={option.audio} style={{ height: 32 }} />
                      <Button
                        size="small"
                        danger
                        onClick={() => {
                          const next = [...options]
                          next[index] = { ...next[index], audio: undefined }
                          setOptions(next)
                        }}
                      >
                        移除音频
                      </Button>
                    </Space>
                  )}
                  {images.length > 0 && (
                    <Space wrap size={8}>
                      {images.map((url, imageIndex) => (
                        <Space
                          key={`${url}-${imageIndex}`}
                          direction="vertical"
                          size={4}
                          style={{ alignItems: 'center' }}
                        >
                          <img
                            src={url}
                            alt={`选项 ${option.id} 图 ${imageIndex + 1}`}
                            style={{ width: 96, height: 72, objectFit: 'contain' }}
                          />
                          <Button
                            size="small"
                            danger
                            onClick={() => removeOptionImage(index, imageIndex)}
                          >
                            移除
                          </Button>
                        </Space>
                      ))}
                    </Space>
                  )}
                </Space>
              )
            })}
            <Space>
              <Button
                onClick={() =>
                  setOptions([
                    ...options,
                    { id: String.fromCharCode(97 + options.length), text: '' },
                  ])
                }
              >
                添加选项
              </Button>
              <Select
                mode="multiple"
                placeholder="正确答案（选项 id）"
                style={{ minWidth: 260 }}
                value={answerIds}
                options={options.map((item) => ({ value: item.id, label: item.id }))}
                onChange={setAnswerIds}
              />
            </Space>
          </Space>
        ) : (
          <Row gutter={16}>
            <Col span={12}>
              <Typography.Text type="secondary">选项 JSON（可选）</Typography.Text>
              <Input.TextArea
                rows={6}
                value={optionsJson}
                onChange={(event) => setOptionsJson(event.target.value)}
              />
            </Col>
            <Col span={12}>
              <Typography.Text type="secondary">答案 JSON</Typography.Text>
              <Input.TextArea
                rows={6}
                value={answerJson}
                onChange={(event) => setAnswerJson(event.target.value)}
              />
            </Col>
          </Row>
        )}

        {question && (
          <>
            <Divider titlePlacement="start">工作流</Divider>
            <Space wrap>
              {status === 'draft' && canWrite && (
                <Button onClick={() => runAction('submit-review')}>提交审核</Button>
              )}
              {status === 'review' && canReview && (
                <>
                  <Button onClick={() => runAction('review', true)}>审核通过</Button>
                  <Button danger onClick={() => runAction('review', false)}>
                    驳回
                  </Button>
                </>
              )}
              {status === 'trial' && canPublish && (
                <Button type="primary" onClick={() => runAction('publish')}>
                  上线
                </Button>
              )}
              {status === 'online' && canPublish && (
                <Popconfirm
                  title="确认下线该题？"
                  onConfirm={() => runAction('offline')}
                >
                  <Button danger>下线</Button>
                </Popconfirm>
              )}
              {status === 'offline' && canWrite && (
                <Button onClick={() => runAction('resubmit')}>重新提交审核</Button>
              )}
              {status === 'draft' && canWrite && (
                <Popconfirm
                  title="确认删除草稿？"
                  onConfirm={async () => {
                    if (!id) return
                    await api.deleteQuestion(id)
                    message.success('已删除')
                    navigate('/questions')
                  }}
                >
                  <Button danger>删除</Button>
                </Popconfirm>
              )}
            </Space>
            <Divider titlePlacement="start">审计</Divider>
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={audit}
              columns={[
                { title: '动作', dataIndex: 'action', width: 160 },
                {
                  title: '时间',
                  dataIndex: 'created_at',
                  width: 200,
                  render: (value: string) => new Date(value).toLocaleString(),
                },
                {
                  title: '操作人',
                  dataIndex: 'actor_id',
                  ellipsis: true,
                },
              ]}
            />
            <Descriptions size="small" column={3} style={{ marginTop: 16 }}>
              <Descriptions.Item label="ID">{question.id}</Descriptions.Item>
              <Descriptions.Item label="版本">v{question.version}</Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {new Date(question.created_at).toLocaleString()}
              </Descriptions.Item>
            </Descriptions>
          </>
        )}
      </Card>
    </Space>
  )
}
