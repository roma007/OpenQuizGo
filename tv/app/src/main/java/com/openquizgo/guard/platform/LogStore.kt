package com.openquizgo.guard.platform

import java.io.File
import java.io.FileOutputStream
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

/**
 * 应用私有目录下的滚动日志文件（049）。
 *
 * - 写入 `files/logs/guard.log`，单文件上限 [MAX_BYTES]（1MB）；
 * - 超出后轮转为 `guard.log.1`，只保留一份备份；
 * - 所有入口线程安全，任何 IO/权限异常都会被吞掉，不影响主流程。
 *
 * 不依赖 Android API，便于在 JVM 单元测试中使用临时目录验证。
 */
class LogStore(
    private val logDir: File,
    private val maxBytes: Long = MAX_BYTES,
) {
    private val logFile = File(logDir, FILE_NAME)
    private val rotatedFile = File(logDir, ROTATED_FILE_NAME)
    private val timeFormat = SimpleDateFormat("yyyy-MM-dd HH:mm:ss.SSS", Locale.US).apply {
        timeZone = TimeZone.getDefault()
    }

    @Synchronized
    fun write(level: String, tag: String, message: String) {
        runCatching {
            val bytes = buildLine(level, tag, message).toByteArray(Charsets.UTF_8)
            val payload = if (bytes.size > maxBytes) {
                bytes.copyOf(maxBytes.coerceAtMost(Int.MAX_VALUE.toLong()).toInt())
            } else {
                bytes
            }
            if (!logDir.exists()) logDir.mkdirs()
            if (logFile.length() > 0 && logFile.length() + payload.size > maxBytes) {
                rotate()
            }
            FileOutputStream(logFile, true).use { it.write(payload) }
        }
    }

    private fun buildLine(level: String, tag: String, message: String): String {
        val safeTag = tag.replace('\n', ' ').replace('\r', ' ').take(MAX_TAG_CHARS)
        val safeMessage = message.replace('\n', ' ').replace('\r', ' ').take(MAX_MESSAGE_CHARS)
        return "${timeFormat.format(Date())} $level/$safeTag $safeMessage\n"
    }

    private fun rotate() {
        if (rotatedFile.exists()) rotatedFile.delete()
        if (!logFile.renameTo(rotatedFile)) {
            // 某些文件系统不允许 rename 覆盖，退化为复制后清空当前文件。
            runCatching {
                logFile.copyTo(rotatedFile, overwrite = true)
                logFile.delete()
            }
        }
    }

    companion object {
        const val DIRECTORY = "logs"
        const val FILE_NAME = "guard.log"
        const val ROTATED_FILE_NAME = "guard.log.1"
        const val MAX_BYTES = 1024L * 1024L

        private const val MAX_TAG_CHARS = 64
        private const val MAX_MESSAGE_CHARS = 8 * 1024
    }
}
