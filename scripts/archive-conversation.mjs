#!/usr/bin/env node
/**
 * 对话留档脚本（铁律 1）
 *
 * 用法：
 *   node scripts/archive-conversation.mjs                     # 归档当前项目最近一次会话
 *   node scripts/archive-conversation.mjs --session ses_xxx    # 归档指定会话
 *   node scripts/archive-conversation.mjs --all                # 全量补档
 *   node scripts/archive-conversation.mjs --sanitize           # 导出时脱敏
 *   node scripts/archive-conversation.mjs --all --sanitize
 *
 * 产出：
 *   docs/conversations/YYYY-MM-DD-NNN-<标题>.md   可读记录
 *   docs/conversations/raw/<sessionID>.json       原始数据
 *   docs/conversations/INDEX.md                   自动索引
 */

import { execFileSync } from 'node:child_process'
import {
  closeSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  openSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const ROOT = resolve(import.meta.dirname, '..')
const DOC_DIR = join(ROOT, 'docs', 'conversations')
const RAW_DIR = join(DOC_DIR, 'raw')
const INDEX_FILE = join(DOC_DIR, 'INDEX.md')

const args = process.argv.slice(2)
const opts = {
  session: valueOf('--session'),
  all: args.includes('--all'),
  sanitize: args.includes('--sanitize'),
}

mkdirSync(RAW_DIR, { recursive: true })

/* ------------------------------ 工具函数 ------------------------------ */

function valueOf(flag) {
  const i = args.indexOf(flag)
  return i >= 0 ? args[i + 1] : undefined
}

/**
 * 通过文件承接 stdout 调用 opencode。
 * 直接走管道时，CLI 可能在进程退出前未刷完 stdout，导致 JSON 被截断；
 * 文件写入在 Node 中是同步的，可完整落盘。
 */
function run(args) {
  const dir = mkdtempSync(join(tmpdir(), 'oqg-archive-'))
  const out = join(dir, 'stdout.txt')
  const fd = openSync(out, 'w')
  try {
    execFileSync('opencode', args, {
      cwd: ROOT,
      stdio: ['ignore', fd, 'pipe'],
      maxBuffer: 256 * 1024 * 1024,
    })
  } finally {
    closeSync(fd)
  }
  try {
    return readFileSync(out, 'utf8')
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
}

function pad(n, len = 2) {
  return String(n).padStart(len, '0')
}

function formatDate(ms) {
  const d = new Date(ms)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

function formatTime(ms) {
  const d = new Date(ms)
  return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function formatDateTime(ms) {
  return `${formatDate(ms)} ${formatTime(ms)}`
}

function slugify(text, max = 40) {
  const s = String(text ?? '未命名会话')
    .replace(/[\\/:*?"<>|：／＼＊？“”＜＞｜\r\n\t]/g, ' ')
    .replace(/\s+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, max)
  return s || '未命名会话'
}

function truncate(text, max) {
  const s = String(text ?? '')
  return s.length > max ? `${s.slice(0, max)}\n…（已截断 ${s.length - max} 字符，完整内容见原始 JSON）` : s
}

function fence(text, lang = '') {
  const s = String(text ?? '')
  const longest = Math.max(3, ...Array.from(s.matchAll(/`{3,}/g), (m) => m[0].length + 1))
  return `${'`'.repeat(longest)}${lang}\n${s}\n${'`'.repeat(longest)}`
}

function quoteYaml(value) {
  return JSON.stringify(String(value))
}

/* ------------------------------ 归档索引 ------------------------------ */

/** 读取已归档文件头部的元信息 */
function readArchiveMeta(file) {
  const text = readFileSync(join(DOC_DIR, file), 'utf8')
  const meta = {}
  for (const line of text.slice(0, 1000).split('\n')) {
    const m = line.match(/^([a-z_]+):\s*(.*)$/)
    if (!m) continue
    let value = m[2]
    if (value.startsWith('"')) {
      try {
        value = JSON.parse(value)
      } catch {
        /* 保持原值 */
      }
    }
    meta[m[1]] = value
  }
  return meta
}

function archivedFiles() {
  return readdirSync(DOC_DIR, { withFileTypes: true })
    .filter((e) => e.isFile() && e.name.endsWith('.md') && e.name !== 'INDEX.md')
    .map((e) => e.name)
}

function findBySessionID(sessionID) {
  for (const file of archivedFiles()) {
    if (readArchiveMeta(file).session_id === sessionID) return file
  }
  return null
}

function nextSeq(date) {
  const used = archivedFiles()
    .map((f) => f.match(/^(\d{4}-\d{2}-\d{2})-(\d{3})-/))
    .filter(Boolean)
    .filter((m) => m[1] === date)
    .map((m) => Number(m[2]))
  return (used.length ? Math.max(...used) : 0) + 1
}

function rebuildIndex() {
  const rows = archivedFiles()
    .map((file) => {
      const meta = readArchiveMeta(file)
      if (!meta.session_id) return null
      return {
        file,
        date: meta.date ?? '',
        created: meta.created ?? '',
        title: meta.title ?? file,
        session_id: meta.session_id,
        messages: Number(meta.message_count ?? 0),
      }
    })
    .filter(Boolean)
    .sort((a, b) => String(b.created).localeCompare(String(a.created)))

  const lines = [
    '# 对话归档索引',
    '',
    '> 本文件由 `scripts/archive-conversation.mjs` 自动生成，请勿手工编辑。',
    `> 最后更新：${formatDateTime(Date.now())}`,
    '',
    `共 ${rows.length} 条归档。`,
    '',
    '| 日期 | 时间 | 标题 | 消息数 | 会话 ID |',
    '| --- | --- | --- | --- | --- |',
    ...rows.map(
      (r) =>
        `| ${r.date} | ${r.created.slice(11)} | [${r.title.replace(/\|/g, '\\|')}](${encodeURI(r.file)}) | ${r.messages} | \`${r.session_id}\` |`,
    ),
    '',
  ]
  writeFileSync(INDEX_FILE, lines.join('\n'))
}

/* ------------------------------ 渲染消息 ------------------------------ */

function toolContent(tool) {
  const input = tool.state?.input ?? {}
  const pieces = [fence(truncate(JSON.stringify(input, null, 2), 4000), 'json')]

  for (const item of tool.state?.content ?? []) {
    if (item.type === 'text') {
      pieces.push(fence(truncate(item.text, 6000), 'text'))
    } else if (item.type === 'file') {
      pieces.push(`> 附件：${item.filename ?? item.path ?? '(file)'}`)
    }
  }
  if (tool.state?.status && tool.state.status !== 'completed') {
    pieces.push(`> 状态：${tool.state.status}`)
  }
  return pieces.join('\n\n')
}

function renderAssistant(msg) {
  const parts = []
  for (const item of msg.content ?? []) {
    if (item.type === 'text' && item.text?.trim()) {
      parts.push(item.text.trim())
    } else if (item.type === 'reasoning' && item.text?.trim()) {
      parts.push(
        `<details>\n<summary>思考过程</summary>\n\n${item.text.trim()}\n\n</details>`,
      )
    } else if (item.type === 'tool') {
      parts.push(
        `<details>\n<summary>🔧 工具调用：${item.name}</summary>\n\n${toolContent(item)}\n\n</details>`,
      )
    } else if (item.type === 'file') {
      parts.push(`> 文件：${item.filename ?? item.path ?? '(file)'}`)
    }
  }
  return parts.join('\n\n')
}

function renderSystem(msg) {
  return `<details>\n<summary>系统消息</summary>\n\n${msg.text ?? ''}\n\n</details>`
}

function renderSession(exported) {
  const { info, messages } = exported
  const created = info.time?.created ?? Date.now()
  const updated = info.time?.updated ?? created
  const model = info.model ? `${info.model.providerID}/${info.model.id}` : '-'

  const body = []
  for (const msg of messages) {
    const time = formatTime(msg.time?.created ?? created)
    if (msg.type === 'user') {
      body.push(`## 👤 用户 · ${time}\n\n${msg.text ?? ''}`)
      for (const file of msg.files ?? []) {
        body.push(`> 附件：${file.filename ?? file.path ?? JSON.stringify(file)}`)
      }
    } else if (msg.type === 'assistant') {
      const content = renderAssistant(msg)
      if (content.trim()) body.push(`## 🤖 助手 · ${time}\n\n${content}`)
    } else if (msg.type === 'system') {
      body.push(`## ⚙️ 系统 · ${time}\n\n${renderSystem(msg)}`)
    }
  }

  const header = [
    '---',
    `session_id: ${info.id}`,
    `title: ${quoteYaml(info.title ?? '未命名会话')}`,
    `date: ${formatDate(created)}`,
    `created: ${formatDateTime(created)}`,
    `updated: ${formatDateTime(updated)}`,
    `directory: ${quoteYaml(info.location?.directory ?? info.directory ?? '')}`,
    `model: ${quoteYaml(model)}`,
    `agent: ${quoteYaml(info.agent ?? '-')}`,
    `message_count: ${messages.length}`,
    `tokens: ${quoteYaml(JSON.stringify(info.tokens ?? {}))}`,
    `cost: ${info.cost ?? 0}`,
    '---',
    '',
  ]

  return {
    markdown: [
      ...header,
      `# 对话记录：${info.title ?? '未命名会话'}`,
      '',
      `> 会话 ID：\`${info.id}\` ｜ 创建：${formatDateTime(created)} ｜ 更新：${formatDateTime(updated)}`,
      `> 模型：${model} ｜ 工作目录：\`${info.location?.directory ?? info.directory ?? ''}\``,
      '',
      body.join('\n\n'),
      '',
    ].join('\n'),
    created,
    updated,
    model,
  }
}

/* ------------------------------ 主流程 ------------------------------ */

function listSessions() {
  const raw = run(['session', 'list', '-n', '1000', '--format', 'json'])
  return JSON.parse(raw)
}

function exportSession(sessionID) {
  const flags = ['session', 'export', sessionID]
  if (opts.sanitize) flags.push('--sanitize')
  return JSON.parse(run(flags))
}

function archiveOne(session) {
  const exported = exportSession(session.id)
  const { markdown, created, updated } = renderSession(exported)

  let file = findBySessionID(session.id)
  if (!file) {
    const date = formatDate(created)
    const seq = pad(nextSeq(date), 3)
    file = `${date}-${seq}-${slugify(exported.info.title ?? session.title)}.md`
  }

  writeFileSync(join(DOC_DIR, file), markdown)
  writeFileSync(join(RAW_DIR, `${session.id}.json`), JSON.stringify(exported, null, 2))

  console.log(`✔ ${file}（${exported.messages.length} 条消息）`)
  return { file, created, updated }
}

function main() {
  const all = listSessions()
  const inProject = all.filter(
    (s) => typeof s.directory === 'string' && resolve(s.directory).startsWith(ROOT),
  )

  let targets
  if (opts.session) {
    const found = all.find((s) => s.id === opts.session)
    if (!found) {
      console.error(`✘ 未找到会话：${opts.session}`)
      process.exit(1)
    }
    targets = [found]
  } else if (opts.all) {
    targets = inProject
  } else {
    targets = inProject.slice(0, 1)
  }

  if (!targets.length) {
    console.log('没有需要归档的会话。')
    return
  }

  for (const session of targets) archiveOne(session)
  rebuildIndex()
  console.log(`\n索引已更新：${INDEX_FILE}`)
}

main()
