package com.openquizgo.guard.data.battle

import okhttp3.ResponseBody
import retrofit2.Response
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.Path
import retrofit2.http.Streaming

/** 使用独立 OkHttp 客户端（无读超时）承载对局长连接。 */
interface BattleEventApi {
    @Streaming
    @GET("v1/device/battles/{battleID}/events")
    suspend fun events(
        @Header("Authorization") authorization: String,
        @Path("battleID") battleId: String,
    ): Response<ResponseBody>
}
