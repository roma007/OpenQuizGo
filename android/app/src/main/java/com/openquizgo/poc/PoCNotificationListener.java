package com.openquizgo.poc;

import android.service.notification.NotificationListenerService;

/** 仅用于获得读取 MediaSession 的权限，不读取通知内容。 */
public class PoCNotificationListener extends NotificationListenerService {

    @Override
    public void onListenerConnected() {
        super.onListenerConnected();
        PoCState.log("通知使用权已连接（可读取 MediaSession）");
    }

    @Override
    public void onListenerDisconnected() {
        super.onListenerDisconnected();
        PoCState.log("通知使用权已断开");
    }
}
