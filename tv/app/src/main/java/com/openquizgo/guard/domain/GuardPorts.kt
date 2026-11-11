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
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow

/**
 * 守护状态机所需的本地存储端口（075）。
 *
 * 仅为可测性抽取，生产实现仍为 [com.openquizgo.guard.data.local.GuardPreferences]。
 */
interface GuardStorage {
    val deviceToken: Flow<String?>

    suspend fun guardSnapshot(): Pair<String, Long>

    suspend fun saveGuardSnapshot(state: String, watchSeconds: Long)

    suspend fun storedGrant(): StoredGrant?

    suspend fun saveStoredGrant(grant: StoredGrant?)

    suspend fun storedLease(): StoredLease?

    suspend fun saveStoredLease(lease: StoredLease?)

    suspend fun pendingOffline(): PendingOffline?

    suspend fun savePendingOffline(pending: PendingOffline?)
}

/** 前台与媒体播放状态端口。 */
interface ForegroundSource {
    val state: StateFlow<ForegroundState>
}

/** 授权、心跳与离线补报端口。 */
interface WatchGateway {
    suspend fun activeGrant(): WatchGrant?

    suspend fun heartbeat(
        grantId: String,
        foregroundApp: String?,
        paused: Boolean,
    ): HeartbeatResult

    suspend fun offlineLease(): OfflineLease?

    suspend fun reportOffline(
        leaseId: String,
        entries: List<OfflineEntry>,
    ): OfflineReportResult
}
