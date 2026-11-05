package com.openquizgo.guard.platform

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class BootReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent?) {
        when (intent?.action) {
            Intent.ACTION_BOOT_COMPLETED,
            Intent.ACTION_MY_PACKAGE_REPLACED -> {
                GuardLog.i("boot", "boot/replace -> start guard service")
                runCatching { GuardService.start(context) }
                    .onFailure { GuardLog.e("boot", "start guard service failed", it) }
            }
        }
    }
}
