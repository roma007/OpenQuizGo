import { Alert, Button, Card, Input, Space, Tabs, Typography, message } from 'antd'
import { useState } from 'react'
import { api, ApiError } from '../api'

export default function ImportPage() {
  const [mode, setMode] = useState('json')
  const [text, setText] = useState('')
  const [result, setResult] = useState<string | null>(null)
  const [errors, setErrors] = useState<any[]>([])
  const [loading, setLoading] = useState(false)

  const submit = async () => {
    setLoading(true)
    setResult(null)
    setErrors([])
    try {
      const body =
        mode === 'csv' ? text : JSON.stringify({ questions: JSON.parse(text) })
      const response = await api.importQuestions(
        body,
        mode === 'csv' ? 'text/csv' : 'application/json',
      )
      setResult(`成功导入 ${response.imported} 题`)
    } catch (error) {
      if (error instanceof ApiError) {
        setResult(error.message)
        const detail = error.details as { errors?: any[] } | undefined
        setErrors(detail?.errors || [])
      } else {
        message.error(String(error))
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <Card title="批量导入">
      <Tabs
        activeKey={mode}
        onChange={setMode}
        items={[
          {
            key: 'json',
            label: 'JSON',
            children: (
              <Input.TextArea
                rows={14}
                value={text}
                onChange={(event) => setText(event.target.value)}
                placeholder='[{"subject":"math","category":"core","stage":"low","difficulty":2,"type":"choice","stem":"1+1=?","options":[{"id":"a","text":"2"},{"id":"b","text":"3"}],"answer":["a"]}]'
              />
            ),
          },
          {
            key: 'csv',
            label: 'CSV',
            children: (
              <Input.TextArea
                rows={14}
                value={text}
                onChange={(event) => setText(event.target.value)}
                placeholder="subject,category,stage,difficulty,type,stem,options,answer"
              />
            ),
          },
        ]}
      />
      <Space style={{ marginTop: 16 }}>
        <Button type="primary" loading={loading} onClick={submit}>
          导入
        </Button>
        <Typography.Text type="secondary">
          任一题校验失败会整批拒绝；单次最多 2000 题。
        </Typography.Text>
      </Space>
      {result && (
        <Alert
          style={{ marginTop: 16 }}
          type={result.startsWith('成功') ? 'success' : 'error'}
          message={result}
        />
      )}
      {errors.length > 0 && (
        <Alert
          style={{ marginTop: 12 }}
          type="error"
          message="校验失败"
          description={
            <ul style={{ margin: 0, paddingLeft: 18 }}>
              {errors.map((item, index) => (
                <li key={index}>
                  第 {item.row || '?'} 行 · {item.field}：{item.message}
                </li>
              ))}
            </ul>
          }
        />
      )}
    </Card>
  )
}
