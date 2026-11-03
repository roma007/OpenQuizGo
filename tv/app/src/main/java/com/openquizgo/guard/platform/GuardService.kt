package com.openquizgo.guard.platform

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.widget.Toast
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.data.foreground.ForegroundState
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.ui.BlockerActivity
import com.openquizgo.guard.ui.MainActivity
import com.openquizgo.guard.ui.QuizActivity
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

class GuardService : Service() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private val mainHandler = Handler(Looper.getMainLooper())
    private var stateJob: Job? = null
    private var promptJob: Job? = null
    private var watchdogJob: Job? = null
    private var engine: com.openquizgo.guard.domain.GuardEngine? = null
    private var quizRequiredSince = 0L
    private var lastGuardLaunchAt = 0L

    override fun onCreate() {
        super.onCreate()
        createChannel()
        startForeground(NOTIFICATION_ID, buildNotification("守护端启动中"))
        engine = (application as GuardApplication).container.guardEngine
        stateJob = scope.launch {
            engine?.state?.collect { state ->
                updateNotification(label(state))
                when (state) {
                    GuardState.QUIZ_REQUIRED -> {
                        if (quizRequiredSince == 0L) quizRequiredSince = System.currentTimeMillis()
                        startPromptLoop()
                    }
                    GuardState.BLOCKED -> {
                        quizRequiredSince = 0L
                        promptJob?.cancel()
                        GuardAccessibilityService.disarmQuiz()
                        launchBlocker()
                    }
                    GuardState.GRANTED -> {
                        quizRequiredSince = 0L
                        promptJob?.cancel()
                        GuardAccessibilityService.disarmQuiz()
                        toast("按确认键继续观看")
                    }
                    else -> {
                        quizRequiredSince = 0L
                        promptJob?.cancel()
                        GuardAccessibilityService.disarmQuiz()
                    }
                }
            }
        }
        watchdogJob = scope.launch {
            while (isActive) {
                delay(WATCHDOG_INTERVAL_MS)
                watchdogTick()
            }
        }
    }

    private fun watchdogTick() {
        val current = engine?.state?.value ?: return
        val foreground = (application as GuardApplication).container.foregroundDetector.state.value
        when (current) {
            GuardState.BLOCKED -> {
                if (blockerEscaped(foreground) && !BlockerActivity.isShowing &&
                    nowSinceLaunch() > GUARD_RELAUNCH_GAP_MS
                ) {
                    GuardLog.w("watchdog: blocker missing pkg=${foreground.packageName}")
                    launchBlocker()
                }
            }
            GuardState.QUIZ_REQUIRED -> {
                if (!Permissions.accessibilityEnabled(this) ||
                    !GuardAccessibilityService.isConnected()
                ) {
                    return
                }
                GuardAccessibilityService.armQuiz()
                val waited = System.currentTimeMillis() - quizRequiredSince
                if (waited > QUIZ_FALLBACK_MS && guardTarget(foreground) &&
                    nowSinceLaunch() > GUARD_RELAUNCH_GAP_MS
                ) {
                    GuardLog.w("watchdog: quiz fallback after ${waited}ms")
                    launchQuiz()
                }
            }
            else -> Unit
        }
    }

    /** 阻挡态：除家长需要的系统设置外，任何前台都重新拉起阻挡页。 */
    private fun blockerEscaped(foreground: ForegroundState): Boolean {
        val pkg = foreground.packageName ?: return false
        if (pkg == packageName) return false
        return pkg !in ALLOWED_WHILE_BLOCKED
    }

    private fun guardTarget(foreground: ForegroundState): Boolean {
        val pkg = foreground.packageName ?: return false
        if (pkg == packageName) return false
        if (pkg in IGNORED_PACKAGES) return false
        return foreground.mediaActive || pkg in MEDIA_PACKAGES
    }

    private fun nowSinceLaunch(): Long = System.currentTimeMillis() - lastGuardLaunchAt

    private fun launchBlocker() {
        lastGuardLaunchAt = System.currentTimeMillis()
        val intent = Intent(this, BlockerActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        runCatching { startActivity(intent) }
            .onFailure { GuardLog.e("blocker launch failed", it) }
    }

    private fun launchQuiz() {
        lastGuardLaunchAt = System.currentTimeMillis()
        val intent = Intent(this, QuizActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        runCatching { startActivity(intent) }
            .onFailure { GuardLog.e("quiz launch failed", it) }
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        return START_STICKY
    }

    override fun onDestroy() {
        stateJob?.cancel()
        watchdogJob?.cancel()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun startPromptLoop() {
        promptJob?.cancel()
        promptJob = scope.launch {
            var warned = false
            while (isActive) {
                if (Permissions.accessibilityEnabled(this@GuardService) &&
                    GuardAccessibilityService.isConnected()
                ) {
                    toast("按确认键开始答题")
                    GuardAccessibilityService.armQuiz()
                    GuardLog.i("quiz required, armed for confirm key")
                    return@launch
                }
                if (!warned) {
                    toast("请开启无障碍服务以开始答题")
                    GuardLog.w("quiz required but accessibility not ready")
                    warned = true
                }
                delay(2_000)
            }
        }
    }

    private fun toast(message: String) {
        mainHandler.post {
            Toast.makeText(this, message, Toast.LENGTH_LONG).show()
        }
    }

    private fun createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        val channel = NotificationChannel(
            CHANNEL_ID,
            "守护服务",
            NotificationManager.IMPORTANCE_LOW,
        )
        manager.createNotificationChannel(channel)
    }

    private fun buildNotification(text: String): Notification {
        val pending = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java),
            PendingIntent.FLAG_IMMUTABLE,
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            Notification.Builder(this)
        }
        return builder
            .setSmallIcon(android.R.drawable.ic_lock_lock)
            .setContentTitle("OpenQuizGo 守护中")
            .setContentText(text)
            .setContentIntent(pending)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(NOTIFICATION_ID, buildNotification(text))
    }

    private fun label(state: GuardState): String = when (state) {
        GuardState.UNBOUND -> "设备未激活"
        GuardState.IDLE -> "空闲"
        GuardState.WATCHING -> "观看中"
        GuardState.QUIZ_REQUIRED -> "需要答题"
        GuardState.QUIZZING -> "答题中"
        GuardState.GRANTED -> "已授权"
        GuardState.BLOCKED -> "已锁定"
    }

    companion object {
        private const val CHANNEL_ID = "guard_service"
        private const val NOTIFICATION_ID = 1001
        private const val WATCHDOG_INTERVAL_MS = 1_500L
        private const val GUARD_RELAUNCH_GAP_MS = 2_000L
        private const val QUIZ_FALLBACK_MS = 90_000L

        private val MEDIA_PACKAGES = setOf(
            "com.tianci.movieplatform",
            "com.tianci.localmedia",
            "com.tianci.videoplayer",
            "com.qiyi.video",
            "com.tencent.qqlive",
            "com.youku.tv",
            "tv.danmaku.bili",
        )

        private val IGNORED_PACKAGES = setOf(
            "com.tianci.system",
            "com.android.settings",
            "com.android.systemui",
            "com.android.tv.settings",
            "com.skyworth.setting",
            "com.skyworth.settings",
            "com.tianci.launcher",
        )

        private val ALLOWED_WHILE_BLOCKED = setOf(
            "com.android.settings",
            "com.android.systemui",
            "com.android.tv.settings",
            "com.skyworth.setting",
            "com.skyworth.settings",
            "com.tianci.system",
        )

        fun start(context: Context) {
            val intent = Intent(context, GuardService::class.java)
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                context.startForegroundService(intent)
            } else {
                context.startService(intent)
            }
        }
    }
}
