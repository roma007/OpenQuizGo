package com.openquizgo.guard.ui

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
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
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.data.remote.WatchGrant
import com.openquizgo.guard.data.watch.WatchRepository
import com.openquizgo.guard.domain.GrantInfo
import com.openquizgo.guard.domain.GuardEngine
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * 阻挡界面（P2-T8）。
 *
 * 答题失败/授权被撤销后展示，拦截返回键；
 * 重新答题直接拉起全屏答题；家长解锁轮询有效授权（远程放行）。
 */
class BlockerActivity : ComponentActivity() {

    private lateinit var engine: GuardEngine
    private lateinit var watchRepository: WatchRepository

    private var screen by mutableStateOf(Screen.MAIN)
    private var parentStatus by mutableStateOf("输入家庭 PIN 直接解锁，或等待家长端放行")
    private var networkError by mutableStateOf(false)
    private var unlockBusy by mutableStateOf(false)
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        engine = (application as GuardApplication).container.guardEngine
        watchRepository = (application as GuardApplication).container.watchRepository

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                GuardLog.i("blocker back ignored")
            }
        })

        GuardLog.i("blocker shown reason=${engine.blockReason.value}")

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    val guardState by engine.state.collectAsState()

                    LaunchedEffect(screen, guardState) {
                        if (screen != Screen.PARENT) return@LaunchedEffect
                        if (guardState == GuardState.GRANTED ||
                            guardState == GuardState.WATCHING ||
                            guardState == GuardState.IDLE
                        ) {
                            GuardLog.i("parent unlock accepted, state=$guardState")
                            finish()
                            return@LaunchedEffect
                        }
                        while (isActive) {
                            val grant = runCatching { watchRepository.activeGrant() }
                            grant.onSuccess { active ->
                                networkError = false
                                if (active != null) {
                                    GuardLog.i(
                                        "parent unlock grant=${active.grantId} " +
                                            "minutes=${active.minutes}",
                                    )
                                    engine.adoptGrant(active.toGrantInfo())
                                    finish()
                                    return@LaunchedEffect
                                }
                                parentStatus = "家长端尚未放行，请稍候…"
                            }.onFailure { error ->
                                networkError = true
                                parentStatus = "网络不可用：${error.message ?: "请检查网络"}"
                            }
                            delay(5_000)
                        }
                    }

                    when (screen) {
                        Screen.MAIN -> BlockerScreen(
                            reason = engine.blockReason.value,
                            onRetryQuiz = { startQuiz() },
                            onParentUnlock = {
                                screen = Screen.PARENT
                                networkError = false
                                parentStatus = "输入家庭 PIN 直接解锁，或等待家长端放行"
                            },
                        )
                        Screen.PARENT -> ParentUnlockScreen(
                            status = parentStatus,
                            networkError = networkError,
                            busy = unlockBusy,
                            onUnlock = { pin ->
                                if (unlockBusy) return@ParentUnlockScreen
                                unlockBusy = true
                                parentStatus = "正在解锁…"
                                scope.launch {
                                    runCatching { watchRepository.deviceUnlock(pin, 10) }
                                        .onSuccess { grant ->
                                            networkError = false
                                            GuardLog.i(
                                                "device pin unlock grant=${grant.grantId} " +
                                                    "minutes=${grant.minutes}",
                                            )
                                            engine.adoptGrant(grant.toGrantInfo())
                                            finish()
                                        }
                                        .onFailure { error ->
                                            networkError = true
                                            parentStatus = "解锁失败：${error.message ?: "请重试"}"
                                        }
                                    unlockBusy = false
                                }
                            },
                            onBack = { screen = Screen.MAIN },
                        )
                    }
                }
            }
        }
    }

    override fun onResume() {
        super.onResume()
        isShowing = true
        val state = engine.state.value
        if (state != GuardState.BLOCKED && state != GuardState.QUIZZING) {
            GuardLog.i("blocker dismissed, state=$state")
            finish()
        }
    }

    override fun onPause() {
        isShowing = false
        super.onPause()
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    private fun startQuiz() {
        engine.forceQuizRequired()
        val intent = Intent(this, QuizActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        startActivity(intent)
        GuardLog.i("blocker -> restart quiz")
        finish()
    }

    private enum class Screen { MAIN, PARENT }

    companion object {
        @Volatile
        var isShowing: Boolean = false
            private set
    }
}

private fun WatchGrant.toGrantInfo(): GrantInfo = GrantInfo(
    grantId = grantId,
    minutes = minutes,
    remainingSeconds = remainingSeconds,
    validUntilMillis = GuardEngine.parseMillis(validUntil)
        ?: (System.currentTimeMillis() + minutes * 60_000L),
)

@Composable
private fun BlockerScreen(
    reason: String?,
    onRetryQuiz: () -> Unit,
    onParentUnlock: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF12060A))
            .padding(72.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "休息一下吧",
            fontSize = 56.sp,
            fontWeight = FontWeight.Bold,
            color = Color(0xFFFF7A7A),
        )
        Spacer(Modifier.height(20.dp))
        Text(
            text = reasonText(reason),
            fontSize = 28.sp,
            color = Color(0xFFE0E0E0),
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = "完成答题可以获得观看时间，或请家长解锁",
            fontSize = 22.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(48.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(28.dp)) {
            Button(onClick = onRetryQuiz, modifier = Modifier.widthIn(min = 220.dp)) {
                Text("重新答题", fontSize = 28.sp)
            }
            Button(onClick = onParentUnlock, modifier = Modifier.widthIn(min = 220.dp)) {
                Text("家长解锁", fontSize = 28.sp)
            }
        }
    }
}

@Composable
private fun ParentUnlockScreen(
    status: String,
    networkError: Boolean,
    busy: Boolean,
    onUnlock: (String) -> Unit,
    onBack: () -> Unit,
) {
    var pin by remember { mutableStateOf("") }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(48.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "家长解锁",
            fontSize = 42.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        Spacer(Modifier.height(10.dp))
        Text(
            text = "输入家庭 PIN 直接解锁 10 分钟，或在家长端选择「临时解锁」",
            fontSize = 20.sp,
            color = Color(0xFFD0D0D0),
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = if (pin.isEmpty()) "○ ○ ○ ○" else "●".repeat(pin.length),
            fontSize = 32.sp,
            color = Color(0xFF7CC7FF),
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = status,
            fontSize = 18.sp,
            color = if (networkError) Color(0xFFFFC107) else Color(0xFF7CC7FF),
        )
        Spacer(Modifier.height(16.dp))
        for (row in listOf(listOf("1", "2", "3"), listOf("4", "5", "6"), listOf("7", "8", "9"))) {
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                row.forEach { digit ->
                    Button(
                        onClick = { if (pin.length < 6) pin += digit },
                        modifier = Modifier.widthIn(min = 72.dp),
                    ) { Text(digit, fontSize = 24.sp) }
                }
            }
            Spacer(Modifier.height(8.dp))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(onClick = { pin = "" }, modifier = Modifier.widthIn(min = 72.dp)) {
                Text("清除", fontSize = 18.sp)
            }
            Button(
                onClick = { if (pin.length < 6) pin += "0" },
                modifier = Modifier.widthIn(min = 72.dp),
            ) { Text("0", fontSize = 24.sp) }
            Button(
                onClick = { if (pin.isNotEmpty()) pin = pin.dropLast(1) },
                modifier = Modifier.widthIn(min = 72.dp),
            ) { Text("删除", fontSize = 18.sp) }
        }
        Spacer(Modifier.height(16.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(16.dp)) {
            Button(
                onClick = { onUnlock(pin) },
                enabled = pin.length in 4..6 && !busy,
                modifier = Modifier.widthIn(min = 240.dp),
            ) {
                Text(if (busy) "解锁中…" else "解锁 10 分钟", fontSize = 22.sp)
            }
            Button(onClick = onBack, modifier = Modifier.widthIn(min = 160.dp)) {
                Text("返回", fontSize = 22.sp)
            }
        }
    }
}

private fun reasonText(reason: String?): String = when (reason) {
    "quiz_failed" -> "本次答题未通过"
    "time_window" -> "当前不在允许观看的时段"
    "revoked" -> "观看授权已失效"
    else -> "当前不能继续观看"
}
