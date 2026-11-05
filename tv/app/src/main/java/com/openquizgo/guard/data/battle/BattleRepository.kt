package com.openquizgo.guard.data.battle

import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.BattleAnswer
import com.openquizgo.guard.data.remote.BattleSubmitRequest
import com.openquizgo.guard.data.remote.BattleJoinRequest
import com.openquizgo.guard.data.remote.BattleState
import com.openquizgo.guard.data.remote.GuardApi
import kotlinx.coroutines.flow.first
import retrofit2.Response

class BattleRepository(
    private val api: GuardApi,
    private val preferences: GuardPreferences,
) {
    suspend fun join(code: String): BattleState =
        bodyOrThrow(api.joinBattle(authorization(), BattleJoinRequest(code)))

    suspend fun state(battleId: String): BattleState =
        bodyOrThrow(api.battleState(authorization(), battleId))

    suspend fun submit(battleId: String, answers: List<BattleAnswer>): BattleState =
        bodyOrThrow(api.submitBattleAnswers(authorization(), battleId, BattleSubmitRequest(answers)))

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
