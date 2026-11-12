package com.openquizgo.guard.debug

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.platform.GuardLog

/** 仅 debug 构建可用：adb 立即触发一次答题，便于验收 15 分钟定时逻辑。 */
class DebugReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        val app = context.applicationContext as GuardApplication
        if (intent?.action == ACTION_FORCE_QUIZ) {
            app.container.guardEngine.forceQuizRequired()
            GuardLog.i("debug force quiz required")
        }
    }

    companion object {
        const val ACTION_FORCE_QUIZ = "com.openquizgo.guard.DEBUG_FORCE_QUIZ"
    }
}
