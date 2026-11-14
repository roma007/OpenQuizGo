package com.openquizgo.guard.data.foreground

import android.app.usage.UsageEvents
import android.app.usage.UsageStatsManager
import android.content.Context
import android.media.AudioManager
import com.openquizgo.guard.platform.ForegroundTracker
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

data class ForegroundState(
    val packageName: String? = null,
    val mediaActive: Boolean = false,
    val mediaSeconds: Long = 0,
)

/**
 * 前台识别：无障碍事件为主，UsageStats 每 5s 校正；媒体播放以 AudioManager 判断。
 */
class ForegroundDetector(
    context: Context,
    scope: CoroutineScope,
) : com.openquizgo.guard.domain.ForegroundSource {
    private val appContext = context.applicationContext
    private val usageStats = appContext.getSystemService(Context.USAGE_STATS_SERVICE) as? UsageStatsManager
    private val audioManager = appContext.getSystemService(Context.AUDIO_SERVICE) as? AudioManager

    private val _state = MutableStateFlow(ForegroundState())
    override val state: StateFlow<ForegroundState> = _state.asStateFlow()

    init {
        scope.launch {
            ForegroundTracker.packageName.collect { pkg ->
                _state.update { it.copy(packageName = pkg ?: it.packageName) }
            }
        }
        scope.launch {
            while (isActive) {
                delay(5_000)
                refreshFromUsageStats()
                val active = audioManager?.isMusicActive == true
                if (active != _state.value.mediaActive) {
                    GuardLog.i("mediaActive=$active pkg=${_state.value.packageName}")
                }
                _state.update { it.copy(mediaActive = active) }
            }
        }
        scope.launch {
            while (isActive) {
                delay(1_000)
                val current = _state.value
                if (current.mediaActive) {
                    _state.update { it.copy(mediaSeconds = it.mediaSeconds + 1) }
                }
            }
        }
    }

    private fun refreshFromUsageStats() {
        val manager = usageStats ?: return
        val now = System.currentTimeMillis()
        val events: UsageEvents = manager.queryEvents(now - 10_000, now)
        var lastPackage: String? = null
        val event = UsageEvents.Event()
        while (events.hasNextEvent()) {
            events.getNextEvent(event)
            if (event.eventType == UsageEvents.Event.MOVE_TO_FOREGROUND) {
                lastPackage = event.packageName
            }
        }
        if (!lastPackage.isNullOrBlank() && lastPackage != _state.value.packageName) {
            GuardLog.i("usageStats foreground=$lastPackage")
            _state.update { it.copy(packageName = lastPackage) }
        }
    }
}
