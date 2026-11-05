# 049 TV 日志落盘与诊断导出验收报告

| 项目 | 内容 |
| --- | --- |
| 版本 | v0.41.0（TV 0.3.3） |
| 日期 | 2026-11-05 |
| 范围 | 049：TV 日志落盘、轮转与诊断导出 |
| 设计/计划 | [设计](../design/049-TV日志落盘与诊断导出设计.md) / [计划](../plans/049-TV日志落盘与诊断导出实施计划.md) |

## 1. 结论

**通过（构建 + 单元测试级）。** TV 端关键事件写入私有日志文件，1MB 轮转；诊断页可通过 FileProvider 导出并分享日志。无 TV 实机，未做真机分享验证。

## 2. 交付

- `LogStore`：写入 `files/logs/guard.log`，1MB 轮转 `guard.log.1`，异常吞掉不影响主流程；
- `GuardLog`：统一 i/w/e 入口，同时写 logcat、诊断内存列表与文件；
- 埋点：启动、激活/绑定、令牌刷新/失效、授权采纳/刷新、心跳失败、答题/结算失败、服务拉起异常；
- `LogExporter`：合并轮转文件，导出 `openquizgo-tv-log-YYYYMMDD-HHmm.txt`；
- FileProvider 仅暴露 `files/logs/`，`ACTION_SEND` + 读权限；
- 诊断页「导出日志」按钮；
- TV 版本 0.3.3（versionCode 6）。

## 3. 验证

```text
./gradlew :app:testDebugUnitTest        BUILD SUCCESSFUL（7 项，新增 4 项）
./gradlew :app:compileDebugKotlin       BUILD SUCCESSFUL
./gradlew :app:assembleRelease          BUILD SUCCESSFUL
```

单元测试覆盖：写入、超过 1MB 轮转、并发写入、写入失败不抛异常。

## 4. 限制

- 同步小 IO，极端高频日志可能有轻微主线程开销；
- 部分电视系统无处理 `ACTION_SEND` 的应用，分享会失败但不崩溃；
- 未做 TV 实机导出验证；
- 未实现远程上传。

## 5. 后续

- 远程诊断上传；
- 崩溃日志捕获；
- 日志级别配置。
