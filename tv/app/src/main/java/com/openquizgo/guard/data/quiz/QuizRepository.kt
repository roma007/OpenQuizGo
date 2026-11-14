package com.openquizgo.guard.data.quiz

import com.google.gson.Gson
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.AnswerResult
import com.openquizgo.guard.data.remote.AnswerSubmission
import com.openquizgo.guard.data.remote.ApiErrorBody
import com.openquizgo.guard.data.remote.GuardApi
import com.openquizgo.guard.data.remote.SessionQuestion
import com.openquizgo.guard.data.remote.SessionSummary
import kotlinx.coroutines.flow.first
import retrofit2.Response

/** 服务端返回的业务错误（携带 code / request_id / session_id）。 */
class QuizApiException(
    val code: String,
    override val message: String,
    val sessionId: String? = null,
    val httpStatus: Int = 0,
) : Exception(message)

data class QuizSession(
    val sessionId: String,
    val expiresAt: String?,
    val passThreshold: Double,
    val questions: List<SessionQuestion>,
)

data class QuizOutcome(
    val summary: SessionSummary,
    val passed: Boolean,
) {
    val grantMinutes: Int get() = summary.grant?.minutes ?: 0
}

/**
 * 答题会话仓库（P2-T6）。
 *
 * 负责创建会话、断线恢复、提交答案与结算；所有请求使用设备令牌。
 */
class QuizRepository(
    private val api: GuardApi,
    private val preferences: GuardPreferences,
) {
    private val gson = Gson()

    /** 创建会话；若服务端提示已有会话，则尝试恢复该会话。 */
    suspend fun startSession(): QuizSession {
        val authorization = authorization()
        val response = api.createQuizSession(authorization)
        if (response.isSuccessful) {
            val body = response.body()
                ?: throw QuizApiException("EMPTY_RESPONSE", "服务端返回为空", httpStatus = response.code())
            return QuizSession(
                sessionId = body.sessionId,
                expiresAt = body.expiresAt,
                passThreshold = body.passThreshold,
                questions = body.questions.orEmpty(),
            )
        }
        val error = parseError(response)
        if (error.code == "QUIZ_SESSION_ACTIVE" && !error.sessionId.isNullOrBlank()) {
            return resumeSession(error.sessionId)
        }
        throw error
    }

    private suspend fun resumeSession(sessionId: String): QuizSession {
        val summary = requestOrThrow(api.quizSession(authorization(), sessionId))
        if (summary.state != "active") {
            throw QuizApiException(
                code = "QUIZ_SESSION_CLOSED",
                message = "答题会话已结束，请重新发起",
                sessionId = sessionId,
            )
        }
        return QuizSession(
            sessionId = summary.sessionId,
            expiresAt = summary.expiresAt,
            passThreshold = summary.passThreshold,
            questions = summary.questions.orEmpty(),
        )
    }

    suspend fun submitAnswer(
        sessionId: String,
        token: String,
        optionIds: List<String>,
    ): AnswerResult = requestOrThrow(
        api.submitQuizAnswer(
            authorization(),
            sessionId,
            AnswerSubmission(token = token, answer = optionIds),
        ),
    )

    suspend fun finish(sessionId: String): SessionSummary =
        requestOrThrow(api.finishQuizSession(authorization(), sessionId))

    private suspend fun authorization(): String {
        val token = preferences.deviceToken.first()
        if (token.isNullOrBlank()) {
            throw QuizApiException("AUTH_REQUIRED", "设备未激活，请先完成激活绑定", httpStatus = 401)
        }
        return "Bearer $token"
    }

    private fun <T> requestOrThrow(response: Response<T>): T {
        if (response.isSuccessful) {
            return response.body()
                ?: throw QuizApiException("EMPTY_RESPONSE", "服务端返回为空", httpStatus = response.code())
        }
        throw parseError(response)
    }

    private fun parseError(response: Response<*>): QuizApiException {
        val raw = runCatching { response.errorBody()?.string() }.getOrNull()
        val body = raw?.takeIf { it.isNotBlank() }?.let {
            runCatching { gson.fromJson(it, ApiErrorBody::class.java) }.getOrNull()
        }
        val fallback = when (response.code()) {
            401 -> "设备未授权，请重新激活"
            404 -> "请求的资源不存在"
            409 -> "当前状态不允许该操作"
            410 -> "答题已超时，请重新发起"
            429 -> "操作过于频繁，请稍后再试"
            else -> "网络请求失败（HTTP ${response.code()}）"
        }
        return QuizApiException(
            code = body?.code ?: "HTTP_${response.code()}",
            message = body?.message?.takeIf { it.isNotBlank() } ?: fallback,
            sessionId = body?.sessionId,
            httpStatus = response.code(),
        )
    }
}
