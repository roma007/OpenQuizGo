package com.openquizgo.guard.data.remote

import com.google.gson.annotations.SerializedName
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.Header
import retrofit2.http.POST

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
