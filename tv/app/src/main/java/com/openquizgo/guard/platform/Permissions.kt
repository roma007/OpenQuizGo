package com.openquizgo.guard.platform

import android.app.AppOpsManager
import android.content.Context
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

    fun accessibilitySettingsIntent() = android.content.Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)

    fun usageSettingsIntent() = android.content.Intent(Settings.ACTION_USAGE_ACCESS_SETTINGS)
}
