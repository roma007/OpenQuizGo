package com.openquizgo.guard.ui

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
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
import com.openquizgo.guard.data.foreground.ForegroundState
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.platform.GuardAccessibilityService
import com.openquizgo.guard.platform.GuardService
import com.openquizgo.guard.platform.LogExporter
import com.openquizgo.guard.platform.Permissions
import kotlinx.coroutines.delay

class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = (application as GuardApplication).container
        val viewModel = ViewModelProvider(
            this,
            GuardViewModel.Factory(container),
        )[GuardViewModel::class.java]

        GuardService.start(this)

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    GuardScreen(viewModel)
                }
            }
        }
    }
}

private enum class Dashboard { HOME, WIZARD, DIAGNOSTICS }

@Composable
private fun GuardScreen(viewModel: GuardViewModel) {
    val state by viewModel.state.collectAsState()
    val foreground by viewModel.foregroundState.collectAsState()
    val guardState by viewModel.guardState.collectAsState()
    val watchSeconds by viewModel.watchSeconds.collectAsState()
    val grantSeconds by viewModel.grantSeconds.collectAsState()
    var dashboard by remember { mutableStateOf(Dashboard.HOME) }
    when (val current = state) {
        is GuardUiState.Loading -> CenterMessage("正在连接服务端…")
        is GuardUiState.Activation -> ActivationScreen(
            state = current,
            onRegenerate = viewModel::regenerate,
        )
        is GuardUiState.Bound -> when (dashboard) {
            Dashboard.HOME -> BoundScreen(
                childName = current.childName,
                foreground = foreground,
                guardState = guardState,
                watchSeconds = watchSeconds,
                grantSeconds = grantSeconds,
                onWizard = { dashboard = Dashboard.WIZARD },
                onDiagnostics = { dashboard = Dashboard.DIAGNOSTICS },
                onReset = viewModel::resetBinding,
            )
            Dashboard.WIZARD -> PermissionWizardScreen(
                childName = current.childName,
                foreground = foreground,
                guardState = guardState,
                grantSeconds = grantSeconds,
                viewModel = viewModel,
                onBack = { dashboard = Dashboard.HOME },
            )
            Dashboard.DIAGNOSTICS -> DiagnosticsScreen(
                childName = current.childName,
                foreground = foreground,
                guardState = guardState,
                watchSeconds = watchSeconds,
                grantSeconds = grantSeconds,
                viewModel = viewModel,
                onBack = { dashboard = Dashboard.HOME },
            )
        }
        is GuardUiState.Failure -> FailureScreen(
            message = current.message,
            onRetry = viewModel::regenerate,
        )
    }
}

@Composable
private fun CenterMessage(message: String) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = message, fontSize = 28.sp, color = Color(0xFFE0E0E0))
    }
}

@Composable
private fun ActivationScreen(
    state: GuardUiState.Activation,
    onRegenerate: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "设备激活",
            fontSize = 32.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(16.dp))
        Text(
            text = state.code,
            fontSize = 96.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = 12.sp,
            color = Color.White,
        )
        Spacer(Modifier.height(24.dp))
        Text(
            text = if (state.expired) "激活码已过期" else "请在家长端输入此码，完成后电视会自动绑定",
            fontSize = 22.sp,
            color = if (state.expired) Color(0xFFFF7A7A) else Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(32.dp))
        if (state.expired) {
            Button(onClick = onRegenerate) {
                Text("重新生成", fontSize = 22.sp)
            }
        } else {
            Text(text = "等待家长确认…", fontSize = 20.sp, color = Color(0xFF7CFF9B))
        }
        Spacer(Modifier.height(40.dp))
        Text(
            text = "设备 ID：${state.deviceId.take(8)}…",
            fontSize = 14.sp,
            color = Color(0xFF606060),
        )
    }
}

@Composable
private fun BoundScreen(
    childName: String,
    foreground: ForegroundState,
    guardState: GuardState,
    watchSeconds: Long,
    grantSeconds: Long,
    onWizard: () -> Unit,
    onDiagnostics: () -> Unit,
    onReset: () -> Unit,
) {
    val context = LocalContext.current
    var accessibility by remember { mutableStateOf(Permissions.accessibilityEnabled(context)) }
    var usage by remember { mutableStateOf(Permissions.usageAccessEnabled(context)) }

    LaunchedEffect(Unit) {
        while (true) {
            accessibility = Permissions.accessibilityEnabled(context)
            usage = Permissions.usageAccessEnabled(context)
            delay(2000)
        }
    }

    Column(
        modifier = Modifier.fillMaxSize().padding(48.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = "设备已绑定", fontSize = 26.sp, color = Color(0xFF7CFF9B))
        Text(text = "孩子：$childName", fontSize = 34.sp, color = Color.White)
        Spacer(Modifier.height(16.dp))
        Text(
            text = "前台应用：${foreground.packageName ?: "—"}",
            fontSize = 18.sp,
            color = Color(0xFFB0B0B0),
        )
        Text(
            text = "媒体播放：${if (foreground.mediaActive) "是" else "否"} ｜ 计时：${foreground.mediaSeconds}s",
            fontSize = 18.sp,
            color = Color(0xFFB0B0B0),
        )
        Text(
            text = "无障碍：${if (accessibility) "✓" else "✗"} ｜ 使用情况访问：${if (usage) "✓" else "✗"}",
            fontSize = 18.sp,
            color = if (accessibility && usage) Color(0xFF7CFF9B) else Color(0xFFFFC107),
        )
        Text(
            text = "守护状态：${guardStateLabel(guardState)} ｜ 本次观看：${watchSeconds}s",
            fontSize = 18.sp,
            color = Color(0xFF7CC7FF),
        )
        Text(
            text = "剩余授权：${grantSeconds / 60} 分 ${grantSeconds % 60} 秒",
            fontSize = 18.sp,
            color = if (grantSeconds > 0) Color(0xFF7CFF9B) else Color(0xFFFFC107),
        )
        Spacer(Modifier.height(20.dp))
        Button(onClick = onWizard) {
            Text("权限向导 / 自检", fontSize = 18.sp)
        }
        Spacer(Modifier.height(10.dp))
        Button(onClick = onDiagnostics) {
            Text("诊断信息", fontSize = 18.sp)
        }
        Spacer(Modifier.height(10.dp))
        Button(onClick = {
            context.startActivity(Intent(context, BattleActivity::class.java))
        }) {
            Text("加入亲子对战", fontSize = 18.sp)
        }
        Spacer(Modifier.height(10.dp))
        Button(onClick = onReset) {
            Text("重置绑定", fontSize = 18.sp)
        }
    }
}

private fun guardStateLabel(state: GuardState): String = when (state) {
    GuardState.UNBOUND -> "未激活"
    GuardState.IDLE -> "空闲"
    GuardState.WATCHING -> "观看中"
    GuardState.QUIZ_REQUIRED -> "需要答题"
    GuardState.QUIZZING -> "答题中"
    GuardState.GRANTED -> "已授权"
    GuardState.BLOCKED -> "已锁定"
}

@Composable
private fun PermissionWizardScreen(
    childName: String,
    foreground: ForegroundState,
    guardState: GuardState,
    grantSeconds: Long,
    viewModel: GuardViewModel,
    onBack: () -> Unit,
) {
    val context = LocalContext.current
    var accessibility by remember { mutableStateOf(Permissions.accessibilityEnabled(context)) }
    var usage by remember { mutableStateOf(Permissions.usageAccessEnabled(context)) }
    var notifications by remember { mutableStateOf(Permissions.notificationsEnabled(context)) }
    var serviceRunning by remember { mutableStateOf(Permissions.foregroundServiceRunning(context)) }
    val server by viewModel.serverStatus.collectAsState()
    val report by viewModel.reportStatus.collectAsState()

    LaunchedEffect(Unit) {
        viewModel.pingServer()
        while (true) {
            accessibility = Permissions.accessibilityEnabled(context)
            usage = Permissions.usageAccessEnabled(context)
            notifications = Permissions.notificationsEnabled(context)
            serviceRunning = Permissions.foregroundServiceRunning(context)
            delay(2_000)
        }
    }

    val passed = listOf(accessibility, usage, notifications, serviceRunning).count { it }

    Column(
        modifier = Modifier.fillMaxSize().padding(48.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = "权限向导 / 自检", fontSize = 32.sp, color = Color.White)
        Text(
            text = "孩子：$childName ｜ 权限 $passed / 4",
            fontSize = 18.sp,
            color = if (passed == 4) Color(0xFF7CFF9B) else Color(0xFFFFC107),
        )
        Spacer(Modifier.height(14.dp))
        CheckRow("无障碍服务", accessibility, if (accessibility) null else "去开启") {
            context.startActivity(Permissions.accessibilitySettingsIntent())
        }
        CheckRow("使用情况访问", usage, if (usage) null else "去开启") {
            context.startActivity(Permissions.usageSettingsIntent())
        }
        CheckRow("通知权限", notifications, null, null)
        CheckRow("前台守护服务", serviceRunning, null, null)
        CheckRow(
            "服务端连接",
            server.ok == true,
            if (server.checking) "检测中…" else if (server.ok == false) "重试" else "检测",
        ) { viewModel.pingServer() }
        Text(
            text = "服务端：${server.message}",
            fontSize = 14.sp,
            color = if (server.ok == false) Color(0xFFFF7A7A) else Color(0xFF808080),
        )
        Spacer(Modifier.height(6.dp))
        Text(
            text = "守护状态：${guardStateLabel(guardState)} ｜ 剩余授权：${grantSeconds / 60} 分 ${grantSeconds % 60} 秒",
            fontSize = 16.sp,
            color = Color(0xFF7CC7FF),
        )
        Text(
            text = "前台：${foreground.packageName ?: "—"} ｜ 媒体：${if (foreground.mediaActive) "播放中" else "无"}",
            fontSize = 16.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(18.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Button(onClick = viewModel::reportCapabilityNow) {
                Text("上报设备能力", fontSize = 16.sp)
            }
            Button(onClick = onBack) {
                Text("返回", fontSize = 16.sp)
            }
        }
        report?.let {
            Spacer(Modifier.height(8.dp))
            Text(text = it, fontSize = 14.sp, color = Color(0xFF7CFF9B))
        }
    }
}

@Composable
private fun CheckRow(
    label: String,
    ok: Boolean,
    action: String?,
    onAction: (() -> Unit)?,
) {
    Row(
        modifier = Modifier.padding(vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        Text(
            text = "${if (ok) "✓" else "✗"} $label",
            fontSize = 20.sp,
            color = if (ok) Color(0xFF7CFF9B) else Color(0xFFFFC107),
        )
        if (action != null && onAction != null) {
            Button(onClick = onAction) {
                Text(action, fontSize = 14.sp)
            }
        }
    }
}

@Composable
private fun DiagnosticsScreen(
    childName: String,
    foreground: ForegroundState,
    guardState: GuardState,
    watchSeconds: Long,
    grantSeconds: Long,
    viewModel: GuardViewModel,
    onBack: () -> Unit,
) {
    val logs by viewModel.logs.collectAsState()
    val report by viewModel.reportStatus.collectAsState()
    val context = LocalContext.current

    Column(
        modifier = Modifier.fillMaxSize().padding(40.dp),
        verticalArrangement = Arrangement.Top,
        horizontalAlignment = Alignment.Start,
    ) {
        Text(text = "诊断信息", fontSize = 28.sp, color = Color.White)
        Spacer(Modifier.height(8.dp))
        Text(text = "孩子：$childName ｜ 版本：${BuildConfig.VERSION_NAME}", fontSize = 15.sp, color = Color(0xFFB0B0B0))
        Text(text = "服务端：${BuildConfig.API_BASE_URL}", fontSize = 15.sp, color = Color(0xFFB0B0B0))
        Text(
            text = "状态：${guardStateLabel(guardState)} ｜ 本次观看 ${watchSeconds}s ｜ 剩余 ${grantSeconds}s",
            fontSize = 15.sp,
            color = Color(0xFF7CC7FF),
        )
        Text(
            text = "前台：${foreground.packageName ?: "—"} ｜ 媒体：${if (foreground.mediaActive) "是" else "否"}",
            fontSize = 15.sp,
            color = Color(0xFFB0B0B0),
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

@Composable
private fun FailureScreen(message: String, onRetry: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = "激活失败", fontSize = 32.sp, color = Color(0xFFFF7A7A))
        Spacer(Modifier.height(16.dp))
        Text(text = message, fontSize = 20.sp, color = Color(0xFFB0B0B0))
        Spacer(Modifier.height(32.dp))
        Button(onClick = onRetry) {
            Text("重试", fontSize = 22.sp)
        }
    }
}
