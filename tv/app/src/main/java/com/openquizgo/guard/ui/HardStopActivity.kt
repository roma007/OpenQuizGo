package com.openquizgo.guard.ui

import android.os.Bundle
import android.view.KeyEvent
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
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
import androidx.compose.foundation.layout.widthIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
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
import com.openquizgo.guard.data.watch.WatchRepository
import com.openquizgo.guard.domain.GuardEngine
import com.openquizgo.guard.platform.FormFactors
import com.openquizgo.guard.platform.GuardLog
import com.openquizgo.guard.ui.pad.PadHardStopScreen
import com.openquizgo.guard.ui.pad.PadTheme
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * 硬停页（081 T10）。
 *
 * 每日观看上限到点后全屏展示：拦截返回键，被切走后由 GuardService 1 秒内重新拉起；
 * 只有家长 PIN 才能按配置时长放行。
 */
class HardStopActivity : ComponentActivity() {

    private lateinit var engine: GuardEngine
    private lateinit var watchRepository: WatchRepository

    private var busy by mutableStateOf(false)
    private var status by mutableStateOf("请输入家长 PIN 解除限制")
    private var failed by mutableStateOf(false)
    private var pin by mutableStateOf("")
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        engine = (application as GuardApplication).container.guardEngine
        watchRepository = (application as GuardApplication).container.watchRepository

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                GuardLog.i("hardstop", "back ignored")
            }
        })

        GuardLog.i("hardstop", "shown")

        // 家长端「临时解锁」签发的授权在这里轮询生效，不用等下一次上下文刷新。
        scope.launch {
            while (true) {
                delay(5_000)
                if (!isShowing || !engine.hardStop.value) continue
                runCatching { watchRepository.activeGrant() }
                    .onSuccess { grant ->
                        if (grant == null) return@onSuccess
                        GuardLog.i("hardstop", "parent grant=${grant.grantId} minutes=${grant.minutes}")
                        engine.adoptGrant(
                            com.openquizgo.guard.domain.GrantInfo(
                                grantId = grant.grantId,
                                minutes = grant.minutes,
                                remainingSeconds = grant.remainingSeconds,
                                validUntilMillis = GuardEngine.parseMillis(grant.validUntil)
                                    ?: (System.currentTimeMillis() + grant.minutes * 60_000L),
                            ),
                        )
                        finish()
                    }
            }
        }

        val television = FormFactors.isTelevision(this)
        setContent {
            val hardStop by engine.hardStop.collectAsState()
            val grace by engine.hardStopGrace.collectAsState()
            val remaining by engine.contextRemainingMinutes.collectAsState()
            val limit by engine.contextDailyLimit.collectAsState()

            LaunchedEffect(hardStop) {
                // 家长端放行或跨天重置后，硬停解除，页面必须自动退出。
                if (!hardStop) {
                    GuardLog.i("hardstop", "hard stop lifted, dismiss")
                    finish()
                }
            }

            if (television) {
                MaterialTheme(colorScheme = darkColorScheme()) {
                    Surface(modifier = Modifier.fillMaxSize()) {
                        HardStopScreen(
                            graceMinutes = grace,
                            remainingMinutes = remaining,
                            dailyLimit = limit,
                            status = status,
                            failed = failed,
                            busy = busy,
                            pin = pin,
                            onPinChange = { pin = it },
                            onUnlock = { value -> unlockWithPin(value, grace) },
                        )
                    }
                }
            } else {
                PadTheme {
                    PadHardStopScreen(
                        graceMinutes = grace,
                        remainingMinutes = remaining,
                        dailyLimit = limit,
                        pin = pin,
                        onPinChange = { pin = it },
                        status = status,
                        failed = failed,
                        busy = busy,
                        onUnlock = { unlockWithPin(pin, grace) },
                    )
                }
            }
        }
    }

    private fun unlockWithPin(value: String, grace: Int) {
        if (busy) return
        busy = true
        failed = false
        status = "正在解锁…"
        scope.launch {
            val minutes = grace.coerceIn(1, 120)
            runCatching { watchRepository.deviceUnlock(value, minutes) }
                .onSuccess { grant ->
                    GuardLog.i("hardstop", "pin unlock grant=${grant.grantId} minutes=${grant.minutes}")
                    engine.adoptGrant(
                        com.openquizgo.guard.domain.GrantInfo(
                            grantId = grant.grantId,
                            minutes = grant.minutes,
                            remainingSeconds = grant.remainingSeconds,
                            validUntilMillis = GuardEngine.parseMillis(grant.validUntil)
                                ?: (System.currentTimeMillis() + minutes * 60_000L),
                        ),
                    )
                    finish()
                }
                .onFailure { error ->
                    failed = true
                    status = "解锁失败：${error.message ?: "请重试"}"
                }
            busy = false
        }
    }

    /** 遥控器数字键直接输入 PIN，避免只能靠方向键逐个选数字；平板放行给输入框。 */
    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        if (!FormFactors.isTelevision(this)) return super.dispatchKeyEvent(event)
        if (event.action == KeyEvent.ACTION_DOWN) {
            val digit = digitOf(event.keyCode)
            if (digit != null) {
                if (pin.length < 6) pin += digit
                return true
            }
            if (event.keyCode == KeyEvent.KEYCODE_DEL) {
                if (pin.isNotEmpty()) pin = pin.dropLast(1)
                return true
            }
        }
        return super.dispatchKeyEvent(event)
    }

    private fun digitOf(keyCode: Int): String? = when (keyCode) {
        in KeyEvent.KEYCODE_0..KeyEvent.KEYCODE_9 ->
            (keyCode - KeyEvent.KEYCODE_0).toString()
        in KeyEvent.KEYCODE_NUMPAD_0..KeyEvent.KEYCODE_NUMPAD_9 ->
            (keyCode - KeyEvent.KEYCODE_NUMPAD_0).toString()
        else -> null
    }

    override fun onResume() {
        super.onResume()
        isShowing = true
        if (!engine.hardStop.value) finish()
    }

    override fun onPause() {
        isShowing = false
        super.onPause()
    }

    override fun onDestroy() {
        scope.cancel()
        super.onDestroy()
    }

    companion object {
        @Volatile
        var isShowing: Boolean = false
            private set
    }
}

@Composable
private fun HardStopScreen(
    graceMinutes: Int,
    remainingMinutes: Int?,
    dailyLimit: Int?,
    status: String,
    failed: Boolean,
    busy: Boolean,
    pin: String,
    onPinChange: (String) -> Unit,
    onUnlock: (String) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF12060A))
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 48.dp, vertical = 28.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "今天的观看时间用完了",
            fontSize = 38.sp,
            fontWeight = FontWeight.Bold,
            color = Color(0xFFFF7A7A),
        )
        Spacer(Modifier.height(8.dp))
        Text(
            text = if (dailyLimit != null) {
                "已用满 $dailyLimit 分钟（剩余 ${remainingMinutes ?: 0} 分钟），明天再来"
            } else {
                "明天再来，或请家长输入 PIN 临时放行"
            },
            fontSize = 20.sp,
            color = Color(0xFFE0E0E0),
        )
        Spacer(Modifier.height(18.dp))
        Text(text = "家长 PIN", fontSize = 18.sp, color = Color(0xFFB0B0B0))
        Spacer(Modifier.height(4.dp))
        Text(
            text = if (pin.isEmpty()) "○ ○ ○ ○" else "●".repeat(pin.length),
            fontSize = 30.sp,
            color = Color(0xFF7CC7FF),
        )
        Spacer(Modifier.height(6.dp))
        Text(
            text = status,
            fontSize = 15.sp,
            color = if (failed) Color(0xFFFF7A7A) else Color(0xFF7CC7FF),
        )
        Spacer(Modifier.height(10.dp))
        Button(
            onClick = { onUnlock(pin) },
            enabled = pin.length in 4..6 && !busy,
            modifier = Modifier.widthIn(min = 300.dp),
        ) {
            Text(
                text = if (busy) "解锁中…" else "放行 $graceMinutes 分钟",
                fontSize = 20.sp,
            )
        }
        Spacer(Modifier.height(10.dp))
        for (row in listOf(listOf("1", "2", "3"), listOf("4", "5", "6"), listOf("7", "8", "9"))) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                row.forEach { digit ->
                    Button(
                        onClick = { if (pin.length < 6) onPinChange(pin + digit) },
                        modifier = Modifier.widthIn(min = 66.dp),
                    ) { Text(digit, fontSize = 22.sp) }
                }
            }
            Spacer(Modifier.height(4.dp))
        }
        Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(
                onClick = { onPinChange("") },
                modifier = Modifier.widthIn(min = 66.dp),
            ) { Text("清除", fontSize = 15.sp) }
            Button(
                onClick = { if (pin.length < 6) onPinChange(pin + "0") },
                modifier = Modifier.widthIn(min = 66.dp),
            ) { Text("0", fontSize = 22.sp) }
            Button(
                onClick = { if (pin.isNotEmpty()) onPinChange(pin.dropLast(1)) },
                modifier = Modifier.widthIn(min = 66.dp),
            ) { Text("删除", fontSize = 15.sp) }
        }
    }
}
