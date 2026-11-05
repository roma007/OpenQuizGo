import {
  Button,
  Card,
  Descriptions,
  Input,
  Modal,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { FamilyDetail, FamilySummary } from '../types'

const stageLabel: Record<string, string> = {
  preschool: '学龄前',
  low: '小学低',
  mid: '小学中',
  high: '小学高',
}
const membershipLabel: Record<string, string> = {
  active: '会员有效',
  expired: '已到期',
  none: '未开通',
}

function shortId(value: string) {
  return value.length > 12 ? `${value.slice(0, 8)}…` : value
}

export default function FamiliesPage() {
  const [items, setItems] = useState<FamilySummary[]>([])
  const [total, setTotal] = useState(0)
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(false)
  const [detail, setDetail] = useState<FamilyDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const pageSize = 20

  const load = async (nextPage = page, nextQuery = query) => {
    setLoading(true)
    try {
      const result = await api.families({
        query: nextQuery,
        limit: pageSize,
        offset: (nextPage - 1) * pageSize,
      })
      setItems(result.items || [])
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

  const openDetail = async (family: FamilySummary) => {
    setDetailLoading(true)
    try {
      setDetail(await api.family(family.family_id))
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '加载详情失败')
    } finally {
      setDetailLoading(false)
    }
  }

  const columns: ColumnsType<FamilySummary> = [
    {
      title: '家庭 ID',
      dataIndex: 'family_id',
      width: 150,
      render: (value: string) => (
        <Typography.Text code copyable>
          {shortId(value)}
        </Typography.Text>
      ),
    },
    {
      title: '成员 / 孩子 / 设备',
      width: 180,
      render: (_, row) => `${row.members_count} / ${row.children_count} / ${row.devices_count}`,
    },
    {
      title: '会员',
      width: 220,
      render: (_, row) =>
        row.membership_status ? (
          <Space orientation="vertical" size={0}>
            <Tag color={row.membership_status === 'active' ? 'success' : 'default'}>
              {membershipLabel[row.membership_status] || row.membership_status}
              {row.plan_name ? ` · ${row.plan_name}` : ''}
            </Tag>
            {row.expires_at && (
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                至 {new Date(row.expires_at).toLocaleDateString()}
              </Typography.Text>
            )}
          </Space>
        ) : (
          '未开通'
        ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      width: 170,
      render: (value: string) => new Date(value).toLocaleString(),
    },
    {
      title: '操作',
      width: 90,
      render: (_, row) => (
        <Button type="link" onClick={() => void openDetail(row)}>
          详情
        </Button>
      ),
    },
  ]

  return (
    <Card>
      <Space style={{ marginBottom: 16 }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          家庭与设备
        </Typography.Title>
        <Input.Search
          placeholder="家庭 ID / 孩子昵称 / 设备 ID"
          allowClear
          style={{ width: 360 }}
          onSearch={(value) => {
            setQuery(value)
            setPage(1)
            void load(1, value)
          }}
        />
        <Typography.Text type="secondary">不展示手机号与设备令牌</Typography.Text>
      </Space>
      <Table
        rowKey="family_id"
        loading={loading}
        columns={columns}
        dataSource={items}
        scroll={{ x: 900 }}
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
        locale={{ emptyText: '没有匹配的家庭' }}
      />

      <Modal
        open={Boolean(detail)}
        title="家庭详情"
        onCancel={() => setDetail(null)}
        footer={<Button onClick={() => setDetail(null)}>关闭</Button>}
        width={900}
        loading={detailLoading}
        destroyOnHidden
      >
        {detail && (
          <Space orientation="vertical" size={16} style={{ width: '100%' }}>
            <Descriptions column={{ xs: 1, md: 2 }} size="small" bordered>
              <Descriptions.Item label="家庭 ID">
                <Typography.Text code copyable>
                  {detail.family_id}
                </Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label="Owner">
                <Typography.Text code copyable>
                  {detail.owner_id}
                </Typography.Text>
              </Descriptions.Item>
              <Descriptions.Item label="会员">
                {detail.membership_status
                  ? `${membershipLabel[detail.membership_status] || detail.membership_status}${
                      detail.plan_name ? ` · ${detail.plan_name}` : ''
                    }`
                  : '未开通'}
              </Descriptions.Item>
              <Descriptions.Item label="到期">
                {detail.expires_at
                  ? new Date(detail.expires_at).toLocaleString()
                  : '—'}
              </Descriptions.Item>
            </Descriptions>

            <Card size="small" title={`孩子（${detail.children.length}）`}>
              <Table
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={detail.children}
                columns={[
                  { title: '昵称', dataIndex: 'nickname' },
                  {
                    title: '学段',
                    dataIndex: 'stage',
                    render: (value: string) => stageLabel[value] || value,
                  },
                  { title: '年级', dataIndex: 'grade_tag' },
                ]}
              />
            </Card>

            <Card size="small" title={`设备（${detail.devices.length}）`}>
              <Table
                rowKey="device_id"
                size="small"
                pagination={false}
                dataSource={detail.devices}
                columns={[
                  { title: '设备 ID', dataIndex: 'device_id' },
                  { title: '孩子', dataIndex: 'child_name' },
                  { title: '平台', dataIndex: 'platform' },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    render: (value: string) => (
                      <Tag color={value === 'active' ? 'success' : 'default'}>
                        {value === 'active' ? '已绑定' : '已解绑'}
                      </Tag>
                    ),
                  },
                  {
                    title: '绑定时间',
                    dataIndex: 'bound_at',
                    render: (value: string) => new Date(value).toLocaleString(),
                  },
                ]}
              />
            </Card>

            <Card size="small" title={`成员（${detail.members.length}）`}>
              <Table
                rowKey="account_id"
                size="small"
                pagination={false}
                dataSource={detail.members}
                columns={[
                  { title: '账号 ID', dataIndex: 'account_id' },
                  { title: '角色', dataIndex: 'role' },
                  {
                    title: '加入时间',
                    dataIndex: 'joined_at',
                    render: (value: string) => new Date(value).toLocaleString(),
                  },
                ]}
              />
            </Card>
          </Space>
        )}
      </Modal>
    </Card>
  )
}
