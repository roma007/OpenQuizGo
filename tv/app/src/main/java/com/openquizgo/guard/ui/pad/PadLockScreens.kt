package com.openquizgo.guard.ui.pad

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 084：平板拦截页 / 家长解锁页 / 硬停页。
 *
 * 逻辑（轮询家长放行、PIN 解锁）仍在 Activity 里，这里只负责触摸界面。
 */

@Composable
internal fun PadBlockerScreen(
    reason: String?,
    onRetryQuiz: () -> Unit,
    onParentUnlock: () -> Unit,
) {
    val quizUseful = reason != "daily_limit" && reason != "time_window"
    val hint = when {
        reason == "daily_limit" -> "今天的额度已经用完，明天再来；也可以请家长解锁"
        reason == "time_window" -> "现在不在允许观看的时段，换个时间再来；也可以请家长解锁"
        else -> "完成答题可以获得观看时间，或请家长解锁"
    }
    PadPage {
        Spacer(Modifier.height(32.dp))
        Text(
            text = "休息一下吧",
            fontSize = 30.sp,
            fontWeight = FontWeight.Bold,
            color = PadBad,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = padReasonLabel(reason).ifBlank { "当前不能继续观看" },
            fontSize = 16.sp,
            color = PadTextPrimary,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        Text(
            text = hint,
            fontSize = 13.sp,
            color = PadTextSecondary,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(24.dp))
        if (quizUseful) {
            PadPrimaryButton(
                text = "重新答题",
                onClick = onRetryQuiz,
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(10.dp))
        }
        PadSecondaryButton(
            text = "家长解锁",
            onClick = onParentUnlock,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}

@Composable
internal fun PadParentUnlockScreen(
    pin: String,
    onPinChange: (String) -> Unit,
    status: String,
    networkError: Boolean,
    busy: Boolean,
    onUnlock: () -> Unit,
    onBack: () -> Unit,
) {
    PadPage {
        Spacer(Modifier.height(24.dp))
        PadTitle("家长解锁")
        Spacer(Modifier.height(8.dp))
        PadSubtitle("输入 4–6 位家长 PIN 解锁 10 分钟，或在家长端点「临时放行」")
        Spacer(Modifier.height(20.dp))
        OutlinedTextField(
            value = pin,
            onValueChange = { value ->
                onPinChange(value.filter { it.isDigit() }.take(6))
            },
            label = { Text("家长 PIN") },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(10.dp))
        Text(
            text = status,
            fontSize = 13.sp,
            color = if (networkError) PadWarn else PadAccent,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(16.dp))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            PadPrimaryButton(
                text = if (busy) "解锁中…" else "解锁 10 分钟",
                onClick = onUnlock,
                enabled = pin.length in 4..6 && !busy,
                modifier = Modifier.weight(1f),
            )
            PadSecondaryButton(text = "返回", onClick = onBack)
        }
    }
}

@Composable
internal fun PadHardStopScreen(
    graceMinutes: Int,
    remainingMinutes: Int?,
    dailyLimit: Int?,
    pin: String,
    onPinChange: (String) -> Unit,
    status: String,
    failed: Boolean,
    busy: Boolean,
    onUnlock: () -> Unit,
) {
    PadPage {
        Spacer(Modifier.height(32.dp))
        Text(
            text = "今天的观看时间用完了",
            fontSize = 26.sp,
            fontWeight = FontWeight.Bold,
            color = PadBad,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(10.dp))
        Text(
            text = if (dailyLimit != null) {
                "已用满 $dailyLimit 分钟（剩余 ${remainingMinutes ?: 0} 分钟），明天再来"
            } else {
                "明天再来，或请家长输入 PIN 临时放行"
            },
            fontSize = 14.sp,
            color = PadTextPrimary,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(20.dp))
        OutlinedTextField(
            value = pin,
            onValueChange = { value ->
                onPinChange(value.filter { it.isDigit() }.take(6))
            },
            label = { Text("家长 PIN") },
            singleLine = true,
            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(8.dp))
        Text(
            text = status,
            fontSize = 13.sp,
            color = if (failed) PadBad else PadAccent,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(14.dp))
        PadPrimaryButton(
            text = if (busy) "解锁中…" else "放行 $graceMinutes 分钟",
            onClick = onUnlock,
            enabled = pin.length in 4..6 && !busy,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}
