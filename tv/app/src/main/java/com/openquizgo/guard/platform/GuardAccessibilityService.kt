package com.openquizgo.guard.platform

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.content.Intent
import android.view.accessibility.AccessibilityEvent
import com.openquizgo.guard.ui.QuizActivity
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

object ForegroundTracker {
    private val _packageName = MutableStateFlow<String?>(null)
    val packageName: StateFlow<String?> = _packageName.asStateFlow()

    fun update(pkg: CharSequence?) {
        val value = pkg?.toString()?.trim().orEmpty()
        if (value.isNotEmpty() && value != _packageName.value) {
            _packageName.value = value
        }
    }
}

/**
 * 无障碍服务：记录前台应用，并提供「从系统绑定服务拉起 Activity」的可靠通道。
 *
 * Android 10+ 对后台启动 Activity 有严格限制；通过无障碍服务启动答题界面
 * 是电视端到点自动弹题的关键路径。
 */
class GuardAccessibilityService : AccessibilityService() {

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        getServiceInfo()?.let { info ->
            info.flags = info.flags or AccessibilityServiceInfo.FLAG_INCLUDE_NOT_IMPORTANT_VIEWS
            setServiceInfo(info)
        }
        GuardLog.i("accessibility", "connected")
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event?.eventType != AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED) return
        val pkg = event.packageName ?: return
        ForegroundTracker.update(pkg)
        GuardLog.i("foreground=$pkg cls=${event.className}")
    }

    override fun onInterrupt() {
        GuardLog.w("accessibility", "interrupted")
    }

    private fun launchQuiz() {
        val intent = Intent(this, QuizActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        runCatching { startActivity(intent) }
            .onSuccess { GuardLog.i("accessibility", "quiz activity launched") }
            .onFailure { GuardLog.e("accessibility", "quiz launch failed", it) }
    }

    override fun onDestroy() {
        instance = null
        super.onDestroy()
    }

    companion object {
        @Volatile
        private var instance: GuardAccessibilityService? = null

        fun launchQuizActivity(): Boolean {
            val service = instance ?: return false
            service.launchQuiz()
            return true
        }

        fun isConnected(): Boolean = instance != null
    }
}
