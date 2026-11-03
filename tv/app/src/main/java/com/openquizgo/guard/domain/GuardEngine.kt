package com.openquizgo.guard.domain

import com.openquizgo.guard.data.foreground.ForegroundDetector
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * 守护状态机（T4）：
 * UNBOUND -> IDLE -> WATCHING -> QUIZ_REQUIRED -> QUIZZING -> GRANTED/BLOCKED
 *
 * 当前为本地演示阈值，T6/T7 接入服务端规则与答题会话后替换。
 */
class GuardEngine(
    private val preferences: GuardPreferences,
    private val foregroundDetector: ForegroundDetector,
    private val scope: CoroutineScope,
) {
    private val _state = MutableStateFlow(GuardState.UNBOUND)
    val state: StateFlow<GuardState> = _state.asStateFlow()

    private val _watchSeconds = MutableStateFlow(0L)
    val watchSeconds: StateFlow<Long> = _watchSeconds.asStateFlow()

    init {
        scope.launch { restore() }
        scope.launch {
            preferences.deviceToken.collect { token ->
                if (token.isNullOrBlank()) {
                    transition(GuardState.UNBOUND)
                } else if (_state.value == GuardState.UNBOUND) {
                    transition(GuardState.IDLE)
                }
            }
        }
        scope.launch {
            foregroundDetector.state.collect { foreground ->
                val current = _state.value
                if (foreground.mediaActive && (current == GuardState.IDLE || current == GuardState.GRANTED)) {
                    transition(GuardState.WATCHING)
                } else if (!foreground.mediaActive && current == GuardState.WATCHING) {
                    transition(GuardState.IDLE)
                }
            }
        }
        scope.launch {
            while (isActive) {
                delay(1_000)
                if (_state.value == GuardState.WATCHING && foregroundDetector.state.value.mediaActive) {
                    _watchSeconds.value += 1
                    if (_watchSeconds.value >= DEMO_QUIZ_THRESHOLD_SECONDS) {
                        transition(GuardState.QUIZ_REQUIRED)
                    }
                }
            }
        }
        scope.launch {
            while (isActive) {
                delay(10_000)
                persist()
            }
        }
    }

    fun onQuizStarted() {
        transition(GuardState.QUIZZING)
    }

    fun onQuizPassed() {
        _watchSeconds.value = 0
        transition(GuardState.GRANTED)
    }

    fun onQuizFailed() {
        transition(GuardState.BLOCKED)
    }

    fun onQuizCancelled() {
        transition(GuardState.QUIZ_REQUIRED)
    }

    /** 调试/测试入口：立即触发答题。 */
    fun forceQuizRequired() {
        transition(GuardState.QUIZ_REQUIRED)
    }

    fun onWatchingStopped() {
        if (_state.value == GuardState.GRANTED || _state.value == GuardState.BLOCKED) {
            transition(GuardState.IDLE)
        }
    }

    private suspend fun restore() {
        val snapshot = preferences.guardSnapshot()
        _watchSeconds.value = snapshot.second
        val restored = runCatching { GuardState.valueOf(snapshot.first) }.getOrNull()
        val state = when (restored) {
            GuardState.WATCHING, GuardState.QUIZ_REQUIRED,
            GuardState.QUIZZING, GuardState.GRANTED, GuardState.BLOCKED -> restored
            else -> GuardState.IDLE
        }
        val token = preferences.deviceToken.first()
        _state.value = if (token.isNullOrBlank()) GuardState.UNBOUND else state
        GuardLog.i("restored state=${_state.value} seconds=${_watchSeconds.value}")
    }

    private fun transition(next: GuardState) {
        if (_state.value == next) return
        GuardLog.i("state ${_state.value} -> $next")
        _state.value = next
        scope.launch { persist() }
    }

    private suspend fun persist() {
        preferences.saveGuardSnapshot(_state.value.name, _watchSeconds.value)
    }

    companion object {
        const val DEMO_QUIZ_THRESHOLD_SECONDS = 60L
    }
}
