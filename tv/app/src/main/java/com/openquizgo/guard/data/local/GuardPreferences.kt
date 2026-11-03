package com.openquizgo.guard.data.local

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

private val Context.guardDataStore by preferencesDataStore(name = "guard")

class GuardPreferences(private val context: Context) {

    val deviceToken: Flow<String?> = context.guardDataStore.data.map { prefs ->
        prefs[KEY_DEVICE_TOKEN]
    }

    val childName: Flow<String?> = context.guardDataStore.data.map { prefs ->
        prefs[KEY_CHILD_NAME]
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
    }
}
