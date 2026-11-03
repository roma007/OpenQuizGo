package com.openquizgo.guard.platform

import android.app.ActivityManager
import android.app.AppOpsManager
import android.app.NotificationManager
import android.content.Context
import android.os.Build
import android.provider.Settings
import android.os.Process

object Permissions {

    fun accessibilityEnabled(context: Context): Boolean {
        val enabled = Settings.Secure.getString(
            context.contentResolver,
            Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES,
        )
        return enabled != null &&
            enabled.contains(context.packageName) &&
            enabled.contains("GuardAccessibilityService")
    }

    fun usageAccessEnabled(context: Context): Boolean {
        val appOps = context.getSystemService(Context.APP_OPS_SERVICE) as? AppOpsManager ?: return false
        val mode = appOps.checkOpNoThrow(
            AppOpsManager.OPSTR_GET_USAGE_STATS,
            Process.myUid(),
            context.packageName,
        )
        return mode == AppOpsManager.MODE_ALLOWED
    }

    fun notificationsEnabled(context: Context): Boolean {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.N) return true
        val manager = context.getSystemService(Context.NOTIFICATION_SERVICE) as? NotificationManager
            ?: return false
        return manager.areNotificationsEnabled()
    }

    @Suppress("DEPRECATION")
    fun foregroundServiceRunning(context: Context): Boolean {
        val manager = context.getSystemService(Context.ACTIVITY_SERVICE) as? ActivityManager
            ?: return false
        return manager.getRunningServices(Int.MAX_VALUE)
            .any { it.service.className == GuardService::class.java.name }
    }

    fun accessibilitySettingsIntent() = android.content.Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)

    fun usageSettingsIntent() = android.content.Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS)
}
