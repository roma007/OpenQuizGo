package com.openquizgo.guard

import android.app.Application
import com.openquizgo.guard.di.AppContainer
import com.openquizgo.guard.platform.GuardLog

class GuardApplication : Application() {
    lateinit var container: AppContainer
        private set

    override fun onCreate() {
        super.onCreate()
        GuardLog.install(this)
        GuardLog.i("app", "start version=${BuildConfig.VERSION_NAME}")
        container = AppContainer(this)
    }
}
