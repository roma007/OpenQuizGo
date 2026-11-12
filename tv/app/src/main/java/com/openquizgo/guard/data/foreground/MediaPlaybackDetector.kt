package com.openquizgo.guard.data.foreground

import android.content.Context
import android.media.AudioManager
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/** 媒体播放状态端口，便于状态机单测替换。 */
interface PlaybackSource {
    val mediaActive: StateFlow<Boolean>
}

/**
 * 以 AudioManager 判断是否有媒体在播放。
 *
 * 不依赖 UsageStats 与前台包名：电视上只要视频在播放，音频流通常处于活跃状态；
 * 暂停、退出播放后 `isMusicActive` 变为 false，计时随之中断。
 */
class MediaPlaybackDetector(
    context: Context,
    scope: CoroutineScope,
) : PlaybackSource {

    private val audioManager =
        context.applicationContext.getSystemService(Context.AUDIO_SERVICE) as? AudioManager

    private val _mediaActive = MutableStateFlow(false)
    override val mediaActive: StateFlow<Boolean> = _mediaActive.asStateFlow()

    init {
        scope.launch {
            while (isActive) {
                val active = audioManager?.isMusicActive == true
                if (active != _mediaActive.value) {
                    GuardLog.i("playback", "mediaActive=$active")
                }
                _mediaActive.value = active
                delay(POLL_INTERVAL_MS)
            }
        }
    }

    private companion object {
        const val POLL_INTERVAL_MS = 2_000L
    }
}
