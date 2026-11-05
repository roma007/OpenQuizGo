# 无人值守自动推进（10 分钟定时器）

`scripts/auto-continue.sh` 每隔 10 分钟向指定 OpenCode 会话发送一次「无人值守继续」提示，使开发在无人盯守时继续推进。

## 快速使用

```bash
# 后台启动（默认间隔 600 秒、最多 12 次）
scripts/auto-continue.sh start

# 查看是否运行
scripts/auto-continue.sh status

# 停止
scripts/auto-continue.sh stop

# 前台运行（调试，会阻塞终端）
scripts/auto-continue.sh run
```

## 配置

| 环境变量 | 默认 | 说明 |
| --- | --- | --- |
| `AUTO_CONTINUE_INTERVAL` | `600` | 间隔秒数 |
| `AUTO_CONTINUE_MAX_RUNS` | `12` | 最多执行次数；`0` 表示不限 |
| `AUTO_CONTINUE_SESSION` | 当前项目最近会话 | 要续跑的会话 ID |
| `AUTO_CONTINUE_PROMPT` | 内置提示词 | 每轮发送的内容 |

示例：

```bash
AUTO_CONTINUE_INTERVAL=600 AUTO_CONTINUE_MAX_RUNS=24 \
  scripts/auto-continue.sh start
```

查看会话 ID：

```bash
opencode session list
```

## 停止方式

- `scripts/auto-continue.sh stop`
- 或创建停止标记：`touch .opencode/auto-continue.stop`（下一轮检查时退出）

## 日志

```bash
tail -f .opencode/auto-continue.log
```

日志与 PID 存放在 `.opencode/`，已加入 `.gitignore`。

## 注意事项

1. 本机需保持唤醒、OpenCode 后台服务可用；
2. 每轮会消耗模型额度；
3. 默认最多 12 轮（约 2 小时），避免无限运行；设为 `0` 前请确认可随时停止；
4. 提示词要求每轮完成设计、测试、归档、提交并推送两个远端；如遇外部阻塞会输出剩余清单；
5. 不要在已有循环运行时重复启动（脚本会检测 PID）；
6. 自动推进仍遵守 `AGENTS.md`，不会绕过设计/计划与验收要求。
