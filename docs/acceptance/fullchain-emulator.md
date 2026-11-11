# 模拟器全链路验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.65.2（TV 0.4.3 debug） |
| 日期 | 2026-11-11 |
| 范围 | 绑定 → 规则 → 待答题 → 答题 → 授权计量 → 时段锁定 → 家长解锁 → 亲子对战 → 诊断上传 |
| 环境 | 本机 AVD `tvhome-tv`，Android 14（SDK 34），1920×1080 |
| 服务端 | 本机 `http://10.0.2.2:18080/`（模拟器宿主机回环） |
| 孩子 / 设备 | `T2孩子` / 设备 ID `4bffb821-…` |

## 1. 结论

**通过。** 主链路均在模拟器上完成端到端验证，TV、服务端数据库与家长端接口三侧数据一致；权限撤销→篡改上报对「使用情况访问」与「无障碍」两条路径均已验证。

## 2. 分步结果

### 2.1 设备激活与能力上报

```text
POST /v1/family/devices/activate {"code":"280269","child_id":"…"} → 200
{"device_id":"4bffb821-5d93-4c60-858d-05957637e56f","status":"confirmed"}
```

- TV 自动进入「设备已绑定 / 孩子：T2孩子」；
- 无障碍、使用情况访问均显示 ✓；
- 日志 `capability reported`，服务端 `device_capability` 已更新。

### 2.2 规则下发

```text
PUT /v1/family/children/{id}/rule
unlock_minutes=5, pass_threshold=1, time_window=07:00–21:00
→ 200，version=2
```

### 2.3 待答题 → 答题 → 授权

- 调试广播强制 `QUIZ_REQUIRED`，电视显示「需要答题」；
- 打开答题页，5 道题全部答 `a`（与题库答案一致）：

```text
13:50:54 quiz    : session=31af3cbc-… questions=5 pending=5
12:51:52 quiz    : answer pos=0 correct=true score=1.5/1.0
12:52:25 quiz    : answer pos=1 correct=true score=3.0/1.0
12:52:29 quiz    : answer pos=2 correct=true score=4.5/1.0
12:52:33 quiz    : answer pos=3 correct=true score=6.0/1.0
12:52:37 quiz    : answer pos=4 correct=true score=7.5/1.0
12:52:38 quiz    : finished state=passed score=7.5/1.0 grant=5
12:52:38 grant   : accepted id=1dd692b6-… minutes=5 remaining=300
12:52:38 OQG_GUARD: state QUIZZING -> GRANTED
```

- 结算页显示「挑战成功 / 答对 5/5 / 得分 7.5/1 / 本次获得 5 分钟」；
- 数据库：`quiz_session` passed、score 7.50、correct_count 5；`watch_grant` source=`quiz`、5 分钟、active；`answer_log` 5 条。

### 2.4 观看计时与心跳

- 通过 `AudioTrack` 模拟持续媒体播放，`mediaActive=true` 后进入 `WATCHING`；
- 心跳正常递减：

```text
12:56:52 heartbeat: status=active remaining=270 paused=false revoked=false
12:57:22 heartbeat: status=active remaining=240 paused=false revoked=false
12:57:39 OQG_GUARD: watching without usable grant
12:57:39 OQG_GUARD: state WATCHING -> QUIZ_REQUIRED
```

- `watch_grant.consumed_seconds=60`；
- `watch_log` 两条各 30 秒，`watched_seconds` 合计 60。

### 2.5 待答题状态下家长解锁（074 媒体分支）

- 保持媒体播放、状态 `QUIZ_REQUIRED`，家长端签发 5 分钟授权；
- TV 5.1 秒采纳：

```text
12:59:52 grant   : refreshed id=298b5776-… remaining=300 status=active quiet=true
12:59:52 OQG_GUARD: state QUIZ_REQUIRED -> WATCHING
```

- 未经过 `GRANTED`，直接恢复观看，无「按确认键继续观看」误提示。

### 2.6 时段锁定与家长解锁

- 将 `time_window` 改为 `00:00–00:01` 后，下一次心跳服务端撤销授权：

```text
13:00:22 heartbeat: status=revoked remaining=300 paused=false revoked=true
13:00:22 OQG_GUARD: state WATCHING -> BLOCKED
13:00:22 OQG_GUARD: blocker shown reason=time_window
```

- 阻挡页显示「休息一下吧 / 当前不在允许观看的时段」；
- 恢复时段后，在阻挡页进入「家长解锁」，家长端签发授权：

```text
13:01:41 grant   : adopt id=95eb717d-… minutes=5
13:01:41 OQG_GUARD: state BLOCKED -> GRANTED
13:01:41 OQG_GUARD: state GRANTED -> WATCHING
13:01:41 OQG_GUARD: parent unlock accepted, state=WATCHING
```

### 2.7 亲子对战（房间码 + 实时更新）

- 家长端创建房间：`mode=room`，房间码 `APZLC8`，5 题；
- TV 输入房间码加入：

```text
13:07:29 POST /v1/device/battles/join → 200
13:07:29 battle: joined id=93aec5d4-… status=child_turn
```

- 孩子 5 题作答，得分 5，进入「等待家长作答」；
- 家长端提交 5 题答案后，TV 自动刷新为「平局 / 孩子 5:5 家长」：

```text
13:11:08 battle: finished child=5 parent=5
```

- 数据库：`family_battle` status=finished、child_score=5、parent_score=5；`family_battle_answer` 双方各 5 条；
- 家长端历史接口返回该对局；排行榜返回 `battles=1, draws=1, points=1, medals=["champion"]`。

### 2.8 诊断导出与上传

- 诊断页「导出日志」生成 `20261111-1321` 分享文件；
- 「上传诊断」：

```text
13:21:31 POST /v1/device/diagnostics → 200
13:21:31 OQG_GUARD: diagnostics uploaded bytes=9381
```

- 服务端 `device_diagnostic` 新增记录：`note="manual upload from TV"`。

### 2.9 权限撤销与篡改上报

- 在应用运行中关闭「使用情况访问」：

```text
13:30:40 POST /v1/device/tamper → 200
13:30:40 OQG_GUARD: tamper reported type=permission_revoked detail={permission=usage_stats}
13:30:40 PUT /v1/device/capability → 200
```

- 在应用运行中移除无障碍服务：

```text
13:33:56 OQG_GUARD: tamper reported type=permission_revoked detail={permission=accessibility}
13:33:56 PUT /v1/device/capability → 200
```

- `tamper_event` 新增 `permission_revoked / usage_stats` 与 `permission_revoked / accessibility` 两条记录；
- 家长端告警接口返回未读告警，含孩子与设备信息。

## 3. 截图

| 步骤 | 截图 |
| --- | --- |
| 答题页 | [01-quiz-question.png](fullchain-emulator/01-quiz-question.png) |
| 答题通过 | [02-quiz-passed.png](fullchain-emulator/02-quiz-passed.png) |
| 时段锁定 | [03-time-window-blocked.png](fullchain-emulator/03-time-window-blocked.png) |
| 阻挡页家长解锁 | [04-blocker-parent.png](fullchain-emulator/04-blocker-parent.png) |
| 房间码输入 | [05-battle-join.png](fullchain-emulator/05-battle-join.png) |
| 孩子作答 | [06-battle-play.png](fullchain-emulator/06-battle-play.png) |
| 对战结果 | [07-battle-result.png](fullchain-emulator/07-battle-result.png) |
| 诊断页 | [08-diagnostics.png](fullchain-emulator/08-diagnostics.png) |

## 4. 未覆盖与限制

- **触摸注入无效**：本机模拟器无法通过 `adb shell input tap` 触发 Compose 按钮，全部操作改用方向键 + 确认键完成，因此未验证触摸交互。
- **误触重置绑定**：导航过程中曾误触「重置绑定」，已重新激活并继续；不影响上述结论。
- 媒体播放由 `app_process` + `AudioTrack` 模拟，并非真实视频 App；真实播放器识别仍以真机 074 结论为准。
- 模拟器为 Android 14，真机为 Android 9；两者在无障碍、权限弹窗上的差异未逐项对比。
- 本报告只做验收，不涉及代码变更。
