package com.openquizgo.guard.platform

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

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
            }
        }
    }

    private companion object {
        const val ACTION_QUICKBOOT_POWERON = "android.intent.action.QUICKBOOT_POWERON"
        const val ACTION_HTC_QUICKBOOT_POWERON = "com.htc.intent.action.QUICKBOOT_POWERON"
    }
}
