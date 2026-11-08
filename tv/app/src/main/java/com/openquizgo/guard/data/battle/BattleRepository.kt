package com.openquizgo.guard.data.battle

import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.BattleAnswer
import com.openquizgo.guard.data.remote.BattleJoinRequest
import com.openquizgo.guard.data.remote.BattleState
import com.openquizgo.guard.data.remote.BattleSubmitRequest
import com.openquizgo.guard.data.remote.GuardApi
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.currentCoroutineContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.isActive
import org.json.JSONObject
import retrofit2.Response

class BattleRepository(
    private val api: GuardApi,
    private val eventsApi: BattleEventApi,
    private val preferences: GuardPreferences,
) {
    suspend fun join(code: String): BattleState =
        bodyOrThrow(api.joinBattle(authorization(), BattleJoinRequest(code)))

    suspend fun state(battleId: String): BattleState =
        bodyOrThrow(api.battleState(authorization(), battleId))

    suspend fun submit(battleId: String, answers: List<BattleAnswer>): BattleState =
        bodyOrThrow(api.submitBattleAnswers(authorization(), battleId, BattleSubmitRequest(answers)))

    /**
     * 订阅对局 SSE，按服务端下发的单调版本号发出事件。
     * 连接结束或异常由调用方负责重连；这里不吞掉异常，便于上层降级轮询。
     */
    fun events(battleId: String): Flow<Long> = flow {
        val response = eventsApi.events(authorization(), battleId)
        if (!response.isSuccessful) {
            throw IllegalStateException("事件流失败（${response.code()}）")
        }
        val body = response.body() ?: throw IllegalStateException("事件流为空")
        body.use {
            val source = it.source()
            val data = StringBuilder()
            while (currentCoroutineContext().isActive) {
                val line = source.readUtf8Line() ?: break
                when {
                    line.isEmpty() -> {
                        if (data.isNotEmpty()) {
                            val version = runCatching {
                                JSONObject(data.toString()).optLong("version", 0L)
                            }.getOrDefault(0L)
                            data.clear()
                            if (version > 0) emit(version)
                        }
                    }
                    line.startsWith("data:") -> data.append(line.removePrefix("data:").trim())
                }
            }
        }
    }.flowOn(Dispatchers.IO)

    private suspend fun authorization(): String {
        val token = preferences.deviceToken.first()
        if (token.isNullOrBlank()) throw IllegalStateException("设备未激活")
        return "Bearer $token"
    }

    private fun <T> bodyOrThrow(response: Response<T>): T {
        if (!response.isSuccessful) {
            throw IllegalStateException("对战请求失败（${response.code()}）")
        }
        return response.body() ?: throw IllegalStateException("对战响应为空")
    }
}
