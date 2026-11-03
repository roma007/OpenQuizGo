package com.openquizgo.guard.platform

import android.util.Log

object GuardLog {
    private const val TAG = "OQG_GUARD"

    fun i(message: String) {
        Log.i(TAG, message)
    }

    fun w(message: String) {
        Log.w(TAG, message)
    }

    fun e(message: String, throwable: Throwable? = null) {
        Log.e(TAG, message, throwable)
    }
}
