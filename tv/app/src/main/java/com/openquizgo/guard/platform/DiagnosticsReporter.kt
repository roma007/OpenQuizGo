package com.openquizgo.guard.platform

import android.content.Context
import android.content.pm.PackageManager
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
import java.security.MessageDigest
import java.time.Duration
import java.time.Instant
import kotlin.math.abs

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
    private var lastClockSkewReportAt = 0L
    private var signatureChecked = false

    init {
        scope.launch { checkSignature() }
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

    /** 比较心跳服务端时间，偏差 >5 分钟且距上次上报超过 1 小时时上报。 */
    suspend fun reportClockSkew(serverTime: String?) {
        val nowMillis = System.currentTimeMillis()
        val delta = clockSkewMillis(serverTime, nowMillis) ?: return
        if (abs(delta) <= CLOCK_SKEW_MS) return
        if (nowMillis - lastClockSkewReportAt < CLOCK_SKEW_REPORT_INTERVAL_MS) return
        lastClockSkewReportAt = nowMillis
        reportTamper(
            "clock_skew",
            mapOf(
                "delta_seconds" to (delta / 1000).toString(),
                "server_time" to (serverTime ?: ""),
            ),
        )
    }

    /** 校验 APK 签名 pin；未配置 expectedSignature 时跳过。 */
    private suspend fun checkSignature() {
        if (signatureChecked) return
        signatureChecked = true
        val expected = BuildConfig.EXPECTED_SIGNATURE.trim().lowercase()
        if (expected.isEmpty()) return
        val actual = currentSignatureHashes()
        if (actual.isEmpty() || expected !in actual) {
            reportTamper(
                "package_tampered",
                mapOf(
                    "expected" to expected,
                    "actual" to actual.joinToString(","),
                ),
            )
        }
    }

    @Suppress("DEPRECATION")
    private fun currentSignatureHashes(): List<String> {
        return runCatching {
            val pm = appContext.packageManager
            val packageName = appContext.packageName
            val signatures = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
                pm.getPackageInfo(packageName, PackageManager.GET_SIGNING_CERTIFICATES)
                    .signingInfo?.apkContentsSigners.orEmpty()
            } else {
                pm.getPackageInfo(packageName, PackageManager.GET_SIGNATURES)
                    .signatures.orEmpty()
            }
            signatures.map { signature ->
                MessageDigest.getInstance("SHA-256")
                    .digest(signature.toByteArray())
                    .joinToString("") { "%02x".format(it) }
            }
        }.getOrDefault(emptyList())
    }

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
        private const val CLOCK_SKEW_MS = 5 * 60 * 1000L
        private const val CLOCK_SKEW_REPORT_INTERVAL_MS = 60 * 60 * 1000L
        private const val PERMISSION_CHECK_MS = 3_000L
        private const val REPORT_INTERVAL_MS = 10 * 60 * 1000L

        internal fun clockSkewMillis(serverTime: String?, nowMillis: Long): Long? {
            if (serverTime.isNullOrBlank()) return null
            return runCatching {
                Duration.between(Instant.parse(serverTime), Instant.ofEpochMilli(nowMillis)).toMillis()
            }.getOrNull()
        }
    }
}
