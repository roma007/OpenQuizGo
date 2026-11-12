package com.openquizgo.guard.platform

import android.content.Context
import android.content.Intent
import android.provider.Settings

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

    fun accessibilitySettingsIntent(): Intent = Intent(Settings.ACTION_ACCESSIBILITY_SETTINGS)
}
