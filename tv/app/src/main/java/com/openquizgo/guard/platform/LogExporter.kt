package com.openquizgo.guard.platform

import android.content.ClipData
import android.content.Context
import android.content.Intent
import androidx.core.content.FileProvider
import java.io.File
import java.io.Writer
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 诊断日志导出（049）。
 *
 * 把 `files/logs` 下的当前日志和轮转日志合并为一份文本，写入
 * `files/logs/export/openquizgo-tv-log-YYYYMMDD-HHmm.txt`，
 * 再通过 FileProvider + ACTION_SEND 交给用户主动分享。
 */
object LogExporter {
    private const val PROVIDER_SUFFIX = ".fileprovider"
    private const val EXPORT_DIR = "export"

    fun share(context: Context) {
        runCatching { buildAndShare(context) }
            .onFailure { GuardLog.e("export", "share log failed", it) }
    }

    private fun buildAndShare(context: Context) {
        val logsDir = File(context.filesDir, LogStore.DIRECTORY)
        val exportDir = File(logsDir, EXPORT_DIR)
        exportDir.mkdirs()

        val stamp = SimpleDateFormat("yyyyMMdd-HHmm", Locale.US).format(Date())
        val exportFile = File(exportDir, "openquizgo-tv-log-$stamp.txt")
        exportFile.bufferedWriter(Charsets.UTF_8).use { writer ->
            appendIfPresent(writer, File(logsDir, LogStore.ROTATED_FILE_NAME))
            appendIfPresent(writer, File(logsDir, LogStore.FILE_NAME))
        }

        val uri = FileProvider.getUriForFile(
            context,
            context.packageName + PROVIDER_SUFFIX,
            exportFile,
        )
        val send = Intent(Intent.ACTION_SEND).apply {
            type = "text/plain"
            putExtra(Intent.EXTRA_STREAM, uri)
            putExtra(Intent.EXTRA_SUBJECT, "OpenQuizGo TV 日志")
            putExtra(Intent.EXTRA_TEXT, "OpenQuizGo TV 诊断日志（$stamp）")
            clipData = ClipData.newRawUri("OpenQuizGo TV 日志", uri)
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        val chooser = Intent.createChooser(send, "导出日志").apply {
            addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(chooser)
        GuardLog.i("export", "log shared file=$stamp")
    }

    private fun appendIfPresent(writer: Writer, file: File) {
        if (!file.exists() || file.length() <= 0) return
        writer.write(file.readText(Charsets.UTF_8))
        writer.write("\n")
    }
}
