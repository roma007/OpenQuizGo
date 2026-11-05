package com.openquizgo.guard.platform

import android.content.Context
import android.util.Log
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.io.File
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 统一日志入口（049）。
 *
 * 同时写入 Android logcat、诊断页内存列表和私有目录滚动日志文件。
 * 调用方不得传入令牌、设备密钥等敏感信息。
 */
object GuardLog {
    private const val TAG = "OQG_GUARD"
    private const val MAX_LINES = 120

    private val timeFormat = SimpleDateFormat("HH:mm:ss", Locale.getDefault())
    private val lines = ArrayDeque<String>()

    @Volatile
    private var store: LogStore? = null

    private val _recent = MutableStateFlow<List<String>>(emptyList())
    val recent: StateFlow<List<String>> = _recent.asStateFlow()

    /** 在 Application.onCreate 中调用，启用本地日志文件。 */
    fun install(context: Context) {
        if (store == null) {
            store = LogStore(File(context.filesDir, LogStore.DIRECTORY))
        }
    }

    fun i(message: String) = i(TAG, message)

    fun i(tag: String, message: String) {
        Log.i(tag, message)
        record("I", tag, message)
    }

    fun w(message: String) = w(TAG, message)

    fun w(tag: String, message: String) {
        Log.w(tag, message)
        record("W", tag, message)
    }

    fun e(message: String, throwable: Throwable? = null) = e(TAG, message, throwable)

    fun e(tag: String, message: String, throwable: Throwable? = null) {
        Log.e(tag, message, throwable)
        val detail = if (throwable == null) {
            message
        } else {
            "$message: ${throwable.javaClass.simpleName}: ${throwable.message}"
        }
        record("E", tag, detail)
    }

    private fun record(level: String, tag: String, message: String) {
        synchronized(lines) {
            lines.addLast("${timeFormat.format(Date())} $level $message")
            while (lines.size > MAX_LINES) lines.removeFirst()
            _recent.value = lines.toList()
        }
        store?.write(level, tag, message)
    }
}
