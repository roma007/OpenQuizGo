package com.openquizgo.guard.ui

import android.os.Build
import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import androidx.lifecycle.viewModelScope
import com.google.gson.Gson
import com.openquizgo.guard.data.foreground.ForegroundDetector
import com.openquizgo.guard.data.foreground.ForegroundState
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.ApiErrorBody
import com.openquizgo.guard.data.remote.GuardApi
import com.openquizgo.guard.data.remote.SmsCodeRequest
import com.openquizgo.guard.data.remote.TvDeviceResponse
import com.openquizgo.guard.data.remote.TvLoginRequest
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
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import retrofit2.Response

sealed interface GuardUiState {
    data object Loading : GuardUiState

    /** 081：电视端不再用激活码，未登录时展示手机号 + 验证码登录页。 */
    data object NeedsLogin : GuardUiState

    data class Ready(val device: TvDeviceResponse) : GuardUiState
    data class Failure(val message: String) : GuardUiState
}

data class LoginFormState(
    val phone: String = "",
    val code: String = "",
    val sending: Boolean = false,
    val loggingIn: Boolean = false,
    val resendCountdown: Int = 0,
    val message: String? = null,
    val error: Boolean = false,
)

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
    /** 084：`tv` / `pad`，登录时上报给服务端。 */
    private val platform: String = "tv",
) : ViewModel() {

    private val _state = MutableStateFlow<GuardUiState>(GuardUiState.Loading)
    val state: StateFlow<GuardUiState> = _state.asStateFlow()
    val foregroundState: StateFlow<ForegroundState> = foregroundDetector.state
    val guardState: StateFlow<GuardState> = guardEngine.state
    val watchSeconds: StateFlow<Long> = guardEngine.watchSeconds
    val grantSeconds: StateFlow<Long> = guardEngine.grantSeconds

    /** 081：今日额度、硬停与下次答题时间（服务端准）。 */
    val contextRemaining: StateFlow<Int?> = guardEngine.contextRemainingMinutes
    val contextUsed: StateFlow<Int?> = guardEngine.contextUsedMinutes
    val contextLimit: StateFlow<Int?> = guardEngine.contextDailyLimit
    val contextReason: StateFlow<String?> = guardEngine.contextReason
    val hardStop: StateFlow<Boolean> = guardEngine.hardStop
    val hardStopGrace: StateFlow<Int> = guardEngine.hardStopGrace
    val nextQuizAtMillis: StateFlow<Long?> = guardEngine.nextQuizAtMillis

    private val _login = MutableStateFlow(LoginFormState())
    val login: StateFlow<LoginFormState> = _login.asStateFlow()

    val logs: StateFlow<List<String>> = GuardLog.recent

    private val _serverStatus = MutableStateFlow(ServerStatus())
    val serverStatus: StateFlow<ServerStatus> = _serverStatus.asStateFlow()

    private val _reportStatus = MutableStateFlow<String?>(null)
    val reportStatus: StateFlow<String?> = _reportStatus.asStateFlow()

    private var countdownJob: Job? = null
    private val gson = Gson()

    init {
        bootstrap()
    }

    /* ------------------------------ 登录 ------------------------------ */

    fun updatePhone(value: String) {
        val digits = value.filter { it.isDigit() }.take(11)
        _login.update { it.copy(phone = digits, error = false) }
    }

    fun updateCode(value: String) {
        val digits = value.filter { it.isDigit() }.take(6)
        _login.update { it.copy(code = digits, error = false) }
    }

    fun sendCode() {
        val form = _login.value
        if (form.sending || form.resendCountdown > 0) return
        if (!PHONE_PATTERN.matches(form.phone)) {
            _login.update { it.copy(message = "请输入 11 位手机号", error = true) }
            return
        }
        viewModelScope.launch {
            _login.update { it.copy(sending = true, message = "正在发送验证码…", error = false) }
            val result = runCatching { api.requestSmsCode(SmsCodeRequest(form.phone)) }
            result.onSuccess { response ->
                if (response.isSuccessful) {
                    GuardLog.i("login", "sms code sent")
                    _login.update {
                        it.copy(sending = false, message = "验证码已发送，请查看手机短信", error = false)
                    }
                    startResendCountdown()
                } else {
                    _login.update {
                        it.copy(sending = false, message = errorMessage(response), error = true)
                    }
                }
            }.onFailure { error ->
                GuardLog.w("login", "send sms failed: ${error.message}")
                _login.update {
                    it.copy(sending = false, message = error.message ?: "发送失败，请重试", error = true)
                }
            }
        }
    }

    fun login() {
        val form = _login.value
        if (form.loggingIn) return
        if (!PHONE_PATTERN.matches(form.phone)) {
            _login.update { it.copy(message = "请输入 11 位手机号", error = true) }
            return
        }
        if (form.code.length != 6) {
            _login.update { it.copy(message = "请输入 6 位验证码", error = true) }
            return
        }
        viewModelScope.launch {
            _login.update { it.copy(loggingIn = true, message = "正在登录…", error = false) }
            val deviceId = preferences.deviceId()
            val fallbackName = if (platform == "pad") "平板" else "电视"
            val deviceName = Build.MODEL?.takeIf { it.isNotBlank() } ?: fallbackName
            runCatching {
                api.tvLogin(
                    TvLoginRequest(
                        phone = form.phone,
                        code = form.code,
                        deviceId = deviceId,
                        deviceName = deviceName,
                        platform = platform,
                    ),
                )
            }.onSuccess { response ->
                val body = response.body()
                val token = body?.deviceToken
                if (response.isSuccessful && !token.isNullOrBlank()) {
                    val device = body.device ?: TvDeviceResponse()
                    GuardLog.i("login", "tv login ok device=${device.id}")
                    preferences.saveBinding(token, device.childId, device.childName)
                    _login.value = LoginFormState(phone = form.phone)
                    _state.value = GuardUiState.Ready(device)
                } else {
                    _login.update {
                        it.copy(loggingIn = false, message = errorMessage(response), error = true)
                    }
                }
            }.onFailure { error ->
                GuardLog.w("login", "tv login failed: ${error.message}")
                _login.update {
                    it.copy(loggingIn = false, message = error.message ?: "登录失败，请重试", error = true)
                }
            }
        }
    }

    /** 退出登录：只清本地令牌，设备仍留在家长端列表里，可再次登录。 */
    fun logout() {
        viewModelScope.launch {
            GuardLog.i("login", "logout requested")
            preferences.clearBinding()
            _login.value = LoginFormState()
            _state.value = GuardUiState.NeedsLogin
        }
    }

    private fun startResendCountdown() {
        countdownJob?.cancel()
        countdownJob = viewModelScope.launch {
            var remaining = RESEND_SECONDS
            while (isActive && remaining > 0) {
                _login.update { it.copy(resendCountdown = remaining) }
                delay(1_000)
                remaining -= 1
            }
            _login.update { it.copy(resendCountdown = 0) }
        }
    }

    /** 平板「重新连接」按钮：重新走一次登录态检查。 */
    fun retryBootstrap() {
        if (_state.value == GuardUiState.Loading) return
        _state.value = GuardUiState.Loading
        bootstrap()
    }

    private fun bootstrap() {
        viewModelScope.launch {
            val token = preferences.deviceToken.first()
            if (token.isNullOrBlank()) {
                GuardLog.i("auth", "no device token, show login")
                _state.value = GuardUiState.NeedsLogin
                return@launch
            }
            _state.value = GuardUiState.Loading
            runCatching { api.tvMe("Bearer $token") }
                .onSuccess { response ->
                    val device = response.body()
                    if (response.isSuccessful && device != null) {
                        GuardLog.i("auth", "token ok device=${device.id}")
                        preferences.saveBinding(token, device.childId, device.childName)
                        _state.value = GuardUiState.Ready(device)
                        return@onSuccess
                    }
                    if (response.code() == 401 || response.code() == 403) {
                        GuardLog.w("auth", "token rejected HTTP ${response.code()}, show login")
                        preferences.clearBinding()
                        _state.value = GuardUiState.NeedsLogin
                    } else {
                        _state.value = GuardUiState.Failure(errorMessage(response))
                    }
                }
                .onFailure { error ->
                    // 网络异常保留本地登录态，离线仍按已缓存授权运行。
                    GuardLog.w("auth", "tv/me failed: ${error.message}")
                    val cachedName = preferences.childName.first()
                    _state.value = GuardUiState.Ready(
                        TvDeviceResponse(
                            name = if (platform == "pad") "平板" else "电视",
                            childName = cachedName,
                            platform = platform,
                        ),
                    )
                }
        }
    }

    private fun errorMessage(response: Response<*>): String {
        val raw = runCatching { response.errorBody()?.string() }.getOrNull()
        val body = raw?.takeIf { it.isNotBlank() }?.let {
            runCatching { gson.fromJson(it, ApiErrorBody::class.java) }.getOrNull()
        }
        body?.message?.takeIf { it.isNotBlank() }?.let { return it }
        return when (response.code()) {
            401 -> "验证码错误或已失效"
            403 -> "该设备已被家长解绑，请重新登录"
            429 -> "操作过于频繁，请稍后再试"
            else -> "请求失败（HTTP ${response.code()}）"
        }
    }

    /* ------------------------------ 诊断 ------------------------------ */

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

    fun uploadDiagnostics() {
        viewModelScope.launch {
            _reportStatus.value = "上传日志中…"
            val result = diagnostics.uploadLogs()
            _reportStatus.value = if (result.isSuccess) {
                "诊断日志已上传"
            } else {
                "上传失败：${result.exceptionOrNull()?.message}"
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
                container.platform,
            ) as T
        }
    }

    private companion object {
        val PHONE_PATTERN = Regex("^1[3-9]\\d{9}$")
        const val RESEND_SECONDS = 60
    }
}
