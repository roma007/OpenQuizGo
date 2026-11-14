package com.openquizgo.guard.debug

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.platform.GuardLog

/** 仅 debug 构建可用：adb 触发“需要答题”。 */
class DebugReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val app = context.applicationContext as GuardApplication
        when (intent?.action) {
            ACTION_FORCE_QUIZ -> {
                app.container.guardEngine.forceQuizRequired()
                GuardLog.i("debug force quiz required")
            }
            ACTION_FORCE_BLOCK -> {
                app.container.guardEngine.onQuizFailed()
                GuardLog.i("debug force blocked")
            }
        }
    }

    companion object {
        const val ACTION_FORCE_QUIZ = "com.openquizgo.guard.DEBUG_FORCE_QUIZ"
        const val ACTION_FORCE_BLOCK = "com.openquizgo.guard.DEBUG_FORCE_BLOCK"
    }
}
