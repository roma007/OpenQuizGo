package com.openquizgo.guard

import android.app.Application
import com.openquizgo.guard.di.AppContainer

class GuardApplication : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        container = AppContainer(this)
    }
}
