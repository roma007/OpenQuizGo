package com.openquizgo.guard.domain

import com.openquizgo.guard.data.foreground.ForegroundDetector
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.OfflineEntry
import com.openquizgo.guard.data.watch.PendingOffline
import com.openquizgo.guard.data.watch.StoredGrant
import com.openquizgo.guard.data.watch.StoredLease
import com.openquizgo.guard.data.watch.WatchRepository
import com.openquizgo.guard.platform.DiagnosticsReporter
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.time.Instant

/** 答题通过后下发的授权信息。 */
data class GrantInfo(
    val grantId: String,
    val minutes: Int,
    val remainingSeconds: Int,
    val validUntilMillis: Long,
)

/**
 * 守护状态机（T4/T7）：
 * UNBOUND -> IDLE -> WATCHING -> QUIZ_REQUIRED -> QUIZZING -> GRANTED/BLOCKED
 *
 * T7 起以服务端授权为准：本地倒计时 + 30s 心跳校正；
 * 暂停（无媒体播放）不计入观看时长；断网期间按离线租约累计，联网后补报。
 */
class GuardEngine(
    private val preferences: GuardPreferences,
    private val foregroundDetector: ForegroundDetector,
    private val watchRepository: WatchRepository,
    private val diagnostics: DiagnosticsReporter,
    private val scope: CoroutineScope,
) {
    private val _state = MutableStateFlow(GuardState.UNBOUND)
    val state: StateFlow<GuardState> = _state.asStateFlow()

    private val _watchSeconds = MutableStateFlow(0L)
    val watchSeconds: StateFlow<Long> = _watchSeconds.asStateFlow()

    private val _grantMinutes = MutableStateFlow(0)
    val grantMinutes: StateFlow<Int> = _grantMinutes.asStateFlow()

    /** 当前授权剩余秒数（本地倒计时，心跳后以服务端为准）。 */
    private val _grantSeconds = MutableStateFlow(0L)
    val grantSeconds: StateFlow<Long> = _grantSeconds.asStateFlow()

    /** 进入 BLOCKED 的原因（quiz_failed / time_window / revoked）。 */
    private val _blockReason = MutableStateFlow<String?>(null)
    val blockReason: StateFlow<String?> = _blockReason.asStateFlow()

    private var grantId: String? = null
    private var grantValidUntilMillis: Long = 0
    private var lastHeartbeatMillis: Long = 0
    private var offline = false

    @Volatile
    private var lease: StoredLease? = null

    @Volatile
    private var pending: PendingOffline? = null

    @Volatile
    private var reportingPending = false

    init {
        scope.launch { restore() }
        scope.launch {
            preferences.deviceToken.distinctUntilChanged().collect { token ->
                if (token.isNullOrBlank()) {
                    transition(GuardState.UNBOUND)
                } else {
                    if (_state.value == GuardState.UNBOUND) {
                        transition(GuardState.IDLE)
                    }
                    refreshLease()
                }
            }
        }
        scope.launch {
            foregroundDetector.state.collect { foreground ->
                val current = _state.value
                if (_state.value == GuardState.UNBOUND) return@collect
                if (foreground.mediaActive && (current == GuardState.IDLE || current == GuardState.GRANTED)) {
                    transition(GuardState.WATCHING)
                    if (!hasUsableGrant()) {
                        transition(GuardState.QUIZ_REQUIRED)
                    }
                } else if (!foreground.mediaActive && current == GuardState.WATCHING) {
                    transition(GuardState.IDLE)
                }
            }
        }
        scope.launch {
            while (isActive) {
                delay(1_000)
                tick()
            }
        }
        scope.launch {
            while (isActive) {
                delay(HEARTBEAT_INTERVAL_MS)
                heartbeat()
            }
        }
        scope.launch {
            while (isActive) {
                delay(10_000)
                persist()
            }
        }
        scope.launch {
            while (isActive) {
                delay(LEASE_REFRESH_INTERVAL_MS)
                refreshLease()
            }
        }
        scope.launch {
            while (isActive) {
                delay(PENDING_RETRY_INTERVAL_MS)
                if (pending != null) reportPendingOffline()
            }
        }
    }

    fun onQuizStarted() {
        transition(GuardState.QUIZZING)
    }

    fun onQuizPassed(grant: GrantInfo?) {
        _watchSeconds.value = 0
        if (grant != null) {
            adoptGrantInternal(grant)
        } else {
            grantId = null
            _grantSeconds.value = 0
            _grantMinutes.value = 0
        }
        _blockReason.value = null
        transition(GuardState.GRANTED)
    }

    /** 家长端/后台签发新授权后由阻挡页调用，直接恢复观看。 */
    fun adoptGrant(grant: GrantInfo) {
        GuardLog.i("grant", "adopt id=${grant.grantId} minutes=${grant.minutes}")
        _watchSeconds.value = 0
        adoptGrantInternal(grant)
        _blockReason.value = null
        transition(GuardState.GRANTED)
        if (foregroundDetector.state.value.mediaActive) {
            transition(GuardState.WATCHING)
        }
    }

    private fun adoptGrantInternal(grant: GrantInfo) {
        grantId = grant.grantId
        grantValidUntilMillis = grant.validUntilMillis
        _grantSeconds.value = grant.remainingSeconds.toLong().coerceAtLeast(0)
        _grantMinutes.value = grant.minutes
        lastHeartbeatMillis = System.currentTimeMillis()
        offline = false
        scope.launch {
            preferences.saveStoredGrant(
                StoredGrant(
                    grantId = grant.grantId,
                    minutes = grant.minutes,
                    remainingSeconds = grant.remainingSeconds,
                    validUntilMillis = grant.validUntilMillis,
                ),
            )
        }
        GuardLog.i(
            "grant",
            "accepted id=${grant.grantId} minutes=${grant.minutes} " +
                "remaining=${grant.remainingSeconds}",
        )
    }

    fun onQuizFailed() {
        _blockReason.value = "quiz_failed"
        transition(GuardState.BLOCKED)
    }

    fun onQuizCancelled() {
        if (hasUsableGrant()) {
            transition(GuardState.GRANTED)
            if (foregroundDetector.state.value.mediaActive) {
                transition(GuardState.WATCHING)
            }
        } else {
            transition(GuardState.QUIZ_REQUIRED)
        }
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

        val stored = preferences.storedGrant()
        val now = System.currentTimeMillis()
        if (stored != null && stored.remainingSeconds > 0 && stored.validUntilMillis > now) {
            grantId = stored.grantId
            grantValidUntilMillis = stored.validUntilMillis
            _grantSeconds.value = stored.remainingSeconds.toLong()
            _grantMinutes.value = stored.minutes
            GuardLog.i("restored grant id=${stored.grantId} remaining=${stored.remainingSeconds}")
        } else {
            if (stored != null) {
                GuardLog.i("stored grant expired, clearing")
                preferences.saveStoredGrant(null)
            }
        }
        pending = preferences.pendingOffline()
        lease = preferences.storedLease()

        val restored = runCatching { GuardState.valueOf(snapshot.first) }.getOrNull()
        var state = when (restored) {
            // 重启后答题界面已不存在，QUIZZING 视为待答题，后续按授权/媒体状态恢复。
            GuardState.QUIZZING -> GuardState.QUIZ_REQUIRED
            GuardState.WATCHING, GuardState.QUIZ_REQUIRED,
            GuardState.GRANTED, GuardState.BLOCKED -> restored
            else -> GuardState.IDLE
        }
        if (!hasUsableGrant() && (state == GuardState.GRANTED || state == GuardState.WATCHING)) {
            state = GuardState.QUIZ_REQUIRED
        }
        val token = preferences.deviceToken.first()
        _state.value = if (token.isNullOrBlank()) GuardState.UNBOUND else state
        GuardLog.i("restored state=${_state.value} seconds=${_watchSeconds.value}")

        if (!token.isNullOrBlank()) {
            syncActiveGrant()
        }
    }

    private fun tick() {
        val current = _state.value
        if (current != GuardState.WATCHING) return
        if (!foregroundDetector.state.value.mediaActive) return

        _watchSeconds.value += 1
        if (!hasUsableGrant()) {
            GuardLog.w("watching without usable grant")
            transition(GuardState.QUIZ_REQUIRED)
            return
        }
        val next = (_grantSeconds.value - 1).coerceAtLeast(0)
        _grantSeconds.value = next
        if (next <= 0) {
            GuardLog.i("grant exhausted locally")
            transition(GuardState.QUIZ_REQUIRED)
        }
        if (offline) {
            recordOfflineTick()
        }
    }

    private suspend fun heartbeat() {
        if (_state.value == GuardState.UNBOUND || _state.value == GuardState.QUIZZING) return
        val id = grantId
        if (id == null) {
            if (_state.value == GuardState.QUIZ_REQUIRED || _state.value == GuardState.BLOCKED) return
            if (hasUsableGrant()) return
            syncActiveGrant()
            return
        }
        val now = System.currentTimeMillis()
        if (now - lastHeartbeatMillis < HEARTBEAT_INTERVAL_MS - 2_000) return

        val paused = !(_state.value == GuardState.WATCHING &&
            foregroundDetector.state.value.mediaActive)
        runCatching {
            watchRepository.heartbeat(
                grantId = id,
                foregroundApp = foregroundDetector.state.value.packageName,
                paused = paused,
            )
        }.onSuccess { result ->
            offline = false
            lastHeartbeatMillis = now
            _grantSeconds.value = (result.remainingSeconds ?: 0).toLong().coerceAtLeast(0)
            GuardLog.i(
                "heartbeat",
                "status=${result.status} remaining=${result.remainingSeconds} " +
                    "paused=${result.paused} revoked=${result.revoked}",
            )
            scope.launch { persistGrant() }
            val active = result.status == null || result.status == "active"
            when {
                result.revoked == true -> {
                    clearGrant()
                    _blockReason.value = result.reason ?: "revoked"
                    transition(GuardState.BLOCKED)
                }
                !active -> {
                    clearGrant()
                    transition(GuardState.QUIZ_REQUIRED)
                }
                _grantSeconds.value <= 0 -> transition(GuardState.QUIZ_REQUIRED)
            }
            reportPendingOffline()
        }.onFailure { error ->
            offline = true
            GuardLog.w("heartbeat", "failed: ${error.message}")
            if (error is com.openquizgo.guard.data.watch.WatchApiException) {
                when (error.httpStatus) {
                    401 -> transition(GuardState.UNBOUND)
                    404 -> {
                        grantId = null
                        _grantSeconds.value = 0
                        scope.launch { preferences.saveStoredGrant(null) }
                        syncActiveGrant()
                    }
                }
            }
        }
    }

    private suspend fun syncActiveGrant() {
        if (_state.value == GuardState.UNBOUND || _state.value == GuardState.QUIZZING) return
        runCatching { watchRepository.activeGrant() }
            .onSuccess { grant ->
                if (grant == null) {
                    GuardLog.w("no active grant")
                    if (grantId != null) {
                        grantId = null
                        _grantSeconds.value = 0
                        preferences.saveStoredGrant(null)
                    }
                    if (foregroundDetector.state.value.mediaActive &&
                        _state.value != GuardState.QUIZ_REQUIRED
                    ) {
                        transition(GuardState.QUIZ_REQUIRED)
                    }
                    return@onSuccess
                }
                grantId = grant.grantId
                grantValidUntilMillis = parseMillis(grant.validUntil) ?: grantValidUntilMillis
                _grantSeconds.value = grant.remainingSeconds.toLong().coerceAtLeast(0)
                _grantMinutes.value = grant.minutes
                lastHeartbeatMillis = System.currentTimeMillis()
                offline = false
                GuardLog.i(
                    "grant",
                    "refreshed id=${grant.grantId} remaining=${grant.remainingSeconds} " +
                        "status=${grant.status}",
                )
                preferences.saveStoredGrant(
                    StoredGrant(
                        grantId = grant.grantId,
                        minutes = grant.minutes,
                        remainingSeconds = grant.remainingSeconds,
                        validUntilMillis = grantValidUntilMillis,
                    ),
                )
                if (_state.value == GuardState.QUIZ_REQUIRED) {
                    transition(GuardState.GRANTED)
                    if (foregroundDetector.state.value.mediaActive) {
                        transition(GuardState.WATCHING)
                    }
                }
                reportPendingOffline()
            }
            .onFailure { error ->
                GuardLog.w("grant", "refresh failed: ${error.message}")
                offline = true
            }
    }

    private suspend fun refreshLease() {
        if (preferences.deviceToken.first().isNullOrBlank()) return
        runCatching { watchRepository.offlineLease() }
            .onSuccess { offlineLease ->
                if (offlineLease == null) {
                    GuardLog.i("offline lease unavailable")
                    lease = null
                    preferences.saveStoredLease(null)
                    return@onSuccess
                }
                val expires = parseMillis(offlineLease.expiresAt) ?: 0
                val stored = StoredLease(
                    leaseId = offlineLease.leaseId,
                    maxMinutes = offlineLease.maxMinutes,
                    expiresAtMillis = expires,
                    allowedApps = offlineLease.allowedApps.orEmpty(),
                )
                lease = stored
                preferences.saveStoredLease(stored)
                GuardLog.i("offline lease=${stored.leaseId} max=${stored.maxMinutes}m")
            }
            .onFailure { GuardLog.w("offline lease failed: ${it.message}") }
    }

    private fun recordOfflineTick() {
        val now = System.currentTimeMillis()
        val currentLease = lease ?: return
        if (currentLease.expiresAtMillis in 1..now) return
        val existing = pending
        if (existing != null && existing.watchedSeconds >= currentLease.maxMinutes * 60) {
            GuardLog.w("offline allowance exhausted")
            transition(GuardState.QUIZ_REQUIRED)
            return
        }
        val start = existing?.startMillis ?: now
        pending = PendingOffline(
            leaseId = currentLease.leaseId,
            startMillis = start,
            endMillis = now,
            watchedSeconds = (existing?.watchedSeconds ?: 0) + 1,
            pausedSeconds = existing?.pausedSeconds ?: 0,
            targetApp = foregroundDetector.state.value.packageName ?: existing?.targetApp,
        )
    }

    private suspend fun reportPendingOffline() {
        if (reportingPending) return
        reportingPending = true
        try {
            val snapshot = pending ?: return
            if (snapshot.watchedSeconds <= 0) {
                pending = null
                preferences.savePendingOffline(null)
                return
            }
            val leaseId = snapshot.leaseId
            if (leaseId == null) {
                GuardLog.w("offline usage without lease, dropping ${snapshot.watchedSeconds}s")
                pending = null
                preferences.savePendingOffline(null)
                return
            }
            val entry = OfflineEntry(
                startAt = Instant.ofEpochMilli(snapshot.startMillis).toString(),
                endAt = Instant.ofEpochMilli(snapshot.endMillis).toString(),
                watchedSeconds = snapshot.watchedSeconds,
                pausedSeconds = snapshot.pausedSeconds,
                targetApp = snapshot.targetApp,
            )
            runCatching { watchRepository.reportOffline(leaseId, listOf(entry)) }
                .onSuccess { result ->
                    GuardLog.i(
                        "offline reported=${result.reportedMinutes}m remaining=${result.remainingMinutes}m " +
                            "clamped=${result.clamped}",
                    )
                    pending = null
                    preferences.savePendingOffline(null)
                }
                .onFailure { error ->
                    GuardLog.w("offline report failed: ${error.message}")
                    if (error is com.openquizgo.guard.data.watch.WatchApiException &&
                        error.httpStatus in listOf(401, 403, 404, 409, 410, 422)
                    ) {
                        pending = null
                        preferences.savePendingOffline(null)
                    }
                }
        } finally {
            reportingPending = false
        }
    }

    private fun clearGrant() {
        grantId = null
        grantValidUntilMillis = 0
        _grantSeconds.value = 0
        scope.launch { preferences.saveStoredGrant(null) }
    }

    private fun hasUsableGrant(): Boolean {
        val remaining = _grantSeconds.value
        if (remaining <= 0) return false
        val validUntil = grantValidUntilMillis
        return validUntil <= 0 || validUntil > System.currentTimeMillis()
    }

    private suspend fun persist() {
        preferences.saveGuardSnapshot(_state.value.name, _watchSeconds.value)
        persistGrant()
        persistPending()
    }

    private suspend fun persistGrant() {
        val id = grantId ?: return
        preferences.saveStoredGrant(
            StoredGrant(
                grantId = id,
                minutes = _grantMinutes.value,
                remainingSeconds = _grantSeconds.value.toInt(),
                validUntilMillis = grantValidUntilMillis,
            ),
        )
    }

    private suspend fun persistPending() {
        preferences.savePendingOffline(pending)
    }

    private fun transition(next: GuardState) {
        if (_state.value == next) return
        GuardLog.i("state ${_state.value} -> $next")
        _state.value = next
        scope.launch { persist() }
    }

    companion object {
        private const val PENDING_RETRY_INTERVAL_MS = 5 * 60 * 1000L
        private const val HEARTBEAT_INTERVAL_MS = 30_000L
        private const val LEASE_REFRESH_INTERVAL_MS = 6 * 60 * 60 * 1000L

        fun parseMillis(iso: String?): Long? = runCatching {
            iso?.let { Instant.parse(it).toEpochMilli() }
        }.getOrNull()
    }
}
