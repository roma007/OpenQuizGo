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
  image?: string
  images?: string[]
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
  stem_media?: { url?: string } | null
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

export interface TopicCoverageCell {
  topic_id: string
  topic_name: string
  status: string
  count: number
}

export interface TopicCoverageResult {
  cells: TopicCoverageCell[]
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

export interface CalibrationHistoryItem {
  id: number
  question_id: string
  from: number
  to: number
  actor_id: string
  created_at: string
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
  external_trade_no?: string
  refund_before?: MembershipSnapshot
  refund_after?: MembershipSnapshot
}

export interface MembershipSnapshot {
  status: string
  plan_name: string
  expires_at: string
}

export interface MembershipOrderList {
  items: MembershipOrder[]
  total: number
}

export interface DashboardOrder {
  order_no: string
  plan_name: string
  amount_cents: number
  status: string
  created_at: string
}

export interface DashboardExpiring {
  account_id: string
  plan_name: string
  expires_at: string
  days_remaining: number
}

export interface DashboardTrendPoint {
  date: string
  orders: number
  revenue_cents: number
}

export interface DashboardOverview {
  questions: { total: number; online: number; draft: number; review: number }
  topics: { total: number; online: number; active: number }
  membership: {
    active_members: number
    revenue_cents: number
    orders_30d: number
    paid_30d: number
  }
  families: number
  children: number
  battles_7d: number
  recent_orders: DashboardOrder[]
  expiring_memberships: DashboardExpiring[]
  trend: DashboardTrendPoint[]
  generated_at: string
}

export interface FamilySummary {
  family_id: string
  owner_id: string
  member_status: string
  members_count: number
  children_count: number
  devices_count: number
  membership_status?: string
  plan_name?: string
  expires_at?: string
  created_at: string
}

export interface FamilyMember {
  account_id: string
  role: string
  joined_at: string
}

export interface FamilyChild {
  id: string
  nickname: string
  stage: string
  grade_tag?: string
  created_at: string
}

export interface FamilyDevice {
  device_id: string
  platform: string
  status: string
  child_id: string
  child_name: string
  bound_at: string
  token_expires_at?: string
}

export interface FamilyDetail {
  family_id: string
  owner_id: string
  member_status: string
  membership_status?: string
  plan_name?: string
  expires_at?: string
  created_at: string
  members: FamilyMember[]
  children: FamilyChild[]
  devices: FamilyDevice[]
}

export interface FamilyList {
  items: FamilySummary[]
  total: number
}
