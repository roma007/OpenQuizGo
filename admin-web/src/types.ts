export interface AdminPrincipal {
  id: string
  username: string
  display_name?: string
  roles?: string[]
  permissions?: string[]
}

export interface LoginResult {
  token: string
  expires_in: number
  admin: AdminPrincipal
}

export interface OptionItem {
  id: string
  text: string
}

export interface Question {
  id: string
  subject: string
  category: string
  stage: string
  grade_tag?: string
  difficulty: number
  type: string
  stem: string
  stem_media?: unknown
  options?: OptionItem[] | null
  answer: unknown
  answer_alias?: unknown
  explain?: string
  duration_limit?: number | null
  topic_id?: string
  status: string
  version: number
  schema_ver: number
  created_at: string
  updated_at: string
}

export interface QuestionList {
  items: Question[]
  total: number
}

export interface CoverageCell {
  subject: string
  stage: string
  type: string
  status: string
  difficulty: number
  count: number
}

export interface CoverageResult {
  cells: CoverageCell[]
  total: number
  warnings: string[]
}

export interface AuditEntry {
  id: number
  actor_type: string
  actor_id: string
  action: string
  target_type: string
  target_id: string
  after_data?: unknown
  created_at: string
}

export interface CalibrationItem {
  question_id: string
  stem: string
  subject: string
  stage: string
  difficulty: number
  answered: number
  correct: number
  accuracy: number
  suggested_difficulty: number
  reason: string
}

export interface CalibrationResult {
  items: CalibrationItem[]
  thresholds: { min_answers: number; up: number; down: number }
}

export interface TopicPack {
  id: string
  name: string
  type: string
  subject_scope: string[]
  stage_scope: string[]
  start_at?: string | null
  end_at?: string | null
  is_paid: boolean
  status: string
  question_count: number
  question_ids?: string[]
  created_at: string
  updated_at: string
}

export interface TopicPackList {
  items: TopicPack[]
}

export interface QuizSelected {
  question_id: string
  subject: string
  stage: string
  type: string
  difficulty: number
  repeated: boolean
}

export interface QuizPreviewResult {
  total: number
  quota: Record<string, number>
  selected: QuizSelected[]
  max_score: number
  difficulty_target: number
  warnings?: string[]
  seed: number
  topic_id?: string
  topic_name?: string
  generated_at: string
}

export interface MembershipPlan {
  id: string
  code: string
  name: string
  description: string
  duration_days: number
  price_cents: number
  features: string[]
  entitlement_codes: string[]
  status: string
  sort: number
  created_at: string
  updated_at: string
}

export interface MembershipPlanList {
  items: MembershipPlan[]
}

export interface MembershipOrder {
  id: string
  order_no: string
  account_id?: string
  plan_id: string
  plan_name: string
  amount_cents: number
  duration_days: number
  status: string
  channel: string
  created_at: string
  paid_at?: string
  refunded_at?: string
  refund_reason?: string
  refunded_by?: string
}

export interface MembershipOrderList {
  items: MembershipOrder[]
  total: number
}
