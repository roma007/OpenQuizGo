package com.openquizgo.guard.domain

import com.openquizgo.guard.data.foreground.PlaybackSource
import com.openquizgo.guard.data.local.LocalSettingsStore
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

enum class LocalGuardState {
    IDLE,
    WATCHING,
    QUIZ_REQUIRED,
    QUIZZING,
    GRANTED,
}

/**
 * 本地守护状态机（077）。
 *
 * 只统计媒体播放时间：累计满 [intervalSeconds] 秒进入 QUIZ_REQUIRED；
 * 答对后计时清零并回到观看态；答错由答题界面保持 QUIZZING，直到答对。
 */
class LocalGuardEngine(
    private val settings: LocalSettingsStore,
    private val playback: PlaybackSource,
    private val scope: CoroutineScope,
    val intervalSeconds: Long = DEFAULT_INTERVAL_SECONDS,
    private val tickMillis: Long = 1_000L,
    private val persistEveryTicks: Int = 10,
) {
    private val _state = MutableStateFlow(LocalGuardState.IDLE)
    val state: StateFlow<LocalGuardState> = _state.asStateFlow()

    private val _enabled = MutableStateFlow(true)
    val enabled: StateFlow<Boolean> = _enabled.asStateFlow()

    private val _elapsedSeconds = MutableStateFlow(0L)
    val elapsedSeconds: StateFlow<Long> = _elapsedSeconds.asStateFlow()

    private val _remainingSeconds = MutableStateFlow(intervalSeconds)
    val remainingSeconds: StateFlow<Long> = _remainingSeconds.asStateFlow()

    private val _lastQuestionId = MutableStateFlow<String?>(null)
    val lastQuestionId: StateFlow<String?> = _lastQuestionId.asStateFlow()

    @Volatile
    private var ready = false

    private var tickCount = 0
    private var lastSavedElapsed = -1L
    private var lastSavedEnabled: Boolean? = null

    init {
        scope.launch { restore() }
        scope.launch { tickLoop() }
    }

    fun setEnabled(value: Boolean) {
        if (_enabled.value == value) return
        _enabled.value = value
        if (value) {
            _state.value = if (playback.mediaActive.value) {
                LocalGuardState.WATCHING
            } else {
                LocalGuardState.IDLE
            }
        } else {
            _elapsedSeconds.value = 0
            _remainingSeconds.value = intervalSeconds
            _state.value = LocalGuardState.IDLE
        }
        persist()
        GuardLog.i("local", "quiz enabled=$value")
    }

    fun onQuizStarted() {
        _state.value = LocalGuardState.QUIZZING
        GuardLog.i("local", "quiz started")
    }

    fun onQuizPassed(questionId: String) {
        rememberQuestion(questionId)
        _elapsedSeconds.value = 0
        _remainingSeconds.value = intervalSeconds
        _state.value = LocalGuardState.GRANTED
        persist()
        GuardLog.i("local", "quiz passed question=$questionId, timer reset")
    }

    fun onQuizCancelled() {
        _state.value = if (_enabled.value) {
            LocalGuardState.QUIZ_REQUIRED
        } else {
            LocalGuardState.IDLE
        }
    }

    /** 调试入口：立即进入待答题，便于验收（正式界面不暴露）。 */
    fun forceQuizRequired() {
        if (!_enabled.value) return
        _elapsedSeconds.value = intervalSeconds
        _remainingSeconds.value = 0
        _state.value = LocalGuardState.QUIZ_REQUIRED
        persist()
    }

    /** 记住本轮的题目，下一轮优先换一题。 */
    fun rememberQuestion(questionId: String) {
        if (_lastQuestionId.value == questionId) return
        _lastQuestionId.value = questionId
        scope.launch { runCatching { settings.saveLastQuestionId(questionId) } }
    }

    private suspend fun restore() {
        val snapshot = runCatching { settings.snapshot() }.getOrElse { error ->
            GuardLog.e("local", "read settings failed", error)
            null
        }
        if (snapshot != null) {
            _enabled.value = snapshot.enabled
            _elapsedSeconds.value = snapshot.elapsedSeconds.coerceIn(0, intervalSeconds)
            _lastQuestionId.value = snapshot.lastQuestionId
        }
        _remainingSeconds.value = (intervalSeconds - _elapsedSeconds.value).coerceAtLeast(0)
        _state.value = when {
            !_enabled.value -> LocalGuardState.IDLE
            _elapsedSeconds.value >= intervalSeconds -> LocalGuardState.QUIZ_REQUIRED
            else -> LocalGuardState.IDLE
        }
        ready = true
        GuardLog.i(
            "local",
            "restored enabled=${_enabled.value} elapsed=${_elapsedSeconds.value}s " +
                "state=${_state.value}",
        )
    }

    private suspend fun tickLoop() {
        while (scope.isActive) {
            delay(tickMillis)
            if (!ready) continue

            if (!_enabled.value) {
                if (_state.value != LocalGuardState.IDLE) {
                    _state.value = LocalGuardState.IDLE
                }
                continue
            }

            when (_state.value) {
                LocalGuardState.QUIZ_REQUIRED,
                LocalGuardState.QUIZZING,
                -> Unit

                else -> {
                    if (playback.mediaActive.value) {
                        if (_state.value == LocalGuardState.IDLE ||
                            _state.value == LocalGuardState.GRANTED
                        ) {
                            _state.value = LocalGuardState.WATCHING
                        }
                        val next = (_elapsedSeconds.value + 1).coerceAtMost(intervalSeconds)
                        _elapsedSeconds.value = next
                        if (next >= intervalSeconds) {
                            _state.value = LocalGuardState.QUIZ_REQUIRED
                            GuardLog.i("local", "quiz due after ${next}s")
                            persist()
                        }
                    } else if (_state.value == LocalGuardState.WATCHING ||
                        _state.value == LocalGuardState.GRANTED
                    ) {
                        _state.value = LocalGuardState.IDLE
                    }
                }
            }

            _remainingSeconds.value = (intervalSeconds - _elapsedSeconds.value).coerceAtLeast(0)
            tickCount += 1
            if (persistEveryTicks > 0 && tickCount % persistEveryTicks == 0) {
                persist()
            }
        }
    }

    private fun persist() {
        val enabled = _enabled.value
        val elapsed = _elapsedSeconds.value
        if (lastSavedEnabled == enabled && lastSavedElapsed == elapsed) return
        lastSavedElapsed = elapsed
        lastSavedEnabled = enabled
        scope.launch {
            runCatching {
                settings.saveEnabled(enabled)
                settings.saveElapsedSeconds(elapsed)
            }.onFailure { GuardLog.w("local", "persist failed: ${it.message}") }
        }
    }

    companion object {
        const val DEFAULT_INTERVAL_SECONDS = 15L * 60L
    }
}
