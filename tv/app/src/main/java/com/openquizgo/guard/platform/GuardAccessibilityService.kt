package com.openquizgo.guard.platform

import android.accessibilityservice.AccessibilityService
import android.accessibilityservice.AccessibilityServiceInfo
import android.content.Intent
import android.os.Handler
import android.os.Looper
import android.view.KeyEvent
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

class GuardAccessibilityService : AccessibilityService() {

    private val handler = Handler(Looper.getMainLooper())

    @Volatile
    private var quizArmed = false

    override fun onServiceConnected() {
        super.onServiceConnected()
        instance = this
        getServiceInfo()?.let { info ->
            info.flags = info.flags or AccessibilityServiceInfo.FLAG_REQUEST_FILTER_KEY_EVENTS
            setServiceInfo(info)
        }
        GuardLog.i("accessibility connected")
    }

    override fun onKeyEvent(event: KeyEvent): Boolean {
        if (event.action == KeyEvent.ACTION_UP && event.keyCode == KeyEvent.KEYCODE_DPAD_CENTER) {
            GuardLog.i("confirm key, armed=$quizArmed")
            if (quizArmed) {
                quizArmed = false
                handler.postDelayed({ launchQuiz() }, 500)
            }
        }
        return false
    }

    private fun launchQuiz() {
        val intent = Intent(this, QuizActivity::class.java)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_CLEAR_TOP)
        startActivity(intent)
        GuardLog.i("quiz activity launched")
    }

    fun armQuiz() {
        quizArmed = true
    }

    fun disarmQuiz() {
        quizArmed = false
    }

    override fun onDestroy() {
        instance = null
        super.onDestroy()
    }

    override fun onAccessibilityEvent(event: AccessibilityEvent?) {
        if (event?.eventType != AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED) return
        val pkg = event.packageName ?: return
        ForegroundTracker.update(pkg)
        GuardLog.i("foreground=$pkg cls=${event.className}")
    }

    override fun onInterrupt() {
        GuardLog.w("accessibility interrupted")
    }

    companion object {
        @Volatile
        private var instance: GuardAccessibilityService? = null

        fun armQuiz() {
            instance?.armQuiz()
        }

        fun disarmQuiz() {
            instance?.disarmQuiz()
        }

        fun isConnected(): Boolean = instance != null
    }
}
