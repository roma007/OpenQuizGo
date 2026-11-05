package com.openquizgo.guard.data.remote

import com.google.gson.annotations.SerializedName
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path

data class MetaResponse(
    @SerializedName("service") val service: String,
    @SerializedName("version") val version: String,
    @SerializedName("env") val env: String,
)

interface GuardApi {
    @GET("v1/meta")
    suspend fun meta(): MetaResponse

    @POST("v1/device/activate/code")
    suspend fun requestActivationCode(@Body request: ActivationCodeRequest): ActivationCodeResponse

    @POST("v1/device/token")
    suspend fun exchangeDeviceToken(@Body request: DeviceTokenRequest): Response<DeviceTokenResponse>

    @GET("v1/device/me")
    suspend fun deviceMe(@Header("Authorization") authorization: String): DeviceMeResponse

    @POST("v1/quiz/sessions")
    suspend fun createQuizSession(
        @Header("Authorization") authorization: String,
    ): Response<CreateSessionResponse>

    @GET("v1/quiz/sessions/{sessionID}")
    suspend fun quizSession(
        @Header("Authorization") authorization: String,
        @Path("sessionID") sessionId: String,
    ): Response<SessionSummary>

    @POST("v1/quiz/sessions/{sessionID}/answers")
    suspend fun submitQuizAnswer(
        @Header("Authorization") authorization: String,
        @Path("sessionID") sessionId: String,
        @Body request: AnswerSubmission,
    ): Response<AnswerResult>

    @POST("v1/quiz/sessions/{sessionID}/finish")
    suspend fun finishQuizSession(
        @Header("Authorization") authorization: String,
        @Path("sessionID") sessionId: String,
    ): Response<SessionSummary>

    @GET("v1/watch/grants/active")
    suspend fun activeGrant(
        @Header("Authorization") authorization: String,
    ): Response<WatchGrant>

    @POST("v1/watch/grants/{grantID}/heartbeat")
    suspend fun heartbeat(
        @Header("Authorization") authorization: String,
        @Path("grantID") grantId: String,
        @Body request: HeartbeatRequest,
    ): Response<HeartbeatResult>

    @GET("v1/offline/lease")
    suspend fun offlineLease(
        @Header("Authorization") authorization: String,
    ): Response<OfflineLease>

    @POST("v1/watch/logs")
    suspend fun reportOfflineUsage(
        @Header("Authorization") authorization: String,
        @Body request: OfflineReportRequest,
    ): Response<OfflineReportResult>

    @PUT("v1/device/capability")
    suspend fun reportCapability(
        @Header("Authorization") authorization: String,
        @Body request: CapabilityReport,
    ): Response<Unit>

    @POST("v1/device/tamper")
    suspend fun reportTamper(
        @Header("Authorization") authorization: String,
        @Body request: TamperReport,
    ): Response<Unit>

    @POST("v1/device/diagnostics")
    suspend fun uploadDiagnostics(
        @Header("Authorization") authorization: String,
        @Body request: DiagnosticUploadRequest,
    ): Response<Unit>

    @POST("v1/device/unlock")
    suspend fun deviceUnlock(
        @Header("Authorization") authorization: String,
        @Body request: DeviceUnlockRequest,
    ): Response<WatchGrant>
}

data class DeviceUnlockRequest(
    @SerializedName("pin") val pin: String,
    @SerializedName("minutes") val minutes: Int = 10,
)

data class DiagnosticUploadRequest(
    @SerializedName("content") val content: String,
    @SerializedName("note") val note: String = "",
)

data class ActivationCodeRequest(
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("platform") val platform: String = "tv",
)

data class ActivationCodeResponse(
    @SerializedName("code") val code: String,
    @SerializedName("expires_in") val expiresIn: Int,
)

data class DeviceTokenRequest(
    @SerializedName("code") val code: String,
    @SerializedName("device_id") val deviceId: String,
)

data class DeviceTokenResponse(
    @SerializedName("device_id") val deviceId: String?,
    @SerializedName("child_id") val childId: String?,
    @SerializedName("child_name") val childName: String?,
    @SerializedName("device_token") val deviceToken: String?,
    @SerializedName("expires_at") val expiresAt: String?,
)

data class DeviceMeResponse(
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("child_id") val childId: String,
    @SerializedName("child_name") val childName: String,
)

data class QuizOption(
    @SerializedName("id") val id: String,
    @SerializedName("text") val text: String,
    @SerializedName("image") val image: String?,
    @SerializedName("images") val images: List<String>? = null,
    @SerializedName("audio") val audio: String? = null,
)

data class StemMedia(
    @SerializedName("url") val url: String?,
)

data class SessionQuestion(
    @SerializedName("item_id") val itemId: String?,
    @SerializedName("question_id") val questionId: String?,
    @SerializedName("position") val position: Int,
    @SerializedName("type") val type: String,
    @SerializedName("stem") val stem: String,
    @SerializedName("stem_media") val stemMedia: StemMedia?,
    @SerializedName("options") val options: List<QuizOption>?,
    @SerializedName("token") val token: String,
    @SerializedName("answered") val answered: Boolean = false,
)

data class CreateSessionResponse(
    @SerializedName("session_id") val sessionId: String,
    @SerializedName("expires_at") val expiresAt: String?,
    @SerializedName("pass_threshold") val passThreshold: Double,
    @SerializedName("rule_version") val ruleVersion: Int,
    @SerializedName("questions") val questions: List<SessionQuestion>?,
)

data class AnswerSubmission(
    @SerializedName("token") val token: String,
    @SerializedName("answer") val answer: List<String>,
)

data class AnswerResult(
    @SerializedName("correct") val correct: Boolean,
    @SerializedName("correct_answer") val correctAnswer: List<String>?,
    @SerializedName("explain") val explain: String?,
    @SerializedName("duration_ms") val durationMs: Int,
    @SerializedName("suspicious") val suspicious: Boolean,
    @SerializedName("score") val score: Double,
    @SerializedName("threshold") val threshold: Double,
    @SerializedName("answered") val answered: Int,
    @SerializedName("total") val total: Int,
    @SerializedName("remaining") val remaining: Int,
)

data class QuizGrant(
    @SerializedName("grant_id") val grantId: String?,
    @SerializedName("minutes") val minutes: Int,
    @SerializedName("valid_until") val validUntil: String?,
    @SerializedName("remaining_seconds") val remainingSeconds: Int?,
    @SerializedName("lease_token") val leaseToken: String?,
)

data class SessionSummary(
    @SerializedName("session_id") val sessionId: String,
    @SerializedName("state") val state: String,
    @SerializedName("score") val score: Double,
    @SerializedName("pass_threshold") val passThreshold: Double,
    @SerializedName("correct_count") val correctCount: Int,
    @SerializedName("answered_count") val answeredCount: Int,
    @SerializedName("question_count") val questionCount: Int,
    @SerializedName("expires_at") val expiresAt: String?,
    @SerializedName("questions") val questions: List<SessionQuestion>?,
    @SerializedName("grant") val grant: QuizGrant?,
    @SerializedName("grant_error") val grantError: String?,
)

data class ApiErrorBody(
    @SerializedName("code") val code: String?,
    @SerializedName("message") val message: String?,
    @SerializedName("session_id") val sessionId: String?,
)

data class WatchGrant(
    @SerializedName("grant_id") val grantId: String,
    @SerializedName("session_id") val sessionId: String?,
    @SerializedName("minutes") val minutes: Int,
    @SerializedName("valid_from") val validFrom: String?,
    @SerializedName("valid_until") val validUntil: String?,
    @SerializedName("remaining_seconds") val remainingSeconds: Int,
    @SerializedName("status") val status: String,
    @SerializedName("lease_token") val leaseToken: String?,
)

data class HeartbeatRequest(
    @SerializedName("foreground_app") val foregroundApp: String?,
    @SerializedName("paused") val paused: Boolean,
    @SerializedName("position_seconds") val positionSeconds: Int = 0,
)

data class HeartbeatResult(
    @SerializedName("grant_id") val grantId: String?,
    @SerializedName("status") val status: String?,
    @SerializedName("consumed_seconds") val consumedSeconds: Int?,
    @SerializedName("remaining_seconds") val remainingSeconds: Int?,
    @SerializedName("paused") val paused: Boolean?,
    @SerializedName("revoked") val revoked: Boolean?,
    @SerializedName("reason") val reason: String?,
    @SerializedName("server_time") val serverTime: String?,
)

data class OfflineLease(
    @SerializedName("lease_id") val leaseId: String,
    @SerializedName("lease_token") val leaseToken: String?,
    @SerializedName("public_key") val publicKey: String?,
    @SerializedName("max_minutes") val maxMinutes: Int,
    @SerializedName("allowed_apps") val allowedApps: List<String>?,
    @SerializedName("issued_at") val issuedAt: String?,
    @SerializedName("expires_at") val expiresAt: String?,
)

data class OfflineEntry(
    @SerializedName("start_at") val startAt: String,
    @SerializedName("end_at") val endAt: String,
    @SerializedName("watched_seconds") val watchedSeconds: Int,
    @SerializedName("paused_seconds") val pausedSeconds: Int,
    @SerializedName("target_app") val targetApp: String?,
)

data class OfflineReportRequest(
    @SerializedName("lease_id") val leaseId: String,
    @SerializedName("entries") val entries: List<OfflineEntry>,
)

data class OfflineReportResult(
    @SerializedName("reported_minutes") val reportedMinutes: Int,
    @SerializedName("remaining_minutes") val remainingMinutes: Int,
    @SerializedName("clamped") val clamped: Boolean,
)

data class CapabilityReport(
    @SerializedName("guard_mode") val guardMode: String = "standard",
    @SerializedName("permissions") val permissions: Map<String, Boolean>,
    @SerializedName("os_version") val osVersion: String,
    @SerializedName("app_version") val appVersion: String,
)

data class TamperReport(
    @SerializedName("type") val type: String,
    @SerializedName("detail") val detail: Map<String, String?>,
)
