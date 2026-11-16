package com.openquizgo.guard.ui

import android.content.Intent
import android.os.Bundle
import android.view.KeyEvent
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModelProvider
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.platform.FormFactors
import com.openquizgo.guard.platform.GuardAccessibilityService
import com.openquizgo.guard.platform.GuardService
import com.openquizgo.guard.platform.LogExporter
import com.openquizgo.guard.ui.pad.PadGuardScreen
import com.openquizgo.guard.ui.pad.PadTheme
import kotlinx.coroutines.delay

class MainActivity : ComponentActivity() {

    /** 连按 5 次确认键呼出诊断页：电视端不再有可见的运维大屏。 */
    private var confirmTaps = 0
    private var firstConfirmAt = 0L
    private val diagnosticsRequested = mutableStateOf(false)
    private val loginField = mutableStateOf(LoginField.PHONE)
    private var viewModel: GuardViewModel? = null

    /** 084：电视走遥控器界面，平板/手机走触摸界面。 */
    private val isTelevision: Boolean by lazy { FormFactors.isTelevision(this) }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = (application as GuardApplication).container
        val vm = ViewModelProvider(
            this,
            GuardViewModel.Factory(container),
        )[GuardViewModel::class.java]
        viewModel = vm

        GuardService.start(this)

        setContent {
            if (isTelevision) {
                MaterialTheme(colorScheme = darkColorScheme()) {
                    Surface(modifier = Modifier.fillMaxSize()) {
                        GuardScreen(
                            viewModel = vm,
                            diagnosticsRequested = diagnosticsRequested.value,
                            onDiagnosticsClosed = { diagnosticsRequested.value = false },
                            loginField = loginField.value,
                            onLoginFieldChange = { loginField.value = it },
                        )
                    }
                }
            } else {
                PadTheme {
                    PadGuardScreen(
                        viewModel = vm,
                        diagnosticsRequested = diagnosticsRequested.value,
                        onDiagnosticsClosed = { diagnosticsRequested.value = false },
                        onRequestDiagnostics = { diagnosticsRequested.value = true },
                        onStartQuiz = ::startQuiz,
                    )
                }
            }
        }
    }

    /** 平板主页主动去答题：先置为需要答题，再拉起全屏答题页。 */
    private fun startQuiz() {
        val container = (application as GuardApplication).container
        container.guardEngine.forceQuizRequired()
        val intent = Intent(this, QuizActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        runCatching { startActivity(intent) }
    }

    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        // 电视遥控器通常带数字键：登录页直接用它输入手机号/验证码。
        // 084：平板上必须放行，否则会抢走软键盘输入的字符。
        if (isTelevision && event.action == KeyEvent.ACTION_DOWN &&
            viewModel?.state?.value is GuardUiState.NeedsLogin
        ) {
            val digit = digitOf(event.keyCode)
            if (digit != null) {
                val form = viewModel?.login?.value
                if (loginField.value == LoginField.PHONE) {
                    viewModel?.updatePhone((form?.phone ?: "") + digit)
                } else {
                    viewModel?.updateCode((form?.code ?: "") + digit)
                }
                return true
            }
            if (event.keyCode == KeyEvent.KEYCODE_DEL) {
                val form = viewModel?.login?.value
                if (loginField.value == LoginField.PHONE) {
                    viewModel?.updatePhone((form?.phone ?: "").dropLast(1))
                } else {
                    viewModel?.updateCode((form?.code ?: "").dropLast(1))
                }
                return true
            }
        }
        if (event.action == KeyEvent.ACTION_DOWN && isConfirmKey(event.keyCode) &&
            viewModel?.state?.value is GuardUiState.Ready
        ) {
            val now = System.currentTimeMillis()
            if (confirmTaps == 0 || now - firstConfirmAt > CONFIRM_WINDOW_MS) {
                confirmTaps = 0
                firstConfirmAt = now
            }
            confirmTaps += 1
            if (confirmTaps >= CONFIRM_TAPS) {
                confirmTaps = 0
                diagnosticsRequested.value = true
            }
        }
        return super.dispatchKeyEvent(event)
    }

    private fun isConfirmKey(keyCode: Int): Boolean =
        keyCode == KeyEvent.KEYCODE_DPAD_CENTER ||
            keyCode == KeyEvent.KEYCODE_ENTER ||
            keyCode == KeyEvent.KEYCODE_NUMPAD_ENTER

    private fun digitOf(keyCode: Int): String? = when (keyCode) {
        in KeyEvent.KEYCODE_0..KeyEvent.KEYCODE_9 ->
            (keyCode - KeyEvent.KEYCODE_0).toString()
        in KeyEvent.KEYCODE_NUMPAD_0..KeyEvent.KEYCODE_NUMPAD_9 ->
            (keyCode - KeyEvent.KEYCODE_NUMPAD_0).toString()
        else -> null
    }

    private companion object {
        const val CONFIRM_TAPS = 5
        const val CONFIRM_WINDOW_MS = 3_000L
    }
}

@Composable
private fun GuardScreen(
    viewModel: GuardViewModel,
    diagnosticsRequested: Boolean,
    onDiagnosticsClosed: () -> Unit,
    loginField: LoginField,
    onLoginFieldChange: (LoginField) -> Unit,
) {
    val state by viewModel.state.collectAsState()
    when (val current = state) {
        is GuardUiState.Loading -> CenterMessage("正在连接服务端…")
        is GuardUiState.NeedsLogin ->
            LoginScreen(viewModel, loginField, onLoginFieldChange)
        is GuardUiState.Ready ->
            if (diagnosticsRequested) {
                DiagnosticsScreen(viewModel = viewModel, onBack = onDiagnosticsClosed)
            } else {
                StatusScreen(device = current.device, viewModel = viewModel)
            }
        is GuardUiState.Failure -> CenterMessage(
            "无法连接服务端：${current.message}\n请检查网络后重启电视",
            color = Color(0xFFFF7A7A),
        )
    }
}

@Composable
private fun CenterMessage(message: String, color: Color = Color(0xFFE0E0E0)) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = message, fontSize = 28.sp, color = color)
    }
}

/* ------------------------------ 登录 ------------------------------ */

private enum class LoginField { PHONE, CODE }

@Composable
private fun LoginScreen(
    viewModel: GuardViewModel,
    field: LoginField,
    onFieldChange: (LoginField) -> Unit,
) {
    val form by viewModel.login.collectAsState()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 48.dp, vertical = 28.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = "登录电视", fontSize = 34.sp, fontWeight = FontWeight.Bold, color = Color.White)
        Spacer(Modifier.height(4.dp))
        Text(
            text = "用家长手机号登录，登录状态保持一年",
            fontSize = 18.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(18.dp))

        FieldRow(
            label = "手机号",
            value = form.phone.ifEmpty { "点此输入" },
            highlighted = field == LoginField.PHONE,
            onSelect = { onFieldChange(LoginField.PHONE) },
        )
        Spacer(Modifier.height(8.dp))
        FieldRow(
            label = "验证码",
            value = if (form.code.isEmpty()) "点此输入" else form.code,
            highlighted = field == LoginField.CODE,
            onSelect = { onFieldChange(LoginField.CODE) },
        )

        Spacer(Modifier.height(12.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Button(
                onClick = viewModel::sendCode,
                enabled = !form.sending && form.resendCountdown == 0,
                modifier = Modifier.widthIn(min = 210.dp),
            ) {
                Text(
                    text = when {
                        form.sending -> "发送中…"
                        form.resendCountdown > 0 -> "${form.resendCountdown} 秒后重发"
                        else -> "获取验证码"
                    },
                    fontSize = 18.sp,
                )
            }
            Button(
                onClick = viewModel::login,
                enabled = !form.loggingIn,
                modifier = Modifier.widthIn(min = 210.dp),
            ) {
                Text(if (form.loggingIn) "登录中…" else "登录", fontSize = 18.sp)
            }
        }

        form.message?.let {
            Spacer(Modifier.height(8.dp))
            Text(
                text = it,
                fontSize = 16.sp,
                color = if (form.error) Color(0xFFFF7A7A) else Color(0xFF7CFF9B),
            )
        }

        Spacer(Modifier.height(12.dp))
        Keypad(
            onDigit = { digit ->
                when (field) {
                    LoginField.PHONE -> viewModel.updatePhone(form.phone + digit)
                    LoginField.CODE -> viewModel.updateCode(form.code + digit)
                }
            },
            onClear = {
                when (field) {
                    LoginField.PHONE -> viewModel.updatePhone("")
                    LoginField.CODE -> viewModel.updateCode("")
                }
            },
            onDelete = {
                when (field) {
                    LoginField.PHONE -> viewModel.updatePhone(form.phone.dropLast(1))
                    LoginField.CODE -> viewModel.updateCode(form.code.dropLast(1))
                }
            },
        )
    }
}

@Composable
private fun FieldRow(
    label: String,
    value: String,
    highlighted: Boolean,
    onSelect: () -> Unit,
) {
    Button(
        onClick = onSelect,
        modifier = Modifier.widthIn(min = 520.dp),
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(16.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(text = label, fontSize = 20.sp, color = Color(0xFFB0B0B0))
            Text(
                text = value,
                fontSize = 24.sp,
                fontWeight = if (highlighted) FontWeight.Bold else FontWeight.Normal,
                color = if (highlighted) Color(0xFF7CC7FF) else Color.White,
            )
        }
    }
}

@Composable
private fun Keypad(
    onDigit: (String) -> Unit,
    onClear: () -> Unit,
    onDelete: () -> Unit,
) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        for (row in listOf(listOf("1", "2", "3"), listOf("4", "5", "6"), listOf("7", "8", "9"))) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                row.forEach { digit ->
                    Button(
                        onClick = { onDigit(digit) },
                        modifier = Modifier.widthIn(min = 68.dp),
                    ) { Text(digit, fontSize = 24.sp) }
                }
            }
            Spacer(Modifier.height(4.dp))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = onClear, modifier = Modifier.widthIn(min = 76.dp)) {
                Text("清除", fontSize = 16.sp)
            }
            Button(onClick = { onDigit("0") }, modifier = Modifier.widthIn(min = 76.dp)) {
                Text("0", fontSize = 24.sp)
            }
            Button(onClick = onDelete, modifier = Modifier.widthIn(min = 76.dp)) {
                Text("删除", fontSize = 16.sp)
            }
        }
    }
}

/* ------------------------------ 只读状态页 ------------------------------ */

@Composable
private fun StatusScreen(
    device: com.openquizgo.guard.data.remote.TvDeviceResponse,
    viewModel: GuardViewModel,
) {
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

    val child = device.childName?.takeIf { it.isNotBlank() }
    val remainingMinutes = contextRemaining
    val usedMinutes = contextUsed ?: device.usedMinutes
    val limitMinutes = contextLimit ?: device.config.dailyLimit

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(56.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = if (child == null) "本台电视" else child,
            fontSize = 40.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        Text(
            text = device.name.ifBlank { "客厅电视" } + " · 守护中",
            fontSize = 18.sp,
            color = Color(0xFF808080),
        )
        Spacer(Modifier.height(28.dp))

        Row(horizontalArrangement = Arrangement.spacedBy(56.dp)) {
            MetricBlock(
                label = "今日已看",
                value = "$usedMinutes 分",
                color = Color(0xFF7CC7FF),
            )
            MetricBlock(
                label = "今日剩余",
                value = if (remainingMinutes == null) "同步中…" else "$remainingMinutes 分",
                color = when {
                    remainingMinutes == null -> Color(0xFFB0B0B0)
                    remainingMinutes <= 0 -> Color(0xFFFF7A7A)
                    else -> Color(0xFF7CFF9B)
                },
                hint = "上限 $limitMinutes 分",
            )
            MetricBlock(
                label = "下次答题",
                value = countdownLabel(nextQuizAtMillis, now),
                color = Color(0xFFFFC107),
            )
        }

        if (grantSeconds > 0) {
            Spacer(Modifier.height(20.dp))
            Text(
                text = "家长已放行：剩余 ${formatDuration(grantSeconds)}",
                fontSize = 20.sp,
                color = Color(0xFF7CFF9B),
            )
        }

        Spacer(Modifier.height(24.dp))
        Text(
            text = when {
                hardStop -> "今日观看时间已用完，明天再来"
                guardState == GuardState.QUIZ_REQUIRED -> "该答题啦"
                guardState == GuardState.QUIZZING -> "答题中"
                guardState == GuardState.BLOCKED -> "休息一下"
                else -> "可以正常观看"
            },
            fontSize = 24.sp,
            color = when {
                hardStop -> Color(0xFFFF7A7A)
                contextReason == null || contextReason == "ok" || contextReason == "grant" -> Color(0xFF7CFF9B)
                else -> Color(0xFFFFC107)
            },
        )
        contextReason?.takeIf { it != "ok" && it != "grant" }?.let {
            Spacer(Modifier.height(6.dp))
            Text(text = contextReasonLabel(it), fontSize = 16.sp, color = Color(0xFFB0B0B0))
        }
    }
}

@Composable
private fun MetricBlock(
    label: String,
    value: String,
    color: Color,
    hint: String? = null,
) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(text = label, fontSize = 18.sp, color = Color(0xFFB0B0B0))
        Spacer(Modifier.height(6.dp))
        Text(text = value, fontSize = 34.sp, fontWeight = FontWeight.Bold, color = color)
        hint?.let {
            Spacer(Modifier.height(4.dp))
            Text(text = it, fontSize = 14.sp, color = Color(0xFF707070))
        }
    }
}

private fun countdownLabel(targetMillis: Long?, now: Long): String {
    if (targetMillis == null) return "—"
    val remaining = targetMillis - now
    if (remaining <= 0) return "随时"
    return formatDuration(remaining / 1000)
}

private fun formatDuration(totalSeconds: Long): String {
    val seconds = totalSeconds.coerceAtLeast(0)
    val minutes = seconds / 60
    val rest = seconds % 60
    return if (minutes > 0) "%d 分 %02d 秒".format(minutes, rest) else "${rest} 秒"
}

private fun contextReasonLabel(reason: String?): String = when (reason) {
    "time_window" -> "不在允许观看的时段"
    "daily_limit" -> "今日额度已用完"
    else -> reason ?: ""
}

/* ------------------------------ 诊断（隐藏） ------------------------------ */

@Composable
private fun DiagnosticsScreen(viewModel: GuardViewModel, onBack: () -> Unit) {
    val logs by viewModel.logs.collectAsState()
    val report by viewModel.reportStatus.collectAsState()
    val guardState by viewModel.guardState.collectAsState()
    val watchSeconds by viewModel.watchSeconds.collectAsState()
    val grantSeconds by viewModel.grantSeconds.collectAsState()
    val context = LocalContext.current

    Column(
        modifier = Modifier.fillMaxSize().padding(40.dp),
        verticalArrangement = Arrangement.Top,
        horizontalAlignment = Alignment.Start,
    ) {
        Text(text = "诊断信息", fontSize = 28.sp, color = Color.White)
        Spacer(Modifier.height(8.dp))
        Text(text = "版本：${BuildConfig.VERSION_NAME}", fontSize = 15.sp, color = Color(0xFFB0B0B0))
        Text(text = "服务端：${BuildConfig.API_BASE_URL}", fontSize = 15.sp, color = Color(0xFFB0B0B0))
        Text(
            text = "状态：${guardStateLabel(guardState)} ｜ 本次观看 ${watchSeconds}s ｜ 剩余 ${grantSeconds}s",
            fontSize = 15.sp,
            color = Color(0xFF7CC7FF),
        )
        Text(
            text = "无障碍：${if (GuardAccessibilityService.isConnected()) "已连接" else "未连接"}",
            fontSize = 15.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(onClick = { LogExporter.share(context) }) { Text("导出日志", fontSize = 14.sp) }
            Button(onClick = viewModel::uploadDiagnostics) { Text("上传诊断", fontSize = 14.sp) }
            Button(onClick = viewModel::reportCapabilityNow) { Text("上报能力", fontSize = 14.sp) }
            Button(onClick = onBack) { Text("返回", fontSize = 14.sp) }
        }
        report?.let {
            Spacer(Modifier.height(6.dp))
            Text(text = it, fontSize = 14.sp, color = Color(0xFF7CFF9B))
        }
        Spacer(Modifier.height(10.dp))
        Text(text = "最近日志", fontSize = 16.sp, color = Color(0xFF7CFF9B))
        logs.takeLast(10).forEach { line ->
            Text(text = line, fontSize = 12.sp, color = Color(0xFF909090))
        }
    }
}

private fun guardStateLabel(state: GuardState): String = when (state) {
    GuardState.UNBOUND -> "未登录"
    GuardState.IDLE -> "空闲"
    GuardState.WATCHING -> "观看中"
    GuardState.QUIZ_REQUIRED -> "需要答题"
    GuardState.QUIZZING -> "答题中"
    GuardState.GRANTED -> "已授权"
    GuardState.BLOCKED -> "已锁定"
}
