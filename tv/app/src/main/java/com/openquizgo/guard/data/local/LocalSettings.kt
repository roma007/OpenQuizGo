package com.openquizgo.guard.data.local

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

private val Context.localGuardDataStore by preferencesDataStore(name = "local_guard")

data class LocalSettingsSnapshot(
    val enabled: Boolean,
    val elapsedSeconds: Long,
    val lastQuestionId: String?,
)

interface LocalSettingsStore {
    val quizEnabled: Flow<Boolean>

    suspend fun snapshot(): LocalSettingsSnapshot

    suspend fun saveEnabled(enabled: Boolean)

    suspend fun saveElapsedSeconds(seconds: Long)

    suspend fun saveLastQuestionId(id: String?)
}

class LocalSettings(private val context: Context) : LocalSettingsStore {

    override val quizEnabled: Flow<Boolean> = context.localGuardDataStore.data
        .map { it[KEY_ENABLED] ?: DEFAULT_ENABLED }
        .distinctUntilChanged()

    override suspend fun snapshot(): LocalSettingsSnapshot {
        val prefs = context.localGuardDataStore.data.first()
        return LocalSettingsSnapshot(
            enabled = prefs[KEY_ENABLED] ?: DEFAULT_ENABLED,
            elapsedSeconds = prefs[KEY_ELAPSED] ?: 0L,
            lastQuestionId = prefs[KEY_LAST_QUESTION],
        )
    }

    override suspend fun saveEnabled(enabled: Boolean) {
        context.localGuardDataStore.edit { it[KEY_ENABLED] = enabled }
    }

    override suspend fun saveElapsedSeconds(seconds: Long) {
        context.localGuardDataStore.edit { it[KEY_ELAPSED] = seconds.coerceAtLeast(0) }
    }

    override suspend fun saveLastQuestionId(id: String?) {
        context.localGuardDataStore.edit { prefs ->
            if (id.isNullOrBlank()) prefs.remove(KEY_LAST_QUESTION) else prefs[KEY_LAST_QUESTION] = id
        }
    }

    private companion object {
        const val DEFAULT_ENABLED = true
        val KEY_ENABLED = booleanPreferencesKey("quiz_enabled")
        val KEY_ELAPSED = longPreferencesKey("elapsed_seconds")
        val KEY_LAST_QUESTION = stringPreferencesKey("last_question_id")
    }
}
