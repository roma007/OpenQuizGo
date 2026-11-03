package com.openquizgo.guard.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import com.openquizgo.guard.data.foreground.ForegroundDetector
import com.openquizgo.guard.data.foreground.ForegroundState
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.ActivationCodeRequest
import com.openquizgo.guard.data.remote.DeviceTokenRequest
import com.openquizgo.guard.data.remote.GuardApi
import com.openquizgo.guard.domain.GuardEngine
import com.openquizgo.guard.domain.GuardState
import com.openquizgo.guard.di.AppContainer
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import retrofit2.HttpException

sealed interface GuardUiState {
    data object Loading : GuardUiState
    data class Activation(
        val code: String,
        val expiresInSeconds: Int,
        val deviceId: String,
        val expired: Boolean = false,
    ) : GuardUiState
    data class Bound(val childName: String) : GuardUiState
    data class Failure(val message: String) : GuardUiState
}

class GuardViewModel(
    private val preferences: GuardPreferences,
    private val api: GuardApi,
    foregroundDetector: ForegroundDetector,
    guardEngine: GuardEngine,
) : ViewModel() {

    private val _state = MutableStateFlow<GuardUiState>(GuardUiState.Loading)
    val state: StateFlow<GuardUiState> = _state.asStateFlow()
    val foregroundState: StateFlow<ForegroundState> = foregroundDetector.state
    val guardState: StateFlow<GuardState> = guardEngine.state
    val watchSeconds: StateFlow<Long> = guardEngine.watchSeconds
    val grantSeconds: StateFlow<Long> = guardEngine.grantSeconds

    private var pollJob: Job? = null

    init {
        bootstrap()
    }

    fun regenerate() {
        startActivation()
    }

    fun resetBinding() {
        viewModelScope.launch {
            pollJob?.cancel()
            preferences.clearBinding()
            startActivation()
        }
    }

    private fun bootstrap() {
        viewModelScope.launch {
            val token = preferences.deviceToken.first()
            if (token.isNullOrBlank()) {
                startActivation()
                return@launch
            }
            val cachedName = preferences.childName.first()
            _state.value = GuardUiState.Bound(cachedName ?: "已绑定")
            runCatching { api.deviceMe("Bearer $token") }
                .onSuccess {
                    preferences.saveBinding(token, it.childId, it.childName)
                    _state.value = GuardUiState.Bound(it.childName)
                }
                .onFailure { error ->
                    // 仅令牌失效时清除；网络异常保留本地绑定，离线可用。
                    if (error is HttpException && (error.code() == 401 || error.code() == 403)) {
                        preferences.clearBinding()
                        startActivation()
                    }
                }
        }
    }

    private fun startActivation() {
        pollJob?.cancel()
        viewModelScope.launch {
            _state.value = GuardUiState.Loading
            runCatching {
                val deviceId = preferences.deviceId()
                val code = api.requestActivationCode(ActivationCodeRequest(deviceId))
                code to deviceId
            }.onSuccess { (code, deviceId) ->
                _state.value = GuardUiState.Activation(
                    code = code.code,
                    expiresInSeconds = code.expiresIn,
                    deviceId = deviceId,
                )
                pollActivation(code.code, deviceId, code.expiresIn)
            }.onFailure {
                _state.value = GuardUiState.Failure(it.message ?: "请求激活码失败")
            }
        }
    }

    private fun pollActivation(code: String, deviceId: String, expiresInSeconds: Int) {
        pollJob = viewModelScope.launch {
            val deadline = System.currentTimeMillis() + expiresInSeconds * 1000L
            while (isActive) {
                if (System.currentTimeMillis() > deadline) {
                    val current = _state.value
                    if (current is GuardUiState.Activation) {
                        _state.value = current.copy(expired = true)
                    }
                    return@launch
                }
                delay(2000)
                val response = runCatching {
                    api.exchangeDeviceToken(DeviceTokenRequest(code, deviceId))
                }.getOrElse { continue }

                if (response.code() == 202) continue
                if (response.code() == 410 || response.code() == 401) {
                    _state.value = GuardUiState.Failure("激活码已失效，请重新生成")
                    return@launch
                }
                if (response.isSuccessful) {
                    val body = response.body()
                    val token = body?.deviceToken
                    if (!token.isNullOrBlank()) {
                        preferences.saveBinding(token, body.childId, body.childName)
                        _state.value = GuardUiState.Bound(body.childName ?: "孩子")
                        return@launch
                    }
                }
            }
        }
    }

    class Factory(private val container: AppContainer) : ViewModelProvider.Factory {
        @Suppress("UNCHECKED_CAST")
        override fun <T : ViewModel> create(modelClass: Class<T>): T {
            return GuardViewModel(
                container.preferences,
                container.api,
                container.foregroundDetector,
                container.guardEngine,
            ) as T
        }
    }
}
