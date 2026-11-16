package com.openquizgo.guard.ui.pad

import android.os.Build
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.foundation.text.KeyboardOptions
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.data.remote.TvDeviceResponse
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.platform.GuardAccessibilityService
import com.openquizgo.guard.platform.LogExporter
import com.openquizgo.guard.platform.Permissions
import com.openquizgo.guard.ui.GuardUiState
import com.openquizgo.guard.ui.GuardViewModel
import kotlinx.coroutines.delay

/**
 * 084：平板入口界面（登录 / 状态 / 权限引导 / 诊断）。
 */

@Composable
internal fun PadGuardScreen(
    viewModel: GuardViewModel,
    diagnosticsRequested: Boolean,
    onDiagnosticsClosed: () -> Unit,
    onRequestDiagnostics: () -> Unit,
    onStartQuiz: () -> Unit,
) {
    val state by viewModel.state.collectAsState()
    when (val current = state) {
        is GuardUiState.Loading -> PadMessageScreen(
            title = "正在连接…",
            detail = "稍等一下",
        )
        is GuardUiState.NeedsLogin -> PadLoginScreen(viewModel)
        is GuardUiState.Ready ->
            if (diagnosticsRequested) {
                PadDiagnosticsScreen(viewModel = viewModel, onBack = onDiagnosticsClosed)
            } else {
                PadStatusScreen(
                    device = current.device,
                    viewModel = viewModel,
                    onStartQuiz = onStartQuiz,
                    onRequestDiagnostics = onRequestDiagnostics,
                )
            }
        is GuardUiState.Failure -> PadMessageScreen(
            title = "连不上服务端",
            detail = current.message,
            primary = "重新连接" to { viewModel.retryBootstrap() },
        )
    }
}

/* ------------------------------ 登录 ------------------------------ */

@Composable
internal fun PadLoginScreen(viewModel: GuardViewModel) {
    val form by viewModel.login.collectAsState()

    PadPage {
        Spacer(Modifier.height(24.dp))
        PadTitle("登录守护端")
        Spacer(Modifier.height(6.dp))
        PadSubtitle("用家长手机号登录，登录状态会保持一年")
        Spacer(Modifier.height(24.dp))

        OutlinedTextField(
            value = form.phone,
            onValueChange = viewModel::updatePhone,
            label = { Text("手机号") },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone),
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(12.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            OutlinedTextField(
                value = form.code,
                onValueChange = viewModel::updateCode,
                label = { Text("验证码") },
                singleLine = true,
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                modifier = Modifier.weight(1f),
            )
            PadSecondaryButton(
                text = when {
                    form.sending -> "发送中…"
                    form.resendCountdown > 0 -> "${form.resendCountdown}s"
                    else -> "获取验证码"
                },
                onClick = viewModel::sendCode,
                enabled = !form.sending && form.resendCountdown == 0,
            )
        }

        form.message?.let {
            Spacer(Modifier.height(10.dp))
            Text(
                text = it,
                fontSize = 13.sp,
                color = if (form.error) PadBad else PadGood,
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth(),
            )
        }

        Spacer(Modifier.height(20.dp))
        PadPrimaryButton(
            text = if (form.loggingIn) "登录中…" else "登录",
            onClick = viewModel::login,
            enabled = !form.loggingIn,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(24.dp))
        PadSubtitle("登录后这台设备会出现在家长端「设备」里")
    }
}

/* ------------------------------ 状态 ------------------------------ */

@Composable
internal fun PadStatusScreen(
    device: TvDeviceResponse,
    viewModel: GuardViewModel,
    onStartQuiz: () -> Unit,
    onRequestDiagnostics: () -> Unit,
) {
    val context = LocalContext.current
    val guardState by viewModel.guardState.collectAsState()
    val contextRemaining by viewModel.contextRemaining.collectAsState()
    val contextUsed by viewModel.contextUsed.collectAsState()
    val contextLimit by viewModel.contextLimit.collectAsState()
    val contextReason by viewModel.contextReason.collectAsState()
    val grantSeconds by viewModel.grantSeconds.collectAsState()
    val hardStop by viewModel.hardStop.collectAsState()
    val nextQuizAtMillis by viewModel.nextQuizAtMillis.collectAsState()

    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) {
        while (true) {
            now = System.currentTimeMillis()
            delay(1_000)
        }
    }

    var accessibilityOn by remember { mutableStateOf(Permissions.accessibilityEnabled(context)) }
    var usageOn by remember { mutableStateOf(Permissions.usageAccessEnabled(context)) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(2_000)
            accessibilityOn = Permissions.accessibilityEnabled(context)
            usageOn = Permissions.usageAccessEnabled(context)
        }
    }

    // 隐藏诊断入口：连点版本号 5 次。
    var taps by remember { mutableStateOf(0) }
    var firstTapAt by remember { mutableLongStateOf(0L) }
    val onVersionTap: () -> Unit = {
        val current = System.currentTimeMillis()
        if (taps == 0 || current - firstTapAt > 3_000) {
            taps = 0
            firstTapAt = current
        }
        taps += 1
        if (taps >= 5) {
            taps = 0
            onRequestDiagnostics()
        }
    }

    val child = device.childName?.takeIf { it.isNotBlank() }
    val usedMinutes = contextUsed ?: device.usedMinutes
    val limitMinutes = contextLimit ?: device.config.dailyLimit

    PadPage {
        Spacer(Modifier.height(8.dp))
        PadTitle(child ?: "这台${if (device.platform == "pad") "平板" else "电视"}")
        PadSubtitle(
            (device.name.ifBlank { "守护端" }) +
                " · " + (if (device.platform == "pad") "平板" else "电视") + " · 守护中",
        )
        Spacer(Modifier.height(20.dp))

        PadCard {
            PadRowMetrics(
                Triple("今日已看", "$usedMinutes 分", PadAccent),
                Triple("今日剩余", remainingLabel(contextRemaining), remainingColor(contextRemaining)),
                Triple("下次答题", padCountdownLabel(nextQuizAtMillis, now), PadWarn),
            )
            Spacer(Modifier.height(12.dp))
            Text(
                text = if (limitMinutes > 0) "每日上限 $limitMinutes 分" else "未设置每日上限",
                fontSize = 12.sp,
                color = Color(0xFF808080),
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth(),
            )
        }

        Spacer(Modifier.height(12.dp))
        PadCard {
            Text(
                text = when {
                    hardStop -> "今天的观看时间用完了"
                    guardState == GuardState.QUIZ_REQUIRED -> "需要答题才能继续看"
                    guardState == GuardState.QUIZZING -> "正在答题"
                    guardState == GuardState.BLOCKED -> "已锁定，请答题或让家长解锁"
                    guardState == GuardState.GRANTED -> "已解锁，可以继续看"
                    guardState == GuardState.WATCHING -> "正在观看"
                    else -> "可以正常观看"
                },
                fontSize = 17.sp,
                color = when {
                    hardStop -> PadBad
                    contextReason == null || contextReason == "ok" || contextReason == "grant" -> PadGood
                    else -> PadWarn
                },
            )
            contextReason?.takeIf { it != "ok" && it != "grant" }?.let {
                Spacer(Modifier.height(4.dp))
                Text(text = padReasonLabel(it), fontSize = 12.sp, color = PadTextSecondary)
            }
            if (grantSeconds > 0) {
                Spacer(Modifier.height(4.dp))
                Text(
                    text = "剩余放行 ${padFormatDuration(grantSeconds)}",
                    fontSize = 12.sp,
                    color = PadGood,
                )
            }
            Spacer(Modifier.height(14.dp))
            PadPrimaryButton(
                text = if (guardState == GuardState.QUIZ_REQUIRED) "开始答题" else "去答题",
                onClick = onStartQuiz,
                modifier = Modifier.fillMaxWidth(),
            )
        }

        if (!accessibilityOn || !usageOn) {
            Spacer(Modifier.height(12.dp))
            PadPermissionsCard(accessibilityOn = accessibilityOn, usageOn = usageOn)
        }

        Spacer(Modifier.height(20.dp))
        PadVersionFooter(onTap = onVersionTap, onLogout = viewModel::logout)
    }
}

@Composable
private fun PadPermissionsCard(accessibilityOn: Boolean, usageOn: Boolean) {
    val context = LocalContext.current
    PadCard {
        Text(text = "还差两步才能生效", fontSize = 16.sp, color = PadWarn)
        Spacer(Modifier.height(6.dp))
        Text(
            text = "缺少权限时，孩子打开视频 App 不会被拦截。",
            fontSize = 12.sp,
            color = PadTextSecondary,
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = if (accessibilityOn) "✓ 无障碍服务已开启" else "1. 无障碍服务：用来识别孩子正在看视频",
            fontSize = 13.sp,
            color = if (accessibilityOn) PadGood else PadTextPrimary,
        )
        if (!accessibilityOn) {
            Spacer(Modifier.height(8.dp))
            PadSecondaryButton(
                text = "去开启无障碍",
                onClick = { runCatching { context.startActivity(Permissions.accessibilitySettingsIntent()) } },
                modifier = Modifier.fillMaxWidth(),
            )
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                Spacer(Modifier.height(8.dp))
                Text(
                    text = "如果开关是灰的：应用信息 → 右上角三点 → 允许受限设置，再回来开启。",
                    fontSize = 12.sp,
                    color = PadWarn,
                )
            }
        }
        Spacer(Modifier.height(12.dp))
        Text(
            text = if (usageOn) "✓ 使用情况统计已开启" else "2. 使用情况统计：用来判断哪个 App 在前台",
            fontSize = 13.sp,
            color = if (usageOn) PadGood else PadTextPrimary,
        )
        if (!usageOn) {
            Spacer(Modifier.height(8.dp))
            PadSecondaryButton(
                text = "去开启使用情况",
                onClick = { runCatching { context.startActivity(Permissions.usageSettingsIntent()) } },
                modifier = Modifier.fillMaxWidth(),
            )
        }
        Spacer(Modifier.height(10.dp))
        PadBanner(
            text = "开启后如果一会儿又失效：把守护端设为「自启动」并允许后台运行，不要清理后台。",
        )
    }
}

@Composable
private fun PadVersionFooter(onTap: () -> Unit, onLogout: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = "版本 ${BuildConfig.VERSION_NAME}",
            fontSize = 11.sp,
            color = Color(0xFF606060),
            modifier = Modifier.clickable { onTap() },
        )
        Text(
            text = "退出登录",
            fontSize = 12.sp,
            color = Color(0xFF808080),
            modifier = Modifier.clickable { onLogout() },
        )
    }
}

/* ------------------------------ 诊断（隐藏） ------------------------------ */

@Composable
internal fun PadDiagnosticsScreen(viewModel: GuardViewModel, onBack: () -> Unit) {
    val context = LocalContext.current
    val logs by viewModel.logs.collectAsState()
    val report by viewModel.reportStatus.collectAsState()
    val guardState by viewModel.guardState.collectAsState()
    val watchSeconds by viewModel.watchSeconds.collectAsState()

    PadPage(horizontalAlignment = Alignment.Start) {
        PadTitle("诊断信息")
        Spacer(Modifier.height(8.dp))
        Text("版本：${BuildConfig.VERSION_NAME}", fontSize = 12.sp, color = PadTextSecondary)
        Text("服务端：${BuildConfig.API_BASE_URL}", fontSize = 12.sp, color = PadTextSecondary)
        Text(
            "状态：${padGuardStateLabel(guardState)} ｜ 本次观看 ${watchSeconds}s",
            fontSize = 12.sp,
            color = PadAccent,
        )
        Text(
            "无障碍：${if (GuardAccessibilityService.isConnected()) "已连接" else "未连接"}",
            fontSize = 12.sp,
            color = PadTextSecondary,
        )
        Spacer(Modifier.height(12.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            PadSecondaryButton(text = "导出日志", onClick = { LogExporter.share(context) })
            PadSecondaryButton(text = "上传诊断", onClick = viewModel::uploadDiagnostics)
        }
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            PadSecondaryButton(text = "上报能力", onClick = viewModel::reportCapabilityNow)
            PadPrimaryButton(text = "返回", onClick = onBack)
        }
        report?.let {
            Spacer(Modifier.height(8.dp))
            Text(text = it, fontSize = 12.sp, color = PadGood)
        }
        Spacer(Modifier.height(12.dp))
        Text(text = "最近日志", fontSize = 13.sp, color = PadGood)
        logs.takeLast(12).forEach { line ->
            Text(text = line, fontSize = 10.sp, color = Color(0xFF909090))
        }
    }
}

@Composable
internal fun PadMessageScreen(
    title: String,
    detail: String,
    primary: Pair<String, () -> Unit>? = null,
    secondary: Pair<String, () -> Unit>? = null,
) {
    PadPage {
        Spacer(Modifier.height(48.dp))
        PadTitle(title)
        Spacer(Modifier.height(10.dp))
        Text(
            text = detail,
            fontSize = 14.sp,
            color = PadTextSecondary,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(20.dp))
        primary?.let {
            PadPrimaryButton(text = it.first, onClick = it.second, modifier = Modifier.fillMaxWidth())
            Spacer(Modifier.height(10.dp))
        }
        secondary?.let {
            PadSecondaryButton(text = it.first, onClick = it.second, modifier = Modifier.fillMaxWidth())
        }
    }
}

private fun remainingLabel(minutes: Int?): String =
    if (minutes == null) "同步中…" else "$minutes 分"

private fun remainingColor(minutes: Int?): Color = when {
    minutes == null -> PadTextSecondary
    minutes <= 0 -> PadBad
    else -> PadGood
}

internal fun padCountdownLabel(targetMillis: Long?, now: Long): String {
    if (targetMillis == null) return "—"
    val remaining = targetMillis - now
    if (remaining <= 0) return "随时"
    return padFormatDuration(remaining / 1000)
}

internal fun padReasonLabel(reason: String?): String = when (reason) {
    "time_window" -> "不在允许观看的时段"
    "daily_limit" -> "今日额度已用完"
    "quiz_failed" -> "答题未通过"
    "revoked" -> "观看授权已失效"
    else -> reason ?: ""
}

internal fun padGuardStateLabel(state: GuardState): String = when (state) {
    GuardState.UNBOUND -> "未登录"
    GuardState.IDLE -> "空闲"
    GuardState.WATCHING -> "观看中"
    GuardState.QUIZ_REQUIRED -> "需要答题"
    GuardState.QUIZZING -> "答题中"
    GuardState.GRANTED -> "已授权"
    GuardState.BLOCKED -> "已锁定"
}
