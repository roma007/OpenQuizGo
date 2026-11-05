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
import com.openquizgo.guard.platform.DiagnosticsReporter
import com.openquizgo.guard.platform.GuardLog
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

data class ServerStatus(
    val checking: Boolean = false,
    val ok: Boolean? = null,
    val message: String = "未检测",
)

class GuardViewModel(
    private val preferences: GuardPreferences,
    private val api: GuardApi,
    foregroundDetector: ForegroundDetector,
    guardEngine: GuardEngine,
    private val diagnostics: DiagnosticsReporter,
) : ViewModel() {

    private val _state = MutableStateFlow<GuardUiState>(GuardUiState.Loading)
    val state: StateFlow<GuardUiState> = _state.asStateFlow()
    val foregroundState: StateFlow<ForegroundState> = foregroundDetector.state
    val guardState: StateFlow<GuardState> = guardEngine.state
    val watchSeconds: StateFlow<Long> = guardEngine.watchSeconds
    val grantSeconds: StateFlow<Long> = guardEngine.grantSeconds
    val logs: StateFlow<List<String>> = GuardLog.recent

    private val _serverStatus = MutableStateFlow(ServerStatus())
    val serverStatus: StateFlow<ServerStatus> = _serverStatus.asStateFlow()

    private val _reportStatus = MutableStateFlow<String?>(null)
    val reportStatus: StateFlow<String?> = _reportStatus.asStateFlow()

    private var pollJob: Job? = null

    init {
        bootstrap()
    }

    fun regenerate() {
        startActivation()
    }

    fun pingServer() {
        if (_serverStatus.value.checking) return
        _serverStatus.value = ServerStatus(checking = true, message = "检测中…")
        viewModelScope.launch {
            runCatching { api.meta() }
                .onSuccess {
                    _serverStatus.value = ServerStatus(
                        ok = true,
                        message = "正常 · ${it.service} ${it.version}",
                    )
                }
                .onFailure {
                    _serverStatus.value = ServerStatus(
                        ok = false,
                        message = it.message ?: "连接失败",
                    )
                }
        }
    }

    fun reportCapabilityNow() {
        viewModelScope.launch {
            _reportStatus.value = "上报中…"
            val result = diagnostics.reportCapabilityNow()
            _reportStatus.value = if (result.isSuccess) "能力已上报" else "上报失败：${result.exceptionOrNull()?.message}"
        }
    }

    fun resetBinding() {
        viewModelScope.launch {
            pollJob?.cancel()
            GuardLog.i("binding", "binding reset by user")
            preferences.clearBinding()
            startActivation()
        }
    }

    private fun bootstrap() {
        viewModelScope.launch {
            val token = preferences.deviceToken.first()
            if (token.isNullOrBlank()) {
                GuardLog.i("auth", "no device token, start activation")
                startActivation()
                return@launch
            }
            val cachedName = preferences.childName.first()
            _state.value = GuardUiState.Bound(cachedName ?: "已绑定")
            runCatching { api.deviceMe("Bearer $token") }
                .onSuccess {
                    GuardLog.i("auth", "token refresh ok")
                    preferences.saveBinding(token, it.childId, it.childName)
                    _state.value = GuardUiState.Bound(it.childName)
                }
                .onFailure { error ->
                    // 仅令牌失效时清除；网络异常保留本地绑定，离线可用。
                    if (error is HttpException && (error.code() == 401 || error.code() == 403)) {
                        GuardLog.w("auth", "token rejected HTTP ${error.code()}, re-activate")
                        preferences.clearBinding()
                        startActivation()
                    } else {
                        GuardLog.w("auth", "token refresh failed: ${error.message}")
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
                GuardLog.i("activation", "code issued expires=${code.expiresIn}s")
                _state.value = GuardUiState.Activation(
                    code = code.code,
                    expiresInSeconds = code.expiresIn,
                    deviceId = deviceId,
                )
                pollActivation(code.code, deviceId, code.expiresIn)
            }.onFailure {
                GuardLog.e("activation", "request code failed", it)
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
                    GuardLog.w("activation", "code expired or rejected HTTP ${response.code()}")
                    _state.value = GuardUiState.Failure("激活码已失效，请重新生成")
                    return@launch
                }
                if (response.isSuccessful) {
                    val body = response.body()
                    val token = body?.deviceToken
                    if (!token.isNullOrBlank()) {
                        GuardLog.i("binding", "device bound")
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
                container.diagnosticsReporter,
            ) as T
        }
    }
}
