package com.openquizgo.guard.platform

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import com.openquizgo.guard.ui.SelfCheckActivity

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        when (intent?.action) {
            Intent.ACTION_BOOT_COMPLETED,
            ACTION_QUICKBOOT_POWERON,
            ACTION_HTC_QUICKBOOT_POWERON,
            Intent.ACTION_MY_PACKAGE_REPLACED -> {
                GuardLog.i("boot", "${intent.action} -> start guard service")
                runCatching { GuardService.start(context) }
                    .onFailure { GuardLog.e("boot", "start guard service failed", it) }
                // 081：每次开机先做自检，全部通过会立即结束、不显示界面。
                SelfCheckActivity.maybeLaunch(context)
            }
        }
    }

    private companion object {
        const val ACTION_QUICKBOOT_POWERON = "android.intent.action.QUICKBOOT_POWERON"
        const val ACTION_HTC_QUICKBOOT_POWERON = "com.htc.intent.action.QUICKBOOT_POWERON"
    }
}
