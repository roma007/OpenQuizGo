import {
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import { useAuth } from '../auth'
import type { MembershipOrder } from '../types'

const statusLabel: Record<string, string> = {
  pending: '待支付',
  paid: '已支付',
  cancelled: '已取消',
  refunded: '已退款',
}
const statusColor: Record<string, string> = {
  pending: 'processing',
  paid: 'success',
  cancelled: 'default',
  refunded: 'orange',
}

function yuan(cents: number) {
  return `¥${(cents / 100).toFixed(2)}`
}

export default function MembershipOrdersPage() {
  const { can } = useAuth()
  const [orders, setOrders] = useState<MembershipOrder[]>([])
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState<string | undefined>()
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [refunding, setRefunding] = useState<string | null>(null)
  const [refundTarget, setRefundTarget] = useState<MembershipOrder | null>(null)
  const [reason, setReason] = useState('')
  const [detail, setDetail] = useState<MembershipOrder | null>(null)
  const pageSize = 20

  const load = async (nextPage = page, nextStatus = status) => {
    setLoading(true)
    try {
      const result = await api.membershipOrders({
        status: nextStatus,
        limit: pageSize,
        offset: (nextPage - 1) * pageSize,
      })
      setOrders(result.items || [])
      setTotal(result.total || 0)
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

  const refund = async () => {
    const order = refundTarget
    if (!order) return
    if (!reason.trim()) {
      message.warning('请填写退款原因')
      return
    }
    setRefunding(order.id)
    try {
      await api.refundMembershipOrder(order.id, reason.trim())
      message.success(`订单 ${order.order_no} 已退款`)
      setRefundTarget(null)
      setReason('')
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '退款失败')
    } finally {
      setRefunding(null)
    }
  }

  const columns: ColumnsType<MembershipOrder> = [
    {
      title: '订单号',
      dataIndex: 'order_no',
      width: 220,
      render: (value: string) => <Typography.Text copyable>{value}</Typography.Text>,
    },
    { title: '套餐', dataIndex: 'plan_name', width: 140 },
    {
      title: '金额',
      dataIndex: 'amount_cents',
      width: 100,
      render: (value: number) => yuan(value),
    },
    {
      title: '渠道',
      dataIndex: 'channel',
      width: 100,
      render: (value: string) =>
        value === 'callback' ? '渠道回调' : value === 'mock' ? '模拟支付' : value || '—',
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value: string) => (
        <Tag color={statusColor[value] || 'default'}>{statusLabel[value] || value}</Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 170,
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: '支付时间',
      dataIndex: 'paid_at',
      width: 170,
      render: (value?: string) => (value ? new Date(value).toLocaleString() : '—'),
    },
    {
      title: '退款原因',
      dataIndex: 'refund_reason',
      width: 180,
      ellipsis: true,
      render: (value?: string) => value || '—',
    },
    {
      title: '操作',
      width: 140,
      render: (_, order) => (
        <Space size={0}>
          <Button type="link" onClick={() => setDetail(order)}>
            详情
          </Button>
          {order.status === 'paid' ? (
            <Button
              type="link"
              danger
              loading={refunding === order.id}
              disabled={!can('membership.write')}
              onClick={() => {
                setRefundTarget(order)
                setReason('')
              }}
            >
              退款
            </Button>
          ) : order.status === 'refunded' ? (
            <Typography.Text type="secondary">
              {order.refunded_at ? new Date(order.refunded_at).toLocaleDateString() : '已退款'}
            </Typography.Text>
          ) : null}
        </Space>
      ),
    },
  ]

  return (
    <Card>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          会员订单
        </Typography.Title>
        <Select
          allowClear
          placeholder="全部状态"
          style={{ width: 140 }}
          options={Object.entries(statusLabel).map(([value, label]) => ({ value, label }))}
          onChange={(value) => {
            setStatus(value)
            setPage(1)
            void load(1, value)
          }}
        />
        <Typography.Text type="secondary">退款为模拟操作，回退对应时长</Typography.Text>
      </Space>
      <Table
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={orders}
        scroll={{ x: 1100 }}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: false,
          onChange: (next) => {
            setPage(next)
            void load(next)
          },
        }}
        locale={{ emptyText: '还没有会员订单' }}
      />

      <Modal
        open={Boolean(refundTarget)}
        title={refundTarget ? `退款订单 ${refundTarget.order_no}` : '退款'}
        onCancel={() => setRefundTarget(null)}
        onOk={() => void refund()}
        confirmLoading={refunding !== null}
        okText="确认退款"
        okButtonProps={{ danger: true }}
        destroyOnHidden
      >
        {refundTarget && (
          <Space orientation="vertical" style={{ width: '100%' }}>
            <Typography.Text>
              {refundTarget.plan_name} · ¥{(refundTarget.amount_cents / 100).toFixed(2)} ·{' '}
              {refundTarget.duration_days} 天
            </Typography.Text>
            <Typography.Text type="secondary">
              退款会回退会员到期时间，可能导致权益失效。
            </Typography.Text>
            <Input.TextArea
              rows={3}
              maxLength={200}
              showCount
              placeholder="请填写退款原因（1–200 字）"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
            />
          </Space>
        )}
      </Modal>

      <Modal
        open={Boolean(detail)}
        title={detail ? `订单详情 ${detail.order_no}` : '订单详情'}
        onCancel={() => setDetail(null)}
        footer={<Button onClick={() => setDetail(null)}>关闭</Button>}
        width={760}
        destroyOnHidden
      >
        {detail && (
          <Space orientation="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions column={{ xs: 1, md: 2 }} size="small" bordered>
              <Descriptions.Item label="订单号">
                <Typography.Text copyable>{detail.order_no}</Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={statusColor[detail.status]}>{statusLabel[detail.status] || detail.status}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="套餐">{detail.plan_name}</Descriptions.Item>
              <Descriptions.Item label="金额">{yuan(detail.amount_cents)}</Descriptions.Item>
              <Descriptions.Item label="时长">{detail.duration_days} 天</Descriptions.Item>
              <Descriptions.Item label="渠道">
                {detail.channel === 'callback' ? '渠道回调' : detail.channel === 'mock' ? '模拟支付' : detail.channel}
              </Descriptions.Item>
              <Descriptions.Item label="外部交易号">{detail.external_trade_no || '—'}</Descriptions.Item>
              <Descriptions.Item label="账号">{detail.account_id || '—'}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{new Date(detail.created_at).toLocaleString()}</Descriptions.Item>
              <Descriptions.Item label="支付时间">
                {detail.paid_at ? new Date(detail.paid_at).toLocaleString() : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="退款时间">
                {detail.refunded_at ? new Date(detail.refunded_at).toLocaleString() : '—'}
              </Descriptions.Item>
              <Descriptions.Item label="退款操作人">{detail.refunded_by || '—'}</Descriptions.Item>
              <Descriptions.Item label="退款原因" span={2}>
                {detail.refund_reason || '—'}
              </Descriptions.Item>
            </Descriptions>
            <Card size="small" title="退款前后会员快照">
              <Descriptions column={{ xs: 1, md: 2 }} size="small" bordered>
                <Descriptions.Item label="退款前">
                  {detail.refund_before
                    ? `${detail.refund_before.status} · ${detail.refund_before.plan_name} · ${new Date(detail.refund_before.expires_at).toLocaleString()}`
                    : '—'}
                </Descriptions.Item>
                <Descriptions.Item label="退款后">
                  {detail.refund_after
                    ? `${detail.refund_after.status} · ${detail.refund_after.plan_name} · ${new Date(detail.refund_after.expires_at).toLocaleString()}`
                    : '—'}
                </Descriptions.Item>
              </Descriptions>
            </Card>
          </Space>
        )}
      </Modal>
    </Card>
  )
}
