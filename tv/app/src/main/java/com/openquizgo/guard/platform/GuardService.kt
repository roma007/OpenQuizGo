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
import com.openquizgo.guard.domain.LocalGuardState
import com.openquizgo.guard.ui.MainActivity
import com.openquizgo.guard.ui.QuizActivity
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * 前台守护服务（本地模式）。
 *
 * 观察本地状态机：累计播放满 15 分钟进入 QUIZ_REQUIRED 后拉起答题界面；
 * 看门狗在答题界面被切走/关闭时重新拉起，确保「答对前不能继续看」。
 */
class GuardService : Service() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private val mainHandler = Handler(Looper.getMainLooper())
    private var stateJob: Job? = null
    private var watchdogJob: Job? = null
    private var engine: com.openquizgo.guard.domain.LocalGuardEngine? = null
    private var lastQuizLaunchAt = 0L
    private var quizNotified = false

    override fun onCreate() {
        super.onCreate()
        GuardLog.i("service", "local guard service created")
        createChannel()
        startForeground(NOTIFICATION_ID, buildNotification("启动中"))
        engine = (application as GuardApplication).container.guardEngine
        stateJob = scope.launch {
            combine(engine!!.enabled, engine!!.state) { enabled, state ->
                enabled to state
            }.collect { (enabled, state) ->
                updateNotification(label(enabled, state))
                when (state) {
                    LocalGuardState.QUIZ_REQUIRED -> {
                        if (!quizNotified) {
                            quizNotified = true
                            toast("答题时间到，答对后继续观看")
                        }
                        launchQuiz()
                    }
                    LocalGuardState.QUIZZING -> Unit
                    else -> {
                        quizNotified = false
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
        if (current != LocalGuardState.QUIZ_REQUIRED &&
            current != LocalGuardState.QUIZZING
        ) {
            return
        }
        if (QuizActivity.isShowing) return
        if (System.currentTimeMillis() - lastQuizLaunchAt < RELAUNCH_GAP_MS) return
        GuardLog.w("watchdog", "quiz activity missing, relaunch")
        launchQuiz()
    }

    private fun launchQuiz() {
        lastQuizLaunchAt = System.currentTimeMillis()
        if (GuardAccessibilityService.launchQuizActivity()) {
            GuardLog.i("service", "quiz launched via accessibility")
            return
        }
        GuardLog.w("service", "accessibility unavailable, fallback startActivity")
        val intent = Intent(this, QuizActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        runCatching { startActivity(intent) }
            .onFailure {
                GuardLog.e("service", "quiz launch failed", it)
                toast("请开启无障碍服务，否则无法自动弹出答题")
            }
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
            "定时答题",
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
            .setSmallIcon(android.R.drawable.ic_lock_idle_alarm)
            .setContentTitle("电视时间管家 · 定时答题")
            .setContentText(text)
            .setContentIntent(pending)
            .setOngoing(true)
            .build()
    }

    private fun updateNotification(text: String) {
        val manager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(NOTIFICATION_ID, buildNotification(text))
    }

    private fun label(enabled: Boolean, state: LocalGuardState): String {
        if (!enabled) return "已关闭，不会弹出答题"
        return when (state) {
            LocalGuardState.IDLE -> "已开启 · 等待视频播放"
            LocalGuardState.WATCHING -> "已开启 · 正在累计观看时间"
            LocalGuardState.QUIZ_REQUIRED -> "答题时间到"
            LocalGuardState.QUIZZING -> "答题中，答对后继续"
            LocalGuardState.GRANTED -> "回答正确，继续观看"
        }
    }

    companion object {
        private const val CHANNEL_ID = "local_quiz"
        private const val NOTIFICATION_ID = 1001
        private const val WATCHDOG_INTERVAL_MS = 2_000L
        private const val RELAUNCH_GAP_MS = 3_000L

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
