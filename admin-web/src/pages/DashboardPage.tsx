import {
  Button,
  Card,
  Col,
  Progress,
  Row,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type {
  DashboardExpiring,
  DashboardOrder,
  DashboardOverview,
  DashboardTrendPoint,
} from '../types'

const orderStatusLabel: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  cancelled: '已取消',
  refunded: '已退款',
}
const orderStatusColor: Record<string, string> = {
  pending: 'processing',
  paid: 'success',
  cancelled: 'default',
  refunded: 'orange',
}

function yuan(cents: number) {
  return `¥${(cents / 100).toLocaleString('zh-CN', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  })}`
}

export default function DashboardPage() {
  const [data, setData] = useState<DashboardOverview | null>(null)
  const [loading, setLoading] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      setData(await api.dashboard())
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void load()
  }, [])

  const orderColumns: ColumnsType<DashboardOrder> = [
    {
      title: '订单号',
      dataIndex: 'order_no',
      width: 210,
      render: (value: string) => <Typography.Text copyable>{value}</Typography.Text>,
    },
    { title: '套餐', dataIndex: 'plan_name' },
    {
      title: '金额',
      dataIndex: 'amount_cents',
      width: 110,
      render: (value: number) => yuan(value),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: string) => (
        <Tag color={orderStatusColor[value] || 'default'}>
          {orderStatusLabel[value] || value}
        </Tag>
      ),
    },
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 170,
      render: (value: string) => new Date(value).toLocaleString(),
    },
  ]

  const expiringColumns: ColumnsType<DashboardExpiring> = [
    {
      title: '账号',
      dataIndex: 'account_id',
      width: 120,
      render: (value: string) => <Typography.Text code>{value.slice(0, 8)}…</Typography.Text>,
    },
    { title: '套餐', dataIndex: 'plan_name' },
    {
      title: '到期时间',
      dataIndex: 'expires_at',
      width: 170,
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: '剩余',
      dataIndex: 'days_remaining',
      width: 90,
      render: (value: number) => <Tag color="orange">{value} 天</Tag>,
    },
  ]

  const maxTrendRevenue = Math.max(
    0,
    ...(data?.trend || []).map((item) => item.revenue_cents),
  )

  const trendColumns: ColumnsType<DashboardTrendPoint> = [
    { title: '日期', dataIndex: 'date', width: 90 },
    { title: '订单数', dataIndex: 'orders', width: 90 },
    {
      title: '支付金额',
      dataIndex: 'revenue_cents',
      width: 120,
      render: (value: number) => yuan(value),
    },
    {
      title: '趋势',
      dataIndex: 'revenue_cents',
      key: 'bar',
      render: (value: number) => (
        <Progress
          percent={maxTrendRevenue > 0 ? Math.round((value / maxTrendRevenue) * 100) : 0}
          showInfo={false}
          strokeWidth={8}
        />
      ),
    },
  ]

  return (
    <Space orientation="vertical" size={16} style={{ display: 'flex' }}>
      <Card
        loading={loading && !data}
        title="运营概览"
        extra={
          <Space>
            {data && (
              <Typography.Text type="secondary">
                更新于 {new Date(data.generated_at).toLocaleTimeString()}
              </Typography.Text>
            )}
            <Button onClick={() => void load()} loading={loading}>
              刷新
            </Button>
          </Space>
        }
      >
        {data && (
          <>
            <Row gutter={[16, 16]}>
              <Col xs={12} md={8} xl={4}>
                <Card size="small">
                  <Statistic
                    title="题目总量"
                    value={data.questions.total}
                    suffix={
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        在线 {data.questions.online}
                      </Typography.Text>
                    }
                  />
                </Card>
              </Col>
              <Col xs={12} md={8} xl={4}>
                <Card size="small">
                  <Statistic
                    title="专题在线"
                    value={data.topics.online}
                    suffix={
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        生效 {data.topics.active}
                      </Typography.Text>
                    }
                  />
                </Card>
              </Col>
              <Col xs={12} md={8} xl={4}>
                <Card size="small">
                  <Statistic
                    title="活跃会员"
                    value={data.membership.active_members}
                    suffix={
                      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                        近30天订单 {data.membership.orders_30d}
                      </Typography.Text>
                    }
                  />
                </Card>
              </Col>
              <Col xs={12} md={8} xl={4}>
                <Card size="small">
                  <Statistic title="净收入" value={yuan(data.membership.revenue_cents)} />
                </Card>
              </Col>
              <Col xs={12} md={8} xl={4}>
                <Card size="small">
                  <Statistic
                    title="家庭 / 孩子"
                    value={`${data.families} / ${data.children}`}
                  />
                </Card>
              </Col>
              <Col xs={12} md={8} xl={4}>
                <Card size="small">
                  <Statistic title="近 7 天对战" value={data.battles_7d} />
                </Card>
              </Col>
            </Row>
            <Typography.Paragraph type="secondary" style={{ marginTop: 16, marginBottom: 0 }}>
              题目：草稿 {data.questions.draft} · 待审核 {data.questions.review} · 专题总量{' '}
              {data.topics.total} · 近30天支付 {data.membership.paid_30d}
            </Typography.Paragraph>
            <Card size="small" title="近 7 天趋势" style={{ marginTop: 16 }}>
              <Table
                rowKey="date"
                size="small"
                columns={trendColumns}
                dataSource={data.trend}
                pagination={false}
                locale={{ emptyText: '暂无趋势数据' }}
              />
            </Card>
          </>
        )}
      </Card>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={14}>
          <Card title="最近订单">
            <Table
              rowKey="order_no"
              size="small"
              columns={orderColumns}
              dataSource={data?.recent_orders || []}
              pagination={false}
              locale={{ emptyText: '还没有订单' }}
            />
          </Card>
        </Col>
        <Col xs={24} xl={10}>
          <Card title="7 天内到期会员">
            <Table
              rowKey="account_id"
              size="small"
              columns={expiringColumns}
              dataSource={data?.expiring_memberships || []}
              pagination={false}
              locale={{ emptyText: '暂无即将到期会员' }}
            />
          </Card>
        </Col>
      </Row>
    </Space>
  )
}
