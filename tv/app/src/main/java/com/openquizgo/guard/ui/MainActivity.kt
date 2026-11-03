package com.openquizgo.guard.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
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
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.data.foreground.ForegroundState
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.platform.GuardService
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

@Composable
private fun GuardScreen(viewModel: GuardViewModel) {
    val state by viewModel.state.collectAsState()
    val foreground by viewModel.foregroundState.collectAsState()
    val guardState by viewModel.guardState.collectAsState()
    val watchSeconds by viewModel.watchSeconds.collectAsState()
    when (val current = state) {
        is GuardUiState.Loading -> CenterMessage("正在连接服务端…")
        is GuardUiState.Activation -> ActivationScreen(
            state = current,
            onRegenerate = viewModel::regenerate,
        )
        is GuardUiState.Bound -> BoundScreen(
            childName = current.childName,
            foreground = foreground,
            guardState = guardState,
            watchSeconds = watchSeconds,
            onReset = viewModel::resetBinding,
        )
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
        Spacer(Modifier.height(20.dp))
        Button(onClick = {
            context.startActivity(Permissions.accessibilitySettingsIntent())
        }) {
            Text("无障碍设置", fontSize = 18.sp)
        }
        Spacer(Modifier.height(10.dp))
        Button(onClick = {
            context.startActivity(Permissions.usageSettingsIntent())
        }) {
            Text("使用情况设置", fontSize = 18.sp)
        }
        Spacer(Modifier.height(20.dp))
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
