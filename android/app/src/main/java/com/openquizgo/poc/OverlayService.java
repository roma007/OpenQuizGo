package com.openquizgo.poc;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.Context;
import android.content.Intent;
import android.graphics.PixelFormat;
import android.os.Build;
import android.os.IBinder;
import android.provider.Settings;
import android.view.Gravity;
import android.view.View;
import android.view.WindowManager;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

/** 全屏遮罩：验证能否覆盖第三方视频 App，并承接遥控器焦点。 */
public class OverlayService extends Service {
    public static final String ACTION_SHOW = "com.openquizgo.poc.SHOW_OVERLAY";
    public static final String ACTION_HIDE = "com.openquizgo.poc.HIDE_OVERLAY";
    private static final String CHANNEL_ID = "poc_overlay";

    private WindowManager windowManager;
    private View overlayView;

    public static void show(Context context) {
        Intent intent = new Intent(context, OverlayService.class).setAction(ACTION_SHOW);
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent);
        } else {
            context.startService(intent);
        }
    }

    public static void hide(Context context) {
        context.stopService(new Intent(context, OverlayService.class));
    }

    @Override
    public void onCreate() {
        super.onCreate();
        windowManager = (WindowManager) getSystemService(WINDOW_SERVICE);
        createChannel();
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        String action = intent == null ? null : intent.getAction();
        if (ACTION_HIDE.equals(action)) {
            stopSelf();
            return START_NOT_STICKY;
        }
        if (!Settings.canDrawOverlays(this)) {
            PoCState.log("遮罩失败：未授予悬浮窗权限");
            stopSelf();
            return START_NOT_STICKY;
        }
        startForeground(1, buildNotification());
        showOverlay();
        return START_STICKY;
    }

    private void showOverlay() {
        if (overlayView != null) {
            return;
        }
        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(0xF20B0B0B);
        root.setFocusable(true);
        root.setFocusableInTouchMode(true);

        LinearLayout panel = new LinearLayout(this);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setGravity(Gravity.CENTER);
        FrameLayout.LayoutParams panelParams = new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT);
        panel.setLayoutParams(panelParams);

        TextView title = new TextView(this);
        title.setText("答题时间\n（PoC 遮罩）");
        title.setTextSize(42);
        title.setGravity(Gravity.CENTER);
        title.setTextColor(0xFFFFFFFF);
        panel.addView(title);

        TextView hint = new TextView(this);
        hint.setText("验证点：遮罩是否覆盖播放画面、遥控器焦点是否被拦截");
        hint.setTextSize(18);
        hint.setGravity(Gravity.CENTER);
        hint.setTextColor(0xFFCCCCCC);
        hint.setPadding(0, 24, 0, 48);
        panel.addView(hint);

        Button close = new Button(this);
        close.setText("关闭遮罩（模拟答题完成）");
        close.setTextSize(22);
        close.setFocusable(true);
        close.requestFocus();
        close.setOnClickListener(v -> {
            PoCState.log("遮罩已关闭");
            stopSelf();
        });
        panel.addView(close);

        root.addView(panel);

        WindowManager.LayoutParams params = new WindowManager.LayoutParams(
                WindowManager.LayoutParams.MATCH_PARENT,
                WindowManager.LayoutParams.MATCH_PARENT,
                WindowManager.LayoutParams.TYPE_APPLICATION_OVERLAY,
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN
                        | WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS
                        | WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON,
                PixelFormat.TRANSLUCENT);
        params.gravity = Gravity.TOP | Gravity.START;

        try {
            windowManager.addView(root, params);
            overlayView = root;
            PoCState.log("遮罩已显示");
        } catch (Exception e) {
            PoCState.log("遮罩添加失败：" + e);
            stopSelf();
        }
    }

    private void createChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) {
            return;
        }
        NotificationManager nm = (NotificationManager) getSystemService(NOTIFICATION_SERVICE);
        NotificationChannel channel = new NotificationChannel(
                CHANNEL_ID, "PoC 遮罩", NotificationManager.IMPORTANCE_LOW);
        nm.createNotificationChannel(channel);
    }

    private Notification buildNotification() {
        PendingIntent pending = PendingIntent.getActivity(
                this, 0, new Intent(this, MainActivity.class),
                PendingIntent.FLAG_IMMUTABLE);
        Notification.Builder builder = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O
                ? new Notification.Builder(this, CHANNEL_ID)
                : new Notification.Builder(this);
        return builder
                .setSmallIcon(android.R.drawable.ic_lock_lock)
                .setContentTitle("OpenQuizGo PoC 运行中")
                .setContentText("遮罩服务正在前台运行")
                .setContentIntent(pending)
                .setOngoing(true)
                .build();
    }

    @Override
    public void onDestroy() {
        if (overlayView != null && windowManager != null) {
            try {
                windowManager.removeView(overlayView);
            } catch (Exception ignored) {
                // 视图可能已被系统移除
            }
            overlayView = null;
        }
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}
