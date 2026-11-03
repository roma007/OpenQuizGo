package com.openquizgo.guard.di

import android.content.Context
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.foreground.ForegroundDetector
import com.openquizgo.guard.data.remote.ApiFactory
import com.openquizgo.guard.data.remote.GuardApi
import com.openquizgo.guard.domain.GuardEngine
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class AppContainer(context: Context) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val preferences: GuardPreferences = GuardPreferences(context)
    val api: GuardApi = ApiFactory.create(BuildConfig.API_BASE_URL)
    val foregroundDetector: ForegroundDetector = ForegroundDetector(context, scope)
    val guardEngine: GuardEngine = GuardEngine(preferences, foregroundDetector, scope)
}
