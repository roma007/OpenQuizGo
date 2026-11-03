package com.openquizgo.guard.platform

import android.util.Log
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

object GuardLog {
    private const val TAG = "OQG_GUARD"
    private const val MAX_LINES = 120

    private val timeFormat = SimpleDateFormat("HH:mm:ss", Locale.getDefault())
    private val lines = ArrayDeque<String>()

    private val _recent = MutableStateFlow<List<String>>(emptyList())
    val recent: StateFlow<List<String>> = _recent.asStateFlow()

    fun i(message: String) {
        Log.i(TAG, message)
        record("I", message)
    }

    fun w(message: String) {
        Log.w(TAG, message)
        record("W", message)
    }

    fun e(message: String, throwable: Throwable? = null) {
        Log.e(TAG, message, throwable)
        record("E", if (throwable == null) message else "$message: ${throwable.message}")
    }

    private fun record(level: String, message: String) {
        synchronized(lines) {
            lines.addLast("${timeFormat.format(Date())} $level $message")
            while (lines.size > MAX_LINES) lines.removeFirst()
            _recent.value = lines.toList()
        }
    }
}
