package com.openquizgo.guard.platform

import android.content.Context
import android.os.Build
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.CapabilityReport
import com.openquizgo.guard.data.remote.TamperReport
import com.openquizgo.guard.data.watch.WatchRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * 能力与异常上报（P2-T9）。
 *
 * - 定期上报权限/版本能力；
 * - 无障碍、使用情况访问被关闭时上报 permission_revoked；
 * - 供诊断页手动触发。
 */
class DiagnosticsReporter(
    context: Context,
    private val preferences: GuardPreferences,
    private val watchRepository: WatchRepository,
    private val scope: CoroutineScope,
) {
    private val appContext = context.applicationContext

    private var lastAccessibility: Boolean? = null
    private var lastUsage: Boolean? = null
    private var lastNotifications: Boolean? = null
    private var lastReportAt = 0L

    init {
        scope.launch {
            while (isActive) {
                checkPermissions()
                delay(PERMISSION_CHECK_MS)
            }
        }
        scope.launch {
            while (isActive) {
                if (System.currentTimeMillis() - lastReportAt > REPORT_INTERVAL_MS) {
                    reportCapabilityNow()
                }
                delay(30_000)
            }
        }
    }

    suspend fun reportCapabilityNow(): Result<Unit> = runCatching {
        if (preferences.deviceToken.first().isNullOrBlank()) return@runCatching
        watchRepository.reportCapability(buildCapability())
        lastReportAt = System.currentTimeMillis()
        GuardLog.i("capability reported")
    }.onFailure { GuardLog.w("capability report failed: ${it.message}") }

    /** 手动上传最近日志，服务端限制 256KB，超出保留尾部。 */
    suspend fun uploadLogs(): Result<Unit> = runCatching {
        if (preferences.deviceToken.first().isNullOrBlank()) {
            throw IllegalStateException("设备未激活")
        }
        val content = LogExporter.combined(appContext, MAX_UPLOAD_BYTES)
        watchRepository.uploadDiagnostics(content, "manual upload from TV")
        GuardLog.i("diagnostics uploaded bytes=${content.toByteArray().size}")
    }.onFailure { GuardLog.w("diagnostics upload failed: ${it.message}") }

    suspend fun reportTamper(type: String, detail: Map<String, String?>) {
        runCatching {
            if (preferences.deviceToken.first().isNullOrBlank()) return@runCatching
            watchRepository.reportTamper(TamperReport(type = type, detail = detail))
            GuardLog.i("tamper reported type=$type detail=$detail")
        }.onFailure { GuardLog.w("tamper report failed: ${it.message}") }
    }

    fun currentCapability(): CapabilityReport = buildCapability()

    private fun buildCapability(): CapabilityReport {
        val accessibility = Permissions.accessibilityEnabled(appContext)
        val usage = Permissions.usageAccessEnabled(appContext)
        val notifications = Permissions.notificationsEnabled(appContext)
        val service = Permissions.foregroundServiceRunning(appContext)
        return CapabilityReport(
            guardMode = "standard",
            permissions = mapOf(
                "accessibility" to accessibility,
                "usage_stats" to usage,
                "notifications" to notifications,
                "foreground_service" to service,
                "boot_receiver" to true,
            ),
            osVersion = "Android ${Build.VERSION.RELEASE} (SDK ${Build.VERSION.SDK_INT})",
            appVersion = BuildConfig.VERSION_NAME,
        )
    }

    private suspend fun checkPermissions() {
        val accessibility = Permissions.accessibilityEnabled(appContext)
        val usage = Permissions.usageAccessEnabled(appContext)
        val notifications = Permissions.notificationsEnabled(appContext)

        if (lastAccessibility == true && !accessibility) {
            reportTamper("permission_revoked", mapOf("permission" to "accessibility"))
            reportCapabilityNow()
        }
        if (lastUsage == true && !usage) {
            reportTamper("permission_revoked", mapOf("permission" to "usage_stats"))
            reportCapabilityNow()
        }
        if (lastNotifications == true && !notifications) {
            reportTamper("permission_revoked", mapOf("permission" to "notifications"))
            reportCapabilityNow()
        }
        lastAccessibility = accessibility
        lastUsage = usage
        lastNotifications = notifications
    }

    companion object {
        private const val MAX_UPLOAD_BYTES = 256 * 1024
        private const val PERMISSION_CHECK_MS = 3_000L
        private const val REPORT_INTERVAL_MS = 10 * 60 * 1000L
    }
}
