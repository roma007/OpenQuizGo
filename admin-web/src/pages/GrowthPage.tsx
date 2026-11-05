import { Button, Card, Col, Row, Select, Space, Statistic, Table, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { GrowthReport } from '../types'

export default function GrowthPage() {
  const [days, setDays] = useState(30)
  const [report, setReport] = useState<GrowthReport | null>(null)
  const [loading, setLoading] = useState(false)

  const load = async (value = days) => {
    setLoading(true)
    try {
      setReport(await api.growth(value))
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

  const dailyMax = Math.max(1, ...(report?.daily || []).map((p) => p.new_accounts))
  return (
    <Card loading={loading}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          增长分析
        </Typography.Title>
        <Space>
          <Select
            style={{ width: 140 }}
            value={days}
            onChange={(value) => {
              setDays(value)
              void load(value)
            }}
            options={[
              { value: 7, label: '近 7 天' },
              { value: 30, label: '近 30 天' },
              { value: 90, label: '近 90 天' },
            ]}
          />
          <Button onClick={() => void load()}>刷新</Button>
        </Space>
      </Space>

      <Typography.Title level={5}>转化漏斗</Typography.Title>
      <Row gutter={12} style={{ marginBottom: 20 }}>
        <Col span={4}><Statistic title="注册账号" value={report?.funnel.registered || 0} /></Col>
        <Col span={5}><Statistic title="有孩子" value={report?.funnel.with_child || 0} /></Col>
        <Col span={5}><Statistic title="有活跃设备" value={report?.funnel.with_device || 0} /></Col>
        <Col span={5}><Statistic title="有观看记录" value={report?.funnel.watched || 0} /></Col>
        <Col span={5}><Statistic title="有支付订单" value={report?.funnel.ordered || 0} /></Col>
      </Row>

      <Typography.Title level={5}>注册留存</Typography.Title>
      <Table
        rowKey="cohort_date"
        size="small"
        style={{ marginBottom: 20 }}
        dataSource={report?.retention || []}
        pagination={false}
        columns={[
          { title: '注册日期', dataIndex: 'cohort_date' },
          { title: '新增账号', dataIndex: 'new_accounts' },
          { title: '次日留存', dataIndex: 'd1' },
          { title: 'D7 留存', dataIndex: 'd7' },
          {
            title: '次日留存率',
            render: (_, row) =>
              row.new_accounts > 0 ? `${((row.d1 / row.new_accounts) * 100).toFixed(0)}%` : '—',
          },
        ]}
      />

      <Typography.Title level={5}>每日指标</Typography.Title>
      <Space direction="vertical" size={6} style={{ width: '100%' }}>
        {(report?.daily || []).slice(-14).map((point) => (
          <Space key={point.date} style={{ width: '100%' }}>
            <span style={{ width: 96, display: 'inline-block' }}>{point.date}</span>
            <span
              style={{
                display: 'inline-block',
                width: `${Math.max(4, (point.new_accounts / dailyMax) * 280)}px`,
                height: 10,
                borderRadius: 4,
                background: '#1677ff',
              }}
            />
            <span>
              新增 {point.new_accounts} · 活跃账号 {point.active_accounts} · 活跃孩子{' '}
              {point.active_children} · 支付 {point.paid_orders}
            </span>
          </Space>
        ))}
      </Space>
    </Card>
  )
}
