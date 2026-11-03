package com.openquizgo.guard.di

import android.content.Context
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.data.local.GuardPreferences
import com.openquizgo.guard.data.remote.ApiFactory
import com.openquizgo.guard.data.remote.GuardApi

class AppContainer(context: Context) {
    val preferences: GuardPreferences = GuardPreferences(context)
    val api: GuardApi = ApiFactory.create(BuildConfig.API_BASE_URL)
}
