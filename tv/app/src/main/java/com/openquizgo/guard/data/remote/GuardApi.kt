package com.openquizgo.guard.data.remote

import com.google.gson.annotations.SerializedName
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST
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
}

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
)

data class SessionQuestion(
    @SerializedName("item_id") val itemId: String?,
    @SerializedName("question_id") val questionId: String?,
    @SerializedName("position") val position: Int,
    @SerializedName("type") val type: String,
    @SerializedName("stem") val stem: String,
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
