package com.openquizgo.guard.platform

import android.content.Context
import android.content.pm.PackageManager
import android.content.res.Configuration

/**
 * 084：设备形态。
 *
 * 电视（Android TV）用遥控器焦点交互，平板/手机用触摸交互；
 * 同一个安装包在启动时判定一次，界面按形态二分。
 */
enum class FormFactor { Television, Touch }

object FormFactors {

    @Volatile
    private var cached: FormFactor? = null

    fun current(context: Context): FormFactor {
        cached?.let { return it }
        val detected = detect(
            uiModeTv = (context.getSystemService(Context.UI_MODE_SERVICE) as? android.app.UiModeManager)
                ?.currentModeType == Configuration.UI_MODE_TYPE_TELEVISION,
            leanback = context.packageManager.hasSystemFeature(PackageManager.FEATURE_LEANBACK),
        )
        cached = detected
        return detected
    }

    fun isTelevision(context: Context): Boolean = current(context) == FormFactor.Television

    /** 纯函数，便于 JVM 单测。 */
    fun detect(uiModeTv: Boolean, leanback: Boolean): FormFactor =
        if (uiModeTv || leanback) FormFactor.Television else FormFactor.Touch

    /** 上报给服务端的设备类型：`tv` / `pad`（084）。 */
    fun platform(context: Context): String =
        if (isTelevision(context)) "tv" else "pad"

    fun resetForTest() {
        cached = null
    }
}
