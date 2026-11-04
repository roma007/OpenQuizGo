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
