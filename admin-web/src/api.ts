import type {
  AdminPrincipal,
  AuditEntry,
  CoverageResult,
  LoginResult,
  Question,
  QuestionList,
  CalibrationResult,
  TopicPack,
  TopicPackList,
  QuizPreviewResult,
} from './types'

const TOKEN_KEY = 'openquizgo.admin.token'

export const tokenStore = {
  get: () => localStorage.getItem(TOKEN_KEY),
  set: (token: string) => localStorage.setItem(TOKEN_KEY, token),
  clear: () => localStorage.removeItem(TOKEN_KEY),
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
  withAuth = true,
): Promise<T> {
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string> | undefined),
  }
  if (options.body && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }
  if (withAuth) {
    const token = tokenStore.get()
    if (token) headers.Authorization = `Bearer ${token}`
  }
  const response = await fetch(`/v1${path}`, { ...options, headers })
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
    const result = await request<LoginResult>(
      '/admin/auth/login',
      { method: 'POST', body: JSON.stringify({ username, password }) },
      false,
    )
    tokenStore.set(result.token)
    return result
  },
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
  audit: (targetId?: string) =>
    request<AuditEntry[]>(
      `/admin/audit?limit=100${targetId ? `&target_id=${encodeURIComponent(targetId)}` : ''}`,
    ),
  coverage: () => request<CoverageResult>('/admin/coverage'),
  coverageExport: async () => {
    const token = tokenStore.get()
    const response = await fetch('/v1/admin/coverage/export', {
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    })
    if (!response.ok) throw new ApiError('导出失败', response.status)
    return response.blob()
  },
  calibrationPreview: (minAnswers: number) =>
    request<CalibrationResult>(
      `/admin/calibration/preview?min_answers=${minAnswers}&limit=200`,
    ),
  quizPreview: (payload: unknown) =>
    request<QuizPreviewResult>('/admin/quiz/preview', {
      method: 'POST',
      body: JSON.stringify(payload),
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
