package com.openquizgo.guard.data.remote

import com.google.gson.annotations.SerializedName
import retrofit2.http.GET

data class MetaResponse(
    @SerializedName("service") val service: String,
    @SerializedName("version") val version: String,
    @SerializedName("env") val env: String,
)

interface GuardApi {
    @GET("v1/meta")
    suspend fun meta(): MetaResponse
}
