package com.openquizgo.guard.ui

import android.content.Context
import android.Manifest
import android.app.Activity
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.provider.Settings
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
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
import com.openquizgo.guard.platform.FormFactors
import com.openquizgo.guard.platform.GuardService
import com.openquizgo.guard.platform.GuardLog
import com.openquizgo.guard.platform.Permissions
import com.openquizgo.guard.ui.pad.PadSelfCheckScreen
import com.openquizgo.guard.ui.pad.PadTheme
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking

/**
 * 开机自检（081 T8）。
 *
 * 每次开机检查登录与守护所需权限；全部通过立即结束、不显示任何界面，
 * 未通过时只列出未通过的项并给出对应跳转。
 */
class SelfCheckActivity : ComponentActivity() {

    private val notificationPermission =
        registerForActivityResult(ActivityResultContracts.RequestPermission()) { }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        requestNotificationPermissionIfNeeded()
        if (allPassed()) {
            GuardLog.i("selfcheck", "all passed, skip ui")
            finish()
            return
        }
        GuardLog.i("selfcheck", "some checks failed, show guide")
        val television = FormFactors.isTelevision(this)
        setContent {
            if (television) {
                MaterialTheme(colorScheme = darkColorScheme()) {
                    Surface(modifier = Modifier.fillMaxSize()) {
                        SelfCheckScreen(
                            onAllPassed = { finish() },
                            onSkip = { finish() },
                        )
                    }
                }
            } else {
                PadTheme {
                    PadSelfCheckScreen(
                        onAllPassed = { finish() },
                        onSkip = { finish() },
                    )
                }
            }
        }
    }

    /** Android 13+ 需要运行时授权，否则「通知权限」自检永远不过。 */
    private fun requestNotificationPermissionIfNeeded() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return
        if (Permissions.notificationsEnabled(this)) return
        if (checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) ==
            PackageManager.PERMISSION_GRANTED
        ) {
            return
        }
        runCatching { notificationPermission.launch(Manifest.permission.POST_NOTIFICATIONS) }
    }

    /**
     * 电视没有通知栏，Android TV 上 areNotificationsEnabled 常驻 false，
     * 这一项不作为电视的阻断项（手机/平板仍按真实权限判断）。
     */
    private fun notificationsOk(): Boolean =
        Permissions.notificationsEnabled(this) ||
            packageManager.hasSystemFeature(PackageManager.FEATURE_LEANBACK)

    private fun allPassed(): Boolean {
        val loggedIn = runCatching {
            runBlocking {
                !(application as GuardApplication).container.preferences.deviceToken.first().isNullOrBlank()
            }
        }.getOrDefault(false)
        return loggedIn &&
            Permissions.accessibilityEnabled(this) &&
            Permissions.usageAccessEnabled(this) &&
            notificationsOk() &&
            Permissions.foregroundServiceRunning(this)
    }

    companion object {
        /** 开机或打开 App 时调用；只有存在未通过项才真正弹出界面。 */
        fun maybeLaunch(context: Context) {
            val intent = Intent(context, SelfCheckActivity::class.java)
                .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
            runCatching { context.startActivity(intent) }
                .onFailure { GuardLog.w("selfcheck", "launch failed: ${it.message}") }
        }
    }
}

internal data class CheckItem(
    val label: String,
    val ok: Boolean,
    val action: String,
    val run: (Context) -> Unit,
)

@Composable
private fun SelfCheckScreen(onAllPassed: () -> Unit, onSkip: () -> Unit) {
    val context = androidx.compose.ui.platform.LocalContext.current
    val prefs = (context.applicationContext as GuardApplication).container.preferences

    var items by remember { mutableStateOf<List<CheckItem>>(emptyList()) }
    var loggedIn by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        GuardService.start(context)
        while (true) {
            loggedIn = prefs.deviceToken.first()?.isNotBlank() == true
            items = buildSelfCheckItems(context, loggedIn)
            if (items.all { it.ok }) {
                GuardLog.i("selfcheck", "all passed, dismiss")
                onAllPassed()
                return@LaunchedEffect
            }
            delay(1_500)
        }
    }

    val failed = items.filter { !it.ok }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(56.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = if (failed.isEmpty()) "正在检查…" else "还差 ${failed.size} 项设置",
            fontSize = 40.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        Spacer(Modifier.height(8.dp))
        Text(
            text = "设置完成后会自动继续，下次开机不再显示",
            fontSize = 18.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(28.dp))
        failed.forEach { item ->
            Row(
                horizontalArrangement = Arrangement.spacedBy(20.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(text = "✗ ${item.label}", fontSize = 24.sp, color = Color(0xFFFFC107))
                Button(
                    onClick = { item.run(context) },
                    modifier = Modifier.widthIn(min = 180.dp),
                ) {
                    Text(item.action, fontSize = 18.sp)
                }
            }
            Spacer(Modifier.height(12.dp))
        }
        Spacer(Modifier.height(20.dp))
        Button(onClick = onSkip, modifier = Modifier.widthIn(min = 200.dp)) {
            Text("稍后再说", fontSize = 18.sp)
        }
    }
}

/**
 * 084：电视与平板共用一份自检项，界面各自实现。
 */
internal fun buildSelfCheckItems(context: Context, loggedIn: Boolean): List<CheckItem> = listOf(
    CheckItem(
        label = "登录设备",
        ok = loggedIn,
        action = if (loggedIn) "" else "去登录",
        run = { ctx ->
            ctx.startActivity(
                Intent(ctx, MainActivity::class.java)
                    .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
            )
            // 自检页在最上层，去登录前必须先退出，否则登录页被盖住。
            (ctx as? Activity)?.finish()
        },
    ),
    CheckItem(
        label = "无障碍服务",
        ok = Permissions.accessibilityEnabled(context),
        action = "去开启",
        run = { ctx -> ctx.startActivity(Permissions.accessibilitySettingsIntent()) },
    ),
    CheckItem(
        label = "使用情况访问",
        ok = Permissions.usageAccessEnabled(context),
        action = "去开启",
        run = { ctx -> ctx.startActivity(Permissions.usageSettingsIntent()) },
    ),
    CheckItem(
        label = "通知权限",
        ok = Permissions.notificationsEnabled(context) ||
            context.packageManager.hasSystemFeature(PackageManager.FEATURE_LEANBACK),
        action = "去开启",
        run = { ctx -> ctx.startActivity(notificationSettingsIntent(ctx)) },
    ),
    CheckItem(
        label = "守护服务",
        ok = Permissions.foregroundServiceRunning(context),
        action = "重新启动",
        run = { ctx -> GuardService.start(ctx) },
    ),
)

private fun notificationSettingsIntent(context: Context): Intent =
    if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
        Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS)
            .putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
    } else {
        Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS)
            .setData(android.net.Uri.parse("package:${context.packageName}"))
    }
