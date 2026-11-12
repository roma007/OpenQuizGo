package com.openquizgo.guard.di

import android.content.Context
import com.openquizgo.guard.data.foreground.MediaPlaybackDetector
import com.openquizgo.guard.data.local.LocalSettings
import com.openquizgo.guard.data.local.QuestionBank
import com.openquizgo.guard.domain.LocalGuardEngine
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob

class AppContainer(context: Context) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)

    val settings: LocalSettings = LocalSettings(context)
    val questionBank: QuestionBank = QuestionBank.load(context)
    val playbackDetector: MediaPlaybackDetector = MediaPlaybackDetector(context, scope)
    val guardEngine: LocalGuardEngine = LocalGuardEngine(
        settings = settings,
        playback = playbackDetector,
        scope = scope,
    )
}
