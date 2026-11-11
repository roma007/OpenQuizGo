package com.openquizgo.guard.domain

import com.openquizgo.guard.data.foreground.ForegroundState
import com.openquizgo.guard.data.remote.HeartbeatResult
import com.openquizgo.guard.data.remote.OfflineEntry
import com.openquizgo.guard.data.remote.OfflineLease
import com.openquizgo.guard.data.remote.OfflineReportResult
import com.openquizgo.guard.data.remote.WatchGrant
import com.openquizgo.guard.data.watch.PendingOffline
import com.openquizgo.guard.data.watch.StoredGrant
import com.openquizgo.guard.data.watch.StoredLease
import com.openquizgo.guard.data.watch.WatchApiException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.time.Instant

@OptIn(ExperimentalCoroutinesApi::class)
class GuardEngineTest {

    private val base = 1_700_000_000_000L

    @Test
    fun pollAdoptsParentGrantInQuizRequired() = runTest {
        val h = harness(snapshotState = "QUIZ_REQUIRED")
        assertEquals(GuardState.QUIZ_REQUIRED, h.engine.state.value)

        h.watch.grant = grant(remaining = 300)
        advanceTimeBy(10_001)
        runCurrent()

        assertEquals(GuardState.GRANTED, h.engine.state.value)
        assertEquals(300L, h.engine.grantSeconds.value)
        assertTrue(h.watch.activeCalls >= 2)
    }

    @Test
    fun pollAdoptsGrantWhileMediaActiveGoesStraightToWatching() = runTest {
        val h = harness(snapshotState = "QUIZ_REQUIRED", mediaActive = true)

        h.watch.grant = grant(remaining = 300)
        advanceTimeBy(10_001)
        runCurrent()

        assertEquals(GuardState.WATCHING, h.engine.state.value)
        assertTrue(h.engine.grantSeconds.value in 299L..300L)
    }

    @Test
    fun pollKeepsQuizRequiredWhenNoGrant() = runTest {
        val h = harness(snapshotState = "QUIZ_REQUIRED")

        advanceTimeBy(30_001)
        runCurrent()

        assertEquals(GuardState.QUIZ_REQUIRED, h.engine.state.value)
        assertTrue(h.watch.activeCalls >= 3)
    }

    @Test
    fun pollUnauthorizedMovesToUnbound() = runTest {
        val h = harness(snapshotState = "QUIZ_REQUIRED")
        h.watch.grantError = WatchApiException("AUTH_INVALID_TOKEN", "令牌无效", 401)

        advanceTimeBy(10_001)
        runCurrent()

        assertEquals(GuardState.UNBOUND, h.engine.state.value)
    }

    @Test
    fun heartbeatRevokedMovesToBlocked() = runTest {
        val h = harness(
            snapshotState = "WATCHING",
            mediaActive = true,
            storedGrant = storedGrant(remaining = 300),
        )
        h.watch.heartbeatResult = HeartbeatResult(
            grantId = "g1",
            status = "revoked",
            consumedSeconds = 30,
            remainingSeconds = 300,
            paused = false,
            revoked = true,
            reason = "time_window",
            serverTime = null,
        )

        advanceTimeBy(30_001)
        runCurrent()

        assertEquals(GuardState.BLOCKED, h.engine.state.value)
        assertEquals("time_window", h.engine.blockReason.value)
    }

    @Test
    fun grantExhaustionReturnsToQuizRequired() = runTest {
        val h = harness(
            snapshotState = "WATCHING",
            mediaActive = true,
            storedGrant = storedGrant(remaining = 2, validForMillis = 3_600_000),
        )

        advanceTimeBy(2_001)
        runCurrent()

        assertEquals(GuardState.QUIZ_REQUIRED, h.engine.state.value)
        assertEquals(2L, h.engine.watchSeconds.value)
    }

    @Test
    fun quizPassedAdoptsGrant() = runTest {
        val h = harness(snapshotState = "IDLE")

        h.engine.onQuizPassed(
            GrantInfo(
                grantId = "quiz-grant",
                minutes = 5,
                remainingSeconds = 300,
                validUntilMillis = base + 300_000,
            ),
        )

        assertEquals(GuardState.GRANTED, h.engine.state.value)
        assertEquals(300L, h.engine.grantSeconds.value)
        assertEquals(5, h.engine.grantMinutes.value)
        assertEquals(0L, h.engine.watchSeconds.value)
    }

    @Test
    fun adoptGrantWhileMediaActiveEntersWatching() = runTest {
        val h = harness(snapshotState = "QUIZ_REQUIRED", mediaActive = true)

        h.engine.adoptGrant(
            GrantInfo(
                grantId = "parent-grant",
                minutes = 5,
                remainingSeconds = 300,
                validUntilMillis = base + 300_000,
            ),
        )

        assertEquals(GuardState.WATCHING, h.engine.state.value)
        assertEquals(300L, h.engine.grantSeconds.value)
    }

    @Test
    fun restoreExpiredGrantFallsBackToQuizRequired() = runTest {
        val h = harness(
            snapshotState = "WATCHING",
            mediaActive = true,
            storedGrant = storedGrant(remaining = 0),
        )

        assertEquals(GuardState.QUIZ_REQUIRED, h.engine.state.value)
        assertNull(h.storage.grant)
    }

    // ---- helpers ----

    private data class Harness(
        val engine: GuardEngine,
        val storage: FakeStorage,
        val watch: FakeWatch,
    )

    private fun TestScope.harness(
        snapshotState: String,
        mediaActive: Boolean = false,
        storedGrant: StoredGrant? = null,
    ): Harness {
        val storage = FakeStorage(
            token = "test-token",
            snapshot = snapshotState to 0L,
            initialGrant = storedGrant,
        )
        val foreground = FakeForeground(
            ForegroundState(
                packageName = if (mediaActive) "com.example.video" else null,
                mediaActive = mediaActive,
            ),
        )
        val watch = FakeWatch().apply {
            grant = storedGrant?.takeIf { it.remainingSeconds > 0 }?.let {
                WatchGrant(
                    grantId = it.grantId,
                    sessionId = null,
                    minutes = it.minutes,
                    validFrom = null,
                    validUntil = Instant.ofEpochMilli(it.validUntilMillis).toString(),
                    remainingSeconds = it.remainingSeconds,
                    status = "active",
                    leaseToken = null,
                )
            }
        }
        val engine = GuardEngine(
            storage,
            foreground,
            watch,
            backgroundScope,
        ) { base + testScheduler.currentTime }
        runCurrent()
        return Harness(engine, storage, watch)
    }

    private fun TestScope.grant(remaining: Int): WatchGrant = WatchGrant(
        grantId = "poll-grant",
        sessionId = null,
        minutes = 5,
        validFrom = null,
        validUntil = Instant.ofEpochMilli(base + testScheduler.currentTime + 300_000).toString(),
        remainingSeconds = remaining,
        status = "active",
        leaseToken = null,
    )

    private fun storedGrant(remaining: Int, validForMillis: Long = 300_000): StoredGrant =
        StoredGrant(
            grantId = "g1",
            minutes = 5,
            remainingSeconds = remaining,
            validUntilMillis = base + validForMillis,
        )

    private class FakeStorage(
        token: String?,
        snapshot: Pair<String, Long>,
        initialGrant: StoredGrant?,
    ) : GuardStorage {
        private val tokenFlow = MutableStateFlow(token)
        override val deviceToken: Flow<String?> = tokenFlow

        var snapshot: Pair<String, Long> = snapshot
        var grant: StoredGrant? = initialGrant
        var lease: StoredLease? = null
        var pending: PendingOffline? = null

        override suspend fun guardSnapshot(): Pair<String, Long> = snapshot

        override suspend fun saveGuardSnapshot(state: String, watchSeconds: Long) {
            snapshot = state to watchSeconds
        }

        override suspend fun storedGrant(): StoredGrant? = grant

        override suspend fun saveStoredGrant(grant: StoredGrant?) {
            this.grant = grant
        }

        override suspend fun storedLease(): StoredLease? = lease

        override suspend fun saveStoredLease(lease: StoredLease?) {
            this.lease = lease
        }

        override suspend fun pendingOffline(): PendingOffline? = pending

        override suspend fun savePendingOffline(pending: PendingOffline?) {
            this.pending = pending
        }
    }

    private class FakeForeground(initial: ForegroundState) : ForegroundSource {
        private val flow = MutableStateFlow(initial)
        override val state: StateFlow<ForegroundState> = flow
    }

    private class FakeWatch : WatchGateway {
        var grant: WatchGrant? = null
        var grantError: Throwable? = null
        var heartbeatResult: HeartbeatResult = HeartbeatResult(
            grantId = "g1",
            status = "active",
            consumedSeconds = 30,
            remainingSeconds = 270,
            paused = false,
            revoked = false,
            reason = null,
            serverTime = null,
        )
        var heartbeatError: Throwable? = null
        var activeCalls = 0
        var heartbeatCalls = 0

        override suspend fun activeGrant(): WatchGrant? {
            activeCalls += 1
            grantError?.let { throw it }
            return grant
        }

        override suspend fun heartbeat(
            grantId: String,
            foregroundApp: String?,
            paused: Boolean,
        ): HeartbeatResult {
            heartbeatCalls += 1
            heartbeatError?.let { throw it }
            return heartbeatResult
        }

        override suspend fun offlineLease(): OfflineLease? = null

        override suspend fun reportOffline(
            leaseId: String,
            entries: List<OfflineEntry>,
        ): OfflineReportResult = OfflineReportResult(
            reportedMinutes = 0,
            remainingMinutes = 0,
            clamped = false,
        )
    }
}
