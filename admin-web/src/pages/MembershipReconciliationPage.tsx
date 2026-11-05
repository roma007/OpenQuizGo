import {
  Button,
  Card,
  Col,
  DatePicker,
  Row,
  Space,
  Statistic,
  Table,
  Typography,
  message,
} from 'antd'
import dayjs, { type Dayjs } from 'dayjs'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { ReconciliationReport } from '../types'

const { RangePicker } = DatePicker

export default function MembershipReconciliationPage() {
  const [range, setRange] = useState<[Dayjs, Dayjs]>([
    dayjs().subtract(29, 'day'),
    dayjs(),
  ])
  const [report, setReport] = useState<ReconciliationReport | null>(null)
  const [loading, setLoading] = useState(false)

  const load = async (value = range) => {
    setLoading(true)
    try {
      setReport(
        await api.membershipReconciliation(
          value[0].format('YYYY-MM-DD'),
          value[1].format('YYYY-MM-DD'),
        ),
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

  const totals = report?.totals

  return (
    <Card loading={loading}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          会员对账
        </Typography.Title>
        <Space>
          <RangePicker
            value={range}
            onChange={(value) => {
              if (value && value[0] && value[1]) {
                setRange([value[0], value[1]])
              }
            }}
          />
          <Button onClick={() => void load()}>刷新</Button>
          <Button
            onClick={async () => {
              try {
                const blob = await api.membershipReconciliationExport(
                  range[0].format('YYYY-MM-DD'),
                  range[1].format('YYYY-MM-DD'),
                )
                const url = URL.createObjectURL(blob)
                const link = document.createElement('a')
                link.href = url
                link.download = 'membership-reconciliation.csv'
                link.click()
                URL.revokeObjectURL(url)
              } catch (error) {
                message.error(error instanceof ApiError ? error.message : '导出失败')
              }
            }}
          >
            导出 CSV
          </Button>
        </Space>
      </Space>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={5}><Statistic title="支付笔数" value={totals?.paid_count || 0} /></Col>
        <Col span={5}><Statistic title="支付金额" value={(totals?.paid_cents || 0) / 100} precision={2} prefix="¥" /></Col>
        <Col span={5}><Statistic title="退款金额" value={(totals?.refunded_cents || 0) / 100} precision={2} prefix="¥" /></Col>
        <Col span={5}><Statistic title="优惠金额" value={(totals?.discount_cents || 0) / 100} precision={2} prefix="¥" /></Col>
        <Col span={4}><Statistic title="净收入" value={(totals?.net_cents || 0) / 100} precision={2} prefix="¥" /></Col>
      </Row>
      <Table
        rowKey="date"
        size="small"
        dataSource={report?.days || []}
        pagination={{ pageSize: 31, showSizeChanger: false }}
        columns={[
          { title: '日期', dataIndex: 'date' },
          { title: '支付笔数', dataIndex: 'paid_count' },
          { title: '支付金额', dataIndex: 'paid_cents', render: (v: number) => `¥${(v / 100).toFixed(2)}` },
          { title: '退款笔数', dataIndex: 'refunded_count' },
          { title: '退款金额', dataIndex: 'refunded_cents', render: (v: number) => `¥${(v / 100).toFixed(2)}` },
          { title: '优惠金额', dataIndex: 'discount_cents', render: (v: number) => `¥${(v / 100).toFixed(2)}` },
          { title: '净收入', dataIndex: 'net_cents', render: (v: number) => `¥${(v / 100).toFixed(2)}` },
        ]}
      />
    </Card>
  )
}
