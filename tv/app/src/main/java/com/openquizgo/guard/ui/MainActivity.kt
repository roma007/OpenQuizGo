package com.openquizgo.guard.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
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
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
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
import com.openquizgo.guard.domain.LocalGuardState
import com.openquizgo.guard.platform.GuardService
import com.openquizgo.guard.platform.Permissions
import kotlinx.coroutines.delay

class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = (application as GuardApplication).container
        val viewModel = ViewModelProvider(this, LocalViewModel.Factory(container))[LocalViewModel::class.java]

        GuardService.start(this)

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    LocalGuardScreen(viewModel)
                }
            }
        }
    }
}

@Composable
private fun LocalGuardScreen(viewModel: LocalViewModel) {
    val context = LocalContext.current
    val enabled by viewModel.enabled.collectAsState()
    val state by viewModel.state.collectAsState()
    val remaining by viewModel.remainingSeconds.collectAsState()
    val elapsed by viewModel.elapsedSeconds.collectAsState()
    var accessibility by remember { mutableStateOf(Permissions.accessibilityEnabled(context)) }
    val toggleFocus = remember { FocusRequester() }

    LaunchedEffect(Unit) {
        while (true) {
            accessibility = Permissions.accessibilityEnabled(context)
            delay(2_000)
        }
    }
    LaunchedEffect(Unit) {
        runCatching { toggleFocus.requestFocus() }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 72.dp, vertical = 24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "电视时间管家",
            fontSize = 40.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        Spacer(Modifier.height(6.dp))
        Text(
            text = "每观看 15 分钟，答对 1 道题后继续",
            fontSize = 18.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(22.dp))

        Button(
            onClick = viewModel::toggleQuiz,
            modifier = Modifier.focusRequester(toggleFocus),
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                Text(
                    text = "定时答题",
                    fontSize = 26.sp,
                    fontWeight = FontWeight.Bold,
                )
                Text(
                    text = if (enabled) "已开启" else "已关闭",
                    fontSize = 22.sp,
                    color = if (enabled) Color(0xFF7CFF9B) else Color(0xFFFFC107),
                )
            }
        }
        Spacer(Modifier.height(8.dp))
        Text(
            text = if (enabled) "按确认键切换开关" else "按确认键重新开启",
            fontSize = 14.sp,
            color = Color(0xFF707070),
        )

        Spacer(Modifier.height(20.dp))
        StatusPanel(
            enabled = enabled,
            state = state,
            remainingSeconds = remaining,
            elapsedSeconds = elapsed,
        )

        Spacer(Modifier.height(18.dp))
        Text(
            text = "无障碍服务：${if (accessibility) "已开启" else "未开启"}",
            fontSize = 18.sp,
            color = if (accessibility) Color(0xFF7CFF9B) else Color(0xFFFF7A7A),
        )
        if (!accessibility) {
            Spacer(Modifier.height(6.dp))
            Text(
                text = "需要开启无障碍服务，才能在到点时自动弹出答题",
                fontSize = 14.sp,
                color = Color(0xFFB0B0B0),
            )
            Spacer(Modifier.height(8.dp))
            Button(onClick = {
                context.startActivity(Permissions.accessibilitySettingsIntent())
            }) {
                Text("打开无障碍设置", fontSize = 18.sp)
            }
        }

        Spacer(Modifier.height(20.dp))
        Text(
            text = "题库：${viewModel.questionCount} 道 · 本地离线 · v${BuildConfig.VERSION_NAME}",
            fontSize = 13.sp,
            color = Color(0xFF606060),
        )
    }
}

@Composable
private fun StatusPanel(
    enabled: Boolean,
    state: LocalGuardState,
    remainingSeconds: Long,
    elapsedSeconds: Long,
) {
    val status = when {
        !enabled -> "已关闭，不会弹出答题"
        state == LocalGuardState.QUIZ_REQUIRED || state == LocalGuardState.QUIZZING ->
            "答题时间到，答对后继续"
        state == LocalGuardState.WATCHING ->
            "正在计时 · 距下次答题 ${formatDuration(remainingSeconds)}"
        state == LocalGuardState.GRANTED ->
            "回答正确 · 距下次答题 ${formatDuration(remainingSeconds)}"
        else ->
            "等待视频播放 · 距下次答题 ${formatDuration(remainingSeconds)}"
    }
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            text = status,
            fontSize = 20.sp,
            color = when {
                !enabled -> Color(0xFFFFC107)
                state == LocalGuardState.WATCHING -> Color(0xFF7CFF9B)
                state == LocalGuardState.QUIZ_REQUIRED ||
                    state == LocalGuardState.QUIZZING -> Color(0xFFFF7A7A)
                else -> Color(0xFF7CC7FF)
            },
        )
        Spacer(Modifier.height(4.dp))
        Text(
            text = "本次已累计观看 ${formatDuration(elapsedSeconds)}",
            fontSize = 14.sp,
            color = Color(0xFF808080),
        )
    }
}

private fun formatDuration(seconds: Long): String {
    val safe = seconds.coerceAtLeast(0)
    val minutes = safe / 60
    val rest = safe % 60
    return when {
        minutes <= 0 -> "${rest} 秒"
        rest == 0L -> "${minutes} 分钟"
        else -> "${minutes} 分 ${rest} 秒"
    }
}
