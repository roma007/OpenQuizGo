package com.openquizgo.guard.data.local

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.longPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map

private val Context.guardDataStore by preferencesDataStore(name = "guard")

class GuardPreferences(private val context: Context) {

    private val cipher = TokenCipher()

    val deviceToken: Flow<String?> = context.guardDataStore.data.map { prefs ->
        prefs[KEY_DEVICE_TOKEN]?.let { cipher.decrypt(it) ?: it }
    }

    val childName: Flow<String?> = context.guardDataStore.data.map { prefs ->
        prefs[KEY_CHILD_NAME]
    }

    val childId: Flow<String?> = context.guardDataStore.data.map { prefs ->
        prefs[KEY_CHILD_ID]
    }

    suspend fun deviceId(): String {
        val existing = context.guardDataStore.data.map { it[KEY_DEVICE_ID] }.first()
        if (!existing.isNullOrBlank()) return existing
        val generated = java.util.UUID.randomUUID().toString()
        context.guardDataStore.edit { it[KEY_DEVICE_ID] = generated }
        return generated
    }

    suspend fun saveBinding(token: String, childId: String?, childName: String?) {
        val stored = cipher.encrypt(token) ?: token
        context.guardDataStore.edit { prefs ->
            prefs[KEY_DEVICE_TOKEN] = stored
            if (childId.isNullOrBlank()) prefs.remove(KEY_CHILD_ID) else prefs[KEY_CHILD_ID] = childId
            if (childName.isNullOrBlank()) prefs.remove(KEY_CHILD_NAME) else prefs[KEY_CHILD_NAME] = childName
        }
    }

    suspend fun guardSnapshot(): Pair<String, Long> {
        val prefs = context.guardDataStore.data.first()
        return (prefs[KEY_GUARD_STATE] ?: "IDLE") to (prefs[KEY_WATCH_SECONDS] ?: 0L)
    }

    suspend fun saveGuardSnapshot(state: String, watchSeconds: Long) {
        context.guardDataStore.edit { prefs ->
            prefs[KEY_GUARD_STATE] = state
            prefs[KEY_WATCH_SECONDS] = watchSeconds
        }
    }

    suspend fun clearBinding() {
        context.guardDataStore.edit { prefs ->
            prefs.remove(KEY_DEVICE_TOKEN)
            prefs.remove(KEY_CHILD_ID)
            prefs.remove(KEY_CHILD_NAME)
        }
    }

    suspend fun setDeviceToken(token: String?) {
        context.guardDataStore.edit { prefs ->
            if (token.isNullOrBlank()) prefs.remove(KEY_DEVICE_TOKEN) else prefs[KEY_DEVICE_TOKEN] = token
        }
    }

    suspend fun setChildName(name: String?) {
        context.guardDataStore.edit { prefs ->
            if (name.isNullOrBlank()) prefs.remove(KEY_CHILD_NAME) else prefs[KEY_CHILD_NAME] = name
        }
    }

    private companion object {
        val KEY_DEVICE_TOKEN = stringPreferencesKey("device_token")
        val KEY_CHILD_NAME = stringPreferencesKey("child_name")
        val KEY_CHILD_ID = stringPreferencesKey("child_id")
        val KEY_DEVICE_ID = stringPreferencesKey("device_id")
        val KEY_GUARD_STATE = stringPreferencesKey("guard_state")
        val KEY_WATCH_SECONDS = longPreferencesKey("watch_seconds")
    }
}
