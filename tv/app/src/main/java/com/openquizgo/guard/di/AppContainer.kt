package com.openquizgo.guard.di

import android.content.Context
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.foreground.ForegroundDetector
import com.openquizgo.guard.data.remote.ApiFactory
import com.openquizgo.guard.data.remote.GuardApi
import com.openquizgo.guard.data.battle.BattleRepository
import com.openquizgo.guard.data.quiz.QuizRepository
import com.openquizgo.guard.data.watch.WatchRepository
import com.openquizgo.guard.domain.GuardEngine
import com.openquizgo.guard.platform.DiagnosticsReporter
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class AppContainer(context: Context) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    val preferences: GuardPreferences = GuardPreferences(context)
    val api: GuardApi = ApiFactory.create(BuildConfig.API_BASE_URL)
    val quizRepository: QuizRepository = QuizRepository(api, preferences)
    val watchRepository: WatchRepository = WatchRepository(api, preferences)
    val battleRepository: BattleRepository = BattleRepository(
        api,
        ApiFactory.createBattleEvents(BuildConfig.API_BASE_URL),
        preferences,
    )
    val diagnosticsReporter: DiagnosticsReporter =
        DiagnosticsReporter(context, preferences, watchRepository, scope)
    val foregroundDetector: ForegroundDetector = ForegroundDetector(context, scope)
    val guardEngine: GuardEngine = GuardEngine(
        preferences,
        foregroundDetector,
        watchRepository,
        diagnosticsReporter,
        scope,
    )
}
