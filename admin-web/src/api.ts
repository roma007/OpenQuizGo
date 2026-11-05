import type {
  AdminPrincipal,
  AuditEntry,
  CoverageResult,
  TopicCoverageResult,
  CoverageSnapshot,
  LoginResult,
  Question,
  QuestionList,
  CalibrationResult,
  CalibrationHistoryItem,
  CalibrationTrendPoint,
  CalibrationRollbackResult,
  TopicPack,
  TopicPackList,
  QuizPreviewResult,
  MembershipPlan,
  MembershipPlanList,
  FamilyList,
  FamilyDetail,
  MembershipOrder,
  MembershipOrderList,
  MembershipCoupon,
  MembershipCouponList,
  MembershipInvoice,
  MembershipInvoiceList,
  ReconciliationReport,
  GrowthReport,
  DashboardOverview,
} from './types'

const ADMIN_REQUEST_HEADER = 'X-Admin-Request'

// tokenStore 仅为兼容旧引用保留；045 起令牌只保存在 HttpOnly Cookie 中。
export const tokenStore = {
  get: () => null,
  set: (_token: string) => {},
  clear: () => {},
}

export class ApiError extends Error {
  code?: string
  status: number
  details?: unknown

  constructor(message: string, status: number, code?: string, details?: unknown) {
    super(message)
    this.status = status
    this.code = code
    this.details = details
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers: Record<string, string> = {
    [ADMIN_REQUEST_HEADER]: '1',
    ...(options.headers as Record<string, string> | undefined),
  }
  if (options.body && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }
  const response = await fetch(`/v1${path}`, {
    ...options,
    headers,
    credentials: 'include',
  })
  const text = await response.text()
  let body: any = null
  if (text) {
    try {
      body = JSON.parse(text)
    } catch {
      body = text
    }
  }
  if (!response.ok) {
    throw new ApiError(
      body?.message || `请求失败（${response.status}）`,
      response.status,
      body?.code,
      body,
    )
  }
  return body as T
}

export const api = {
  async login(username: string, password: string) {
    return request<LoginResult>('/admin/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    })
  },
  logout: () => request<{ status: string }>('/admin/auth/logout', { method: 'POST' }),
  me: () => request<AdminPrincipal>('/admin/me'),
  questions: (params: Record<string, string | number | undefined>) => {
    const query = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== '') query.set(key, String(value))
    })
    return request<QuestionList>(`/admin/questions?${query.toString()}`)
  },
  question: (id: string) => request<Question>(`/admin/questions/${id}`),
  createQuestion: (payload: unknown) =>
    request<Question>('/admin/questions', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateQuestion: (id: string, payload: unknown) =>
    request<Question>(`/admin/questions/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  deleteQuestion: (id: string) =>
    request<void>(`/admin/questions/${id}`, { method: 'DELETE' }),
  action: (id: string, action: string) =>
    request<Question>(`/admin/questions/${id}/${action}`, { method: 'POST' }),
  review: (id: string, approve: boolean) =>
    request<Question>(`/admin/questions/${id}/review`, {
      method: 'POST',
      body: JSON.stringify({ approve }),
    }),
  importQuestions: (body: BodyInit, contentType: string) =>
    request<{ imported: number }>('/admin/questions/import', {
      method: 'POST',
      body,
      headers: { 'Content-Type': contentType },
    }),
  uploadMedia: async (file: File) => {
    const form = new FormData()
    form.append('file', file)
    const response = await fetch('/v1/admin/media', {
      method: 'POST',
      headers: { [ADMIN_REQUEST_HEADER]: '1' },
      body: form,
      credentials: 'include',
    })
    const body = await response.json().catch(() => null)
    if (!response.ok) {
      throw new ApiError(body?.message || '上传失败', response.status, body?.code)
    }
    return body as { url: string; bytes: number; content_type: string }
  },
  audit: (params: Record<string, string | undefined> = {}) => {
    const query = new URLSearchParams({ limit: '200' })
    Object.entries(params).forEach(([key, value]) => {
      if (value) query.set(key, value)
    })
    return request<AuditEntry[]>(`/admin/audit?${query.toString()}`)
  },
  coverage: (threshold = 50) =>
    request<CoverageResult>(`/admin/coverage?threshold=${threshold}`),
  coverageTrend: (days = 30) =>
    request<CoverageSnapshot[]>(`/admin/coverage/trend?days=${days}`),
  coverageSnapshot: () =>
    request<CoverageSnapshot>('/admin/coverage/snapshot', { method: 'POST' }),
  coverageTopics: (threshold = 50) =>
    request<TopicCoverageResult>(`/admin/coverage/topics?threshold=${threshold}`),
  coverageTopicsExport: async (threshold = 50) => {
    const response = await fetch(
      `/v1/admin/coverage/export?threshold=${threshold}&dimension=topic`,
      {
        headers: { [ADMIN_REQUEST_HEADER]: '1' },
        credentials: 'include',
      },
    )
    if (!response.ok) throw new ApiError('导出失败', response.status)
    return response.blob()
  },
  auditExport: async (params: Record<string, string | undefined> = {}) => {
    const query = new URLSearchParams({ limit: '5000' })
    Object.entries(params).forEach(([key, value]) => {
      if (value) query.set(key, value)
    })
    const response = await fetch(`/v1/admin/audit/export?${query.toString()}`, {
      headers: { [ADMIN_REQUEST_HEADER]: '1' },
      credentials: 'include',
    })
    if (!response.ok) throw new ApiError('导出失败', response.status)
    return response.blob()
  },
  coverageExport: async (threshold = 50) => {
    const response = await fetch(`/v1/admin/coverage/export?threshold=${threshold}`, {
      headers: { [ADMIN_REQUEST_HEADER]: '1' },
      credentials: 'include',
    })
    if (!response.ok) throw new ApiError('导出失败', response.status)
    return response.blob()
  },
  calibrationPreview: (minAnswers: number) =>
    request<CalibrationResult>(
      `/admin/calibration/preview?min_answers=${minAnswers}&limit=200`,
    ),
  calibrationTrend: (days = 30) =>
    request<CalibrationTrendPoint[]>(`/admin/calibration/trend?days=${days}`),
  calibrationRollback: (questionIds: string[]) =>
    request<CalibrationRollbackResult>('/admin/calibration/rollback', {
      method: 'POST',
      body: JSON.stringify({ question_ids: questionIds }),
    }),
  calibrationHistory: (questionId?: string, limit = 20) => {
    const query = new URLSearchParams({ limit: String(limit) })
    if (questionId) query.set('question_id', questionId)
    return request<CalibrationHistoryItem[]>(
      `/admin/calibration/history?${query.toString()}`,
    )
  },
  quizPreview: (payload: unknown) =>
    request<QuizPreviewResult>('/admin/quiz/preview', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  dashboard: (days = 7) => request<DashboardOverview>(`/admin/dashboard?days=${days}`),
  growth: (days = 30) => request<GrowthReport>(`/admin/analytics/growth?days=${days}`),
  families: (params: Record<string, string | number | undefined>) => {
    const query = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== '') query.set(key, String(value))
    })
    return request<FamilyList>(`/admin/families?${query.toString()}`)
  },
  family: (id: string) => request<FamilyDetail>(`/admin/families/${id}`),
  membershipReconciliation: (from: string, to: string) => {
    const query = new URLSearchParams({ from, to })
    return request<ReconciliationReport>(
      `/admin/membership/reconciliation?${query.toString()}`,
    )
  },
  membershipReconciliationExport: async (from: string, to: string) => {
    const query = new URLSearchParams({ from, to })
    const response = await fetch(
      `/v1/admin/membership/reconciliation/export?${query.toString()}`,
      {
        headers: { [ADMIN_REQUEST_HEADER]: '1' },
        credentials: 'include',
      },
    )
    if (!response.ok) throw new ApiError('导出失败', response.status)
    return response.blob()
  },
  membershipInvoices: (status = '', limit = 50) => {
    const query = new URLSearchParams({ limit: String(limit) })
    if (status) query.set('status', status)
    return request<MembershipInvoiceList>(`/admin/membership/invoices?${query.toString()}`)
  },
  updateMembershipInvoice: (id: string, payload: unknown) =>
    request<MembershipInvoice>(`/admin/membership/invoices/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  membershipCoupons: () => request<MembershipCouponList>('/admin/membership/coupons'),
  createMembershipCoupon: (payload: unknown) =>
    request<MembershipCoupon>('/admin/membership/coupons', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateMembershipCoupon: (id: string, payload: unknown) =>
    request<MembershipCoupon>(`/admin/membership/coupons/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  membershipPlans: () => request<MembershipPlanList>('/admin/membership/plans'),
  createMembershipPlan: (payload: unknown) =>
    request<MembershipPlan>('/admin/membership/plans', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateMembershipPlan: (id: string, payload: unknown) =>
    request<MembershipPlan>(`/admin/membership/plans/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  deleteMembershipPlan: (id: string) =>
    request<void>(`/admin/membership/plans/${id}`, { method: 'DELETE' }),
  membershipOrders: (params: Record<string, string | number | undefined>) => {
    const query = new URLSearchParams()
    Object.entries(params).forEach(([key, value]) => {
      if (value !== undefined && value !== '') query.set(key, String(value))
    })
    return request<MembershipOrderList>(`/admin/membership/orders?${query.toString()}`)
  },
  refundMembershipOrder: (id: string, reason: string) =>
    request<MembershipOrder>(`/admin/membership/orders/${id}/refund`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  topics: () => request<TopicPackList>('/admin/topics'),
  topic: (id: string) => request<TopicPack>(`/admin/topics/${id}`),
  createTopic: (payload: unknown) =>
    request<TopicPack>('/admin/topics', {
      method: 'POST',
      body: JSON.stringify(payload),
    }),
  updateTopic: (id: string, payload: unknown) =>
    request<TopicPack>(`/admin/topics/${id}`, {
      method: 'PUT',
      body: JSON.stringify(payload),
    }),
  deleteTopic: (id: string) =>
    request<void>(`/admin/topics/${id}`, { method: 'DELETE' }),
  setTopicQuestions: (id: string, questionIds: string[]) =>
    request<TopicPack>(`/admin/topics/${id}/questions`, {
      method: 'PUT',
      body: JSON.stringify({ question_ids: questionIds }),
    }),
  calibrationApply: (items: { question_id: string; difficulty: number }[]) =>
    request<{ applied: number }>('/admin/calibration/apply', {
      method: 'POST',
      body: JSON.stringify({ items }),
    }),
}
