package com.openquizgo.guard.data.watch

/** 本地保存的授权快照（用于重启/断网续算）。 */
data class StoredGrant(
    val grantId: String,
    val minutes: Int,
    val remainingSeconds: Int,
    val validUntilMillis: Long,
)

/** 本地保存的离线租约。 */
data class StoredLease(
    val leaseId: String,
    val maxMinutes: Int,
    val expiresAtMillis: Long,
    val allowedApps: List<String> = emptyList(),
)

/** 断网期间累计、待补报的观看记录。 */
data class PendingOffline(
    val leaseId: String?,
    val startMillis: Long,
    val endMillis: Long,
    val watchedSeconds: Int,
    val pausedSeconds: Int,
    val targetApp: String?,
)
