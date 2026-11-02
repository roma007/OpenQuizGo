package com.openquizgo.poc;

import android.content.ComponentName;
import android.content.Context;
import android.media.AudioFocusRequest;
import android.media.AudioManager;
import android.media.session.MediaController;
import android.media.session.MediaSessionManager;
import android.media.session.PlaybackState;
import android.view.KeyEvent;

import java.util.ArrayList;
import java.util.List;

/** 四种暂停/恢复手段的 PoC 实现。 */
public final class MediaControl {
    private static AudioFocusRequest focusRequest;
    private static String lastPausedPackage;

    private MediaControl() {
    }

    public static List<MediaController> sessions(Context context) {
        MediaSessionManager manager =
                (MediaSessionManager) context.getSystemService(Context.MEDIA_SESSION_SERVICE);
        if (manager == null) {
            return new ArrayList<>();
        }
        ComponentName listener = new ComponentName(context, PoCNotificationListener.class);
        try {
            return manager.getActiveSessions(listener);
        } catch (SecurityException e) {
            PoCState.log("读取 MediaSession 失败：未授予通知使用权");
            return new ArrayList<>();
        }
    }

    public static String describeSessions(Context context) {
        List<MediaController> list = sessions(context);
        if (list.isEmpty()) {
            return "无（或未授权）";
        }
        StringBuilder sb = new StringBuilder();
        for (MediaController controller : list) {
            PlaybackState state = controller.getPlaybackState();
            int code = state == null ? -1 : state.getState();
            sb.append(controller.getPackageName())
                    .append(" [").append(stateName(code)).append("] ");
        }
        return sb.toString().trim();
    }

    private static String stateName(int code) {
        switch (code) {
            case PlaybackState.STATE_PLAYING:
                return "播放中";
            case PlaybackState.STATE_PAUSED:
                return "已暂停";
            case PlaybackState.STATE_STOPPED:
                return "已停止";
            case PlaybackState.STATE_BUFFERING:
                return "缓冲";
            default:
                return "状态" + code;
        }
    }

    /** 手段 A：通过 MediaSession 直接暂停。 */
    public static String pauseViaSession(Context context) {
        long start = System.currentTimeMillis();
        for (MediaController controller : sessions(context)) {
            PlaybackState state = controller.getPlaybackState();
            if (state != null && state.getState() == PlaybackState.STATE_PLAYING) {
                controller.getTransportControls().pause();
                lastPausedPackage = controller.getPackageName();
                long cost = System.currentTimeMillis() - start;
                return "MediaSession 暂停 " + lastPausedPackage + "（" + cost + "ms）";
            }
        }
        return "MediaSession 无播放中的会话";
    }

    /** 手段 A：恢复最近一次由本应用暂停的会话。 */
    public static String playViaSession(Context context) {
        for (MediaController controller : sessions(context)) {
            PlaybackState state = controller.getPlaybackState();
            boolean paused = state != null && state.getState() == PlaybackState.STATE_PAUSED;
            boolean target = lastPausedPackage == null
                    || lastPausedPackage.equals(controller.getPackageName());
            if (paused && target) {
                controller.getTransportControls().play();
                return "MediaSession 恢复 " + controller.getPackageName();
            }
        }
        return "MediaSession 无可恢复的会话";
    }

    /** 手段 B：请求瞬时音频焦点，行为良好的播放器会暂停。 */
    public static String requestFocus(Context context) {
        AudioManager am = (AudioManager) context.getSystemService(Context.AUDIO_SERVICE);
        if (am == null) {
            return "无 AudioManager";
        }
        if (focusRequest == null) {
            focusRequest = new AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN_TRANSIENT)
                    .setOnAudioFocusChangeListener(change ->
                            PoCState.log("音频焦点变化：" + change))
                    .build();
        }
        int result = am.requestAudioFocus(focusRequest);
        return result == AudioManager.AUDIOFOCUS_REQUEST_GRANTED
                ? "音频焦点已获取（观察视频是否暂停）"
                : "音频焦点获取失败：" + result;
    }

    public static String abandonFocus(Context context) {
        AudioManager am = (AudioManager) context.getSystemService(Context.AUDIO_SERVICE);
        if (am == null || focusRequest == null) {
            return "无待释放的音频焦点";
        }
        am.abandonAudioFocusRequest(focusRequest);
        return "音频焦点已释放（观察视频是否恢复）";
    }

    /** 手段 C：发送媒体按键。 */
    public static String mediaKey(Context context, boolean pause) {
        AudioManager am = (AudioManager) context.getSystemService(Context.AUDIO_SERVICE);
        if (am == null) {
            return "无 AudioManager";
        }
        int keyCode = pause ? KeyEvent.KEYCODE_MEDIA_PAUSE : KeyEvent.KEYCODE_MEDIA_PLAY;
        long start = System.currentTimeMillis();
        am.dispatchMediaKeyEvent(new KeyEvent(KeyEvent.ACTION_DOWN, keyCode));
        am.dispatchMediaKeyEvent(new KeyEvent(KeyEvent.ACTION_UP, keyCode));
        long cost = System.currentTimeMillis() - start;
        return "媒体键 " + (pause ? "PAUSE" : "PLAY") + " 已发送（" + cost + "ms）";
    }
}
