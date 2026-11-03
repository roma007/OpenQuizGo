package com.openquizgo.guard.data.watch

import com.google.gson.Gson
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.ApiErrorBody
import com.openquizgo.guard.data.remote.GuardApi
import com.openquizgo.guard.data.remote.HeartbeatRequest
import com.openquizgo.guard.data.remote.HeartbeatResult
import com.openquizgo.guard.data.remote.OfflineEntry
import com.openquizgo.guard.data.remote.OfflineLease
import com.openquizgo.guard.data.remote.OfflineReportRequest
import com.openquizgo.guard.data.remote.OfflineReportResult
import com.openquizgo.guard.data.remote.WatchGrant
import kotlinx.coroutines.flow.first
import retrofit2.Response

class WatchApiException(
    val code: String,
    override val message: String,
    val httpStatus: Int = 0,
) : Exception(message)

/** 授权与计量仓库（P2-T7）。 */
class WatchRepository(
    private val api: GuardApi,
    private val preferences: GuardPreferences,
) {
    private val gson = Gson()

    /** 当前有效授权；无授权时返回 null。 */
    suspend fun activeGrant(): WatchGrant? {
        val response = api.activeGrant(authorization())
        if (response.code() == 404) return null
        return bodyOrThrow(response)
    }

    suspend fun heartbeat(
        grantId: String,
        foregroundApp: String?,
        paused: Boolean,
    ): HeartbeatResult = bodyOrThrow(
        api.heartbeat(
            authorization(),
            grantId,
            HeartbeatRequest(
                foregroundApp = foregroundApp,
                paused = paused,
                positionSeconds = 0,
            ),
        ),
    )

    /** 离线租约；规则不允许时返回 null。 */
    suspend fun offlineLease(): OfflineLease? {
        val response = api.offlineLease(authorization())
        if (response.code() == 403 || response.code() == 404) return null
        return bodyOrThrow(response)
    }

    suspend fun reportOffline(
        leaseId: String,
        entries: List<OfflineEntry>,
    ): OfflineReportResult = bodyOrThrow(
        api.reportOfflineUsage(
            authorization(),
            OfflineReportRequest(leaseId = leaseId, entries = entries),
        ),
    )

    private suspend fun authorization(): String {
        val token = preferences.deviceToken.first()
        if (token.isNullOrBlank()) {
            throw WatchApiException("AUTH_REQUIRED", "设备未激活，请先完成激活绑定", 401)
        }
        return "Bearer $token"
    }

    private fun <T> bodyOrThrow(response: Response<T>): T {
        if (response.isSuccessful) {
            return response.body()
                ?: throw WatchApiException("EMPTY_RESPONSE", "服务端返回为空", response.code())
        }
        throw parseError(response)
    }

    private fun parseError(response: Response<*>): WatchApiException {
        val raw = runCatching { response.errorBody()?.string() }.getOrNull()
        val body = raw?.takeIf { it.isNotBlank() }?.let {
            runCatching { gson.fromJson(it, ApiErrorBody::class.java) }.getOrNull()
        }
        val fallback = when (response.code()) {
            401 -> "设备未授权，请重新激活"
            403 -> "当前不可观看"
            404 -> "授权不存在"
            429 -> "操作过于频繁"
            else -> "网络请求失败（HTTP ${response.code()}）"
        }
        return WatchApiException(
            code = body?.code ?: "HTTP_${response.code()}",
            message = body?.message?.takeIf { it.isNotBlank() } ?: fallback,
            httpStatus = response.code(),
        )
    }
}
