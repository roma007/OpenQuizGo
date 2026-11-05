import { Button, Card, Input, Modal, Select, Space, Table, Tag, Typography, message } from 'antd'
import { useEffect, useState } from 'react'
import { api, ApiError } from '../api'
import type { MembershipInvoice } from '../types'

const statusLabel: Record<string, string> = {
  requested: '待处理',
  issued: '已开具',
  rejected: '已驳回',
}

export default function MembershipInvoicesPage() {
  const [items, setItems] = useState<MembershipInvoice[]>([])
  const [status, setStatus] = useState('')
  const [loading, setLoading] = useState(false)
  const [editing, setEditing] = useState<MembershipInvoice | null>(null)
  const [nextStatus, setNextStatus] = useState('issued')
  const [remark, setRemark] = useState('')
  const [saving, setSaving] = useState(false)

  const load = async (value = status) => {
    setLoading(true)
    try {
      setItems((await api.membershipInvoices(value)).items || [])
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

  const openUpdate = (invoice: MembershipInvoice) => {
    setEditing(invoice)
    setNextStatus('issued')
    setRemark('')
  }

  const submit = async () => {
    if (!editing) return
    setSaving(true)
    try {
      await api.updateMembershipInvoice(editing.id, {
        status: nextStatus,
        remark: remark.trim(),
      })
      message.success('发票状态已更新')
      setEditing(null)
      await load()
    } catch (error) {
      message.error(error instanceof ApiError ? error.message : '更新失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card loading={loading}>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          发票申请
        </Typography.Title>
        <Space>
          <Select
            style={{ width: 140 }}
            value={status}
            onChange={(value) => {
              setStatus(value)
              void load(value)
            }}
            options={[
              { value: '', label: '全部状态' },
              { value: 'requested', label: '待处理' },
              { value: 'issued', label: '已开具' },
              { value: 'rejected', label: '已驳回' },
            ]}
          />
          <Button onClick={() => void load()}>刷新</Button>
        </Space>
      </Space>
      <Table
        rowKey="id"
        size="small"
        dataSource={items}
        pagination={false}
        columns={[
          { title: '订单', dataIndex: 'order_id', ellipsis: true },
          {
            title: '抬头',
            render: (_, row) =>
              `${row.title_type === 'company' ? '企业' : '个人'} · ${row.title}`,
          },
          { title: '税号', dataIndex: 'tax_no', render: (v) => v || '—' },
          { title: '邮箱', dataIndex: 'email', render: (v) => v || '—' },
          {
            title: '金额',
            render: (_, row) => `¥${(row.amount_cents / 100).toFixed(2)}`,
          },
          {
            title: '状态',
            dataIndex: 'status',
            render: (value) => (
              <Tag color={value === 'issued' ? 'green' : value === 'rejected' ? 'red' : 'blue'}>
                {statusLabel[value] || value}
              </Tag>
            ),
          },
          { title: '备注', dataIndex: 'remark', render: (v) => v || '—' },
          {
            title: '操作',
            width: 100,
            render: (_, row) => (
              <Button size="small" disabled={row.status !== 'requested'} onClick={() => openUpdate(row)}>
                处理
              </Button>
            ),
          },
        ]}
      />
      <Modal
        title="处理发票申请"
        open={editing != null}
        onCancel={() => setEditing(null)}
        onOk={submit}
        confirmLoading={saving}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Select
            style={{ width: 180 }}
            value={nextStatus}
            onChange={setNextStatus}
            options={[
              { value: 'issued', label: '标记已开具' },
              { value: 'rejected', label: '驳回' },
            ]}
          />
          <Input.TextArea
            rows={3}
            value={remark}
            onChange={(event) => setRemark(event.target.value)}
            placeholder="备注（可选，最多 200 字）"
          />
        </Space>
      </Modal>
    </Card>
  )
}
