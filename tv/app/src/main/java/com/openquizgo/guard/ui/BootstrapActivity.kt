package com.openquizgo.guard.ui

import android.app.Activity
import android.os.Bundle
import com.openquizgo.guard.platform.GuardService

/**
 * 无界面启动入口（供安装脚本 / adb 使用）。
 *
 * Android 新装应用处于 stopped 状态，必须显式启动一次组件才会接收开机广播。
 * 通过本 Activity 启动守护服务后立即结束，不展示任何前端界面。
 */
class BootstrapActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        GuardService.start(this)
        finish()
    }
}
