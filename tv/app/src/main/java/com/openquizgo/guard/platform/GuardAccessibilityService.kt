package com.openquizgo.guard.platform

import android.accessibilityservice.AccessibilityService
import android.view.accessibility.AccessibilityEvent
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

    override fun onServiceConnected() {
        super.onServiceConnected()
        GuardLog.i("accessibility connected")
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
}
