package com.openquizgo.guard.domain

import com.openquizgo.guard.data.foreground.PlaybackSource
import com.openquizgo.guard.data.local.LocalSettingsSnapshot
import com.openquizgo.guard.data.local.LocalSettingsStore
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class LocalGuardEngineTest {

    private class FakeSettings(
        var enabled: Boolean = true,
        var elapsed: Long = 0,
        var lastQuestionId: String? = null,
    ) : LocalSettingsStore {
        private val enabledFlow = MutableStateFlow(enabled)
        override val quizEnabled: Flow<Boolean> = enabledFlow

        override suspend fun snapshot(): LocalSettingsSnapshot =
            LocalSettingsSnapshot(enabled, elapsed, lastQuestionId)

        override suspend fun saveEnabled(enabled: Boolean) {
            this.enabled = enabled
            enabledFlow.value = enabled
        }

        override suspend fun saveElapsedSeconds(seconds: Long) {
            elapsed = seconds
        }

        override suspend fun saveLastQuestionId(id: String?) {
            lastQuestionId = id
        }
    }

    private class FakePlayback : PlaybackSource {
        private val state = MutableStateFlow(false)
        override val mediaActive: StateFlow<Boolean> = state.asStateFlow()

        fun setActive(value: Boolean) {
            state.value = value
        }
    }

    private fun engine(
        scope: kotlinx.coroutines.CoroutineScope,
        settings: FakeSettings = FakeSettings(),
        playback: FakePlayback = FakePlayback(),
        intervalSeconds: Long = 3,
    ): LocalGuardEngine = LocalGuardEngine(
        settings = settings,
        playback = playback,
        scope = scope,
        intervalSeconds = intervalSeconds,
        tickMillis = 1_000,
        persistEveryTicks = 1,
    )

    @Test
    fun `播放满 interval 后进入待答题`() = runTest {
        val settings = FakeSettings()
        val playback = FakePlayback()
        val engine = engine(backgroundScope, settings, playback)
        runCurrent()

        playback.setActive(true)
        advanceTimeBy(2_000)
        runCurrent()
        assertEquals(LocalGuardState.WATCHING, engine.state.value)
        assertEquals(2L, engine.elapsedSeconds.value)

        advanceTimeBy(1_000)
        runCurrent()
        assertEquals(LocalGuardState.QUIZ_REQUIRED, engine.state.value)
        assertEquals(3L, engine.elapsedSeconds.value)
    }

    @Test
    fun `暂停播放不累计时间`() = runTest {
        val playback = FakePlayback()
        val engine = engine(backgroundScope, playback = playback)
        runCurrent()

        playback.setActive(true)
        advanceTimeBy(2_000)
        runCurrent()
        assertEquals(2L, engine.elapsedSeconds.value)

        playback.setActive(false)
        advanceTimeBy(5_000)
        runCurrent()
        assertEquals(2L, engine.elapsedSeconds.value)
        assertEquals(LocalGuardState.IDLE, engine.state.value)
    }

    @Test
    fun `答对后计时清零并回到观看态`() = runTest {
        val playback = FakePlayback()
        val engine = engine(backgroundScope, playback = playback)
        runCurrent()
        playback.setActive(true)
        advanceTimeBy(3_000)
        runCurrent()
        assertEquals(LocalGuardState.QUIZ_REQUIRED, engine.state.value)

        engine.onQuizStarted()
        assertEquals(LocalGuardState.QUIZZING, engine.state.value)
        engine.onQuizPassed("m001")
        runCurrent()

        assertEquals(LocalGuardState.GRANTED, engine.state.value)
        assertEquals(0L, engine.elapsedSeconds.value)

        advanceTimeBy(1_000)
        runCurrent()
        assertEquals(LocalGuardState.WATCHING, engine.state.value)
        assertEquals(1L, engine.elapsedSeconds.value)
    }

    @Test
    fun `关闭开关后不再计时和触发`() = runTest {
        val playback = FakePlayback()
        val engine = engine(backgroundScope, playback = playback)
        runCurrent()
        playback.setActive(true)

        engine.setEnabled(false)
        runCurrent()
        assertFalse(engine.enabled.value)
        assertEquals(LocalGuardState.IDLE, engine.state.value)

        advanceTimeBy(10_000)
        runCurrent()
        assertEquals(0L, engine.elapsedSeconds.value)
        assertEquals(LocalGuardState.IDLE, engine.state.value)
    }

    @Test
    fun `重新开启后从零开始计时`() = runTest {
        val playback = FakePlayback()
        val engine = engine(backgroundScope, playback = playback)
        runCurrent()
        playback.setActive(true)
        advanceTimeBy(2_000)
        runCurrent()
        assertEquals(2L, engine.elapsedSeconds.value)

        engine.setEnabled(false)
        engine.setEnabled(true)
        runCurrent()
        assertEquals(0L, engine.elapsedSeconds.value)

        advanceTimeBy(1_000)
        runCurrent()
        assertEquals(1L, engine.elapsedSeconds.value)
        assertTrue(engine.enabled.value)
    }

    @Test
    fun `启动时恢复已累计时间`() = runTest {
        val playback = FakePlayback()
        val engine = engine(
            scope = backgroundScope,
            settings = FakeSettings(enabled = true, elapsed = 2),
            playback = playback,
        )
        runCurrent()
        assertEquals(2L, engine.elapsedSeconds.value)

        playback.setActive(true)
        advanceTimeBy(1_000)
        runCurrent()
        assertEquals(LocalGuardState.QUIZ_REQUIRED, engine.state.value)
    }

    @Test
    fun `启动时开关关闭则不触发`() = runTest {
        val playback = FakePlayback()
        val engine = engine(
            scope = backgroundScope,
            settings = FakeSettings(enabled = false, elapsed = 0),
            playback = playback,
        )
        runCurrent()
        assertFalse(engine.enabled.value)

        playback.setActive(true)
        advanceTimeBy(5_000)
        runCurrent()
        assertEquals(LocalGuardState.IDLE, engine.state.value)
    }
}
