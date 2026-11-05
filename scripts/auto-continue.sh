#!/usr/bin/env bash
# 无人值守自动推进：每 N 秒向指定 OpenCode 会话发送一次「继续」提示。
#
# 用法：
#   scripts/auto-continue.sh start     # 后台启动
#   scripts/auto-continue.sh stop      # 停止
#   scripts/auto-continue.sh status    # 查看状态
#   scripts/auto-continue.sh run       # 前台运行（调试）
#
# 环境变量：
#   AUTO_CONTINUE_SESSION     会话 ID（默认当前项目最近会话）
#   AUTO_CONTINUE_INTERVAL    间隔秒数（默认 600 = 10 分钟）
#   AUTO_CONTINUE_MAX_RUNS    最多执行次数（默认 12；0 表示不限）
#   AUTO_CONTINUE_PROMPT      发送的提示词
set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE_DIR="$ROOT/.opencode"
STOP_FILE="$STATE_DIR/auto-continue.stop"
PID_FILE="$STATE_DIR/auto-continue.pid"
LOG_FILE="$STATE_DIR/auto-continue.log"

INTERVAL="${AUTO_CONTINUE_INTERVAL:-600}"
MAX_RUNS="${AUTO_CONTINUE_MAX_RUNS:-12}"
SESSION="${AUTO_CONTINUE_SESSION:-ses_e64488e2cffeDYNmw42RxNLybD}"
PROMPT="${AUTO_CONTINUE_PROMPT:-无人值守继续：读取 docs/plans 与 docs/README，选择下一个不依赖外部凭据/硬件的待办，按 AGENTS.md 先设计再实现，运行测试，归档对话，提交并推送两个远端。不要重复已完成的工作；若可做项已完成，输出剩余阻塞清单并停止。不要启动新的自动循环。}"

mkdir -p "$STATE_DIR"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$*" | tee -a "$LOG_FILE"
}

is_running() {
  if [ -f "$PID_FILE" ]; then
    local pid
    pid="$(cat "$PID_FILE" 2>/dev/null || true)"
    if [ -n "${pid:-}" ] && kill -0 "$pid" 2>/dev/null; then
      return 0
    fi
  fi
  return 1
}

stop_loop() {
  if is_running; then
    local pid
    pid="$(cat "$PID_FILE")"
    kill "$pid" 2>/dev/null || true
    sleep 1
    kill -9 "$pid" 2>/dev/null || true
    log "已停止（pid=${pid}）"
  else
    log "未在运行"
  fi
  rm -f "$PID_FILE"
}

run_followup() {
  local run_no="$1"
  log "第 ${run_no} 次自动推进开始（session=${SESSION}）"
  local timeout_cmd=()
  if command -v gtimeout >/dev/null 2>&1; then
    timeout_cmd=(gtimeout 1800)
  elif command -v timeout >/dev/null 2>&1; then
    timeout_cmd=(timeout 1800)
  fi
  local status=0
  if [ "${#timeout_cmd[@]}" -gt 0 ]; then
    (cd "$ROOT" && "${timeout_cmd[@]}" opencode run --auto --session "$SESSION" "$PROMPT") >>"$LOG_FILE" 2>&1 || status=$?
  else
    (cd "$ROOT" && opencode run --auto --session "$SESSION" "$PROMPT") >>"$LOG_FILE" 2>&1 || status=$?
  fi
  if [ "$status" -ne 0 ]; then
    log "第 $run_no 次自动推进失败或超时（exit=${status}），继续下一轮"
  else
    log "第 $run_no 次自动推进完成"
  fi
}

loop() {
  trap 'log "收到退出信号"; rm -f "$PID_FILE"; exit 0' INT TERM
  echo $$ >"$PID_FILE"
  rm -f "$STOP_FILE"
  log "自动推进启动：间隔 ${INTERVAL}s，最多 ${MAX_RUNS} 次（0=不限），session=$SESSION"
  local run_no=0
  while :; do
    if [ -f "$STOP_FILE" ]; then
      log "检测到停止标记 $STOP_FILE，退出"
      break
    fi
    if [ "$MAX_RUNS" != "0" ] && [ "$run_no" -ge "$MAX_RUNS" ]; then
      log "已达到最大执行次数 ${MAX_RUNS}，退出"
      break
    fi
    sleep "$INTERVAL"
    if [ -f "$STOP_FILE" ]; then
      log "检测到停止标记 $STOP_FILE，退出"
      break
    fi
    run_no=$((run_no + 1))
    run_followup "$run_no"
  done
  rm -f "$PID_FILE"
}

start_loop() {
  if is_running; then
    log "已在运行（pid=$(cat "$PID_FILE")）"
    return 0
  fi
  nohup "$0" run >>"$LOG_FILE" 2>&1 &
  local pid=$!
  echo "$pid" >"$PID_FILE"
  log "已后台启动（pid=${pid}，间隔 ${INTERVAL}s，最多 ${MAX_RUNS} 次）"
}

case "${1:-status}" in
  start) start_loop ;;
  stop) stop_loop ;;
  status)
    if is_running; then
      log "运行中（pid=$(cat "$PID_FILE")）"
    else
      log "未运行"
    fi
    ;;
  run) loop ;;
  *) echo "用法: $0 {start|stop|status|run}" >&2; exit 2 ;;
esac
