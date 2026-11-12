package com.openquizgo.guard.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.ViewModelProvider
import com.openquizgo.guard.di.AppContainer
import com.openquizgo.guard.domain.LocalGuardEngine
import com.openquizgo.guard.domain.LocalGuardState
import kotlinx.coroutines.flow.StateFlow

class LocalViewModel(
    private val engine: LocalGuardEngine,
    val questionCount: Int,
) : ViewModel() {

    val enabled: StateFlow<Boolean> = engine.enabled
    val state: StateFlow<LocalGuardState> = engine.state
    val elapsedSeconds: StateFlow<Long> = engine.elapsedSeconds
    val remainingSeconds: StateFlow<Long> = engine.remainingSeconds

    fun toggleQuiz() {
        engine.setEnabled(!enabled.value)
    }

    class Factory(private val container: AppContainer) : ViewModelProvider.Factory {
        @Suppress("UNCHECKED_CAST")
        override fun <T : ViewModel> create(modelClass: Class<T>): T {
            return LocalViewModel(
                engine = container.guardEngine,
                questionCount = container.questionBank.size,
            ) as T
        }
    }
}
