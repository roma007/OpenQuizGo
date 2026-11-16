package com.openquizgo.guard.ui.pad

import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.platform.GuardLog
import com.openquizgo.guard.platform.GuardService
import com.openquizgo.guard.ui.CheckItem
import com.openquizgo.guard.ui.buildSelfCheckItems
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first

/**
 * 084：平板开机自检页（触摸）。
 *
 * 与电视端共用 [buildSelfCheckItems]，只是把竖排项目改成适合手指点的卡片；
 * 全部通过后自动关闭，未通过的项每 1.5 秒刷新一次。
 */
@Composable
internal fun PadSelfCheckScreen(onAllPassed: () -> Unit, onSkip: () -> Unit) {
    val context = LocalContext.current
    val prefs = (context.applicationContext as GuardApplication).container.preferences

    var items by remember { mutableStateOf<List<CheckItem>>(emptyList()) }

    LaunchedEffect(Unit) {
        GuardService.start(context)
        while (true) {
            val loggedIn = prefs.deviceToken.first()?.isNotBlank() == true
            val current = buildSelfCheckItems(context, loggedIn)
            items = current
            if (current.all { it.ok }) {
                GuardLog.i("selfcheck", "all passed, dismiss (pad)")
                onAllPassed()
                return@LaunchedEffect
            }
            delay(1_500)
        }
    }

    val failed = items.filter { !it.ok }

    PadPage {
        Spacer(Modifier.height(24.dp))
        PadTitle(
            if (failed.isEmpty()) "正在检查…" else "还差 ${failed.size} 项设置",
        )
        Spacer(Modifier.height(8.dp))
        PadSubtitle("设置完成后会自动继续，下次开机不再显示")
        Spacer(Modifier.height(24.dp))

        failed.forEach { item ->
            PadCard {
                Text(
                    text = "✗ ${item.label}",
                    fontSize = 17.sp,
                    fontWeight = FontWeight.Bold,
                    color = PadWarn,
                )
                Spacer(Modifier.height(10.dp))
                PadPrimaryButton(
                    text = item.action,
                    onClick = { item.run(context) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Spacer(Modifier.height(12.dp))
        }

        if (failed.isEmpty()) {
            Text(
                text = "正在确认权限状态…",
                fontSize = 13.sp,
                color = PadTextSecondary,
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth(),
            )
            Spacer(Modifier.height(12.dp))
        }

        Spacer(Modifier.height(8.dp))
        PadSecondaryButton(
            text = "稍后再说",
            onClick = onSkip,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}
