package com.openquizgo.poc;

import android.Manifest;
import android.app.Activity;
import android.app.AppOpsManager;
import android.content.Intent;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.os.Process;
import android.provider.Settings;
import android.text.method.ScrollingMovementMethod;
import android.view.Gravity;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.ScrollView;
import android.widget.TextView;

/** PoC 主界面：权限引导 + 四种暂停/恢复手段 + 遮罩测试。 */
public class MainActivity extends Activity {
    private TextView statusView;
    private TextView logView;
    private final Handler handler = new Handler(Looper.getMainLooper());

    private final Runnable refresh = new Runnable() {
        @Override
        public void run() {
            refreshStatus();
            handler.postDelayed(this, 1000);
        }
    };

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(buildUi());
        requestNotificationPermission();
        PoCState.log("PoC 启动");
    }

    @Override
    protected void onResume() {
        super.onResume();
        handler.post(refresh);
    }

    @Override
    protected void onPause() {
        handler.removeCallbacks(refresh);
        super.onPause();
    }

    private ViewGroup buildUi() {
        ScrollView scroll = new ScrollView(this);
        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setPadding(48, 32, 48, 48);
        scroll.addView(root);

        TextView title = new TextView(this);
        title.setText("OpenQuizGo 设备管控 PoC");
        title.setTextSize(26);
        title.setGravity(Gravity.START);
        root.addView(title);

        statusView = new TextView(this);
        statusView.setTextSize(15);
        statusView.setPadding(0, 20, 0, 20);
        root.addView(statusView);

        addButton(root, "1. 开启无障碍服务", () -> openSettings(Settings.ACTION_ACCESSIBILITY_SETTINGS));
        addButton(root, "2. 授予悬浮窗权限", () -> {
            Intent intent = new Intent(Settings.ACTION_MANAGE_OVERLAY_PERMISSION,
                    Uri.parse("package:" + getPackageName()));
            open(intent);
        });
        addButton(root, "3. 授予通知使用权（读 MediaSession）", () ->
                openSettings(Settings.ACTION_NOTIFICATION_LISTENER_SETTINGS));
        addButton(root, "4. 授予使用情况访问权限", () ->
                openSettings(Settings.ACTION_USAGE_ACCESS_SETTINGS));

        addButton(root, "5. 显示全屏遮罩", () -> {
            PoCState.log("请求显示遮罩");
            OverlayService.show(this);
        });
        addButton(root, "6. 关闭遮罩", () -> OverlayService.hide(this));

        addButton(root, "A. MediaSession 暂停", () ->
                PoCState.log(MediaControl.pauseViaSession(this)));
        addButton(root, "A. MediaSession 恢复", () ->
                PoCState.log(MediaControl.playViaSession(this)));

        addButton(root, "B. 请求音频焦点（暂停）", () ->
                PoCState.log(MediaControl.requestFocus(this)));
        addButton(root, "B. 释放音频焦点（恢复）", () ->
                PoCState.log(MediaControl.abandonFocus(this)));

        addButton(root, "C. 媒体键暂停", () ->
                PoCState.log(MediaControl.mediaKey(this, true)));
        addButton(root, "C. 媒体键播放", () ->
                PoCState.log(MediaControl.mediaKey(this, false)));

        addButton(root, "每秒刷新状态 / 清空日志", () -> {
            PoCState.clearLogs();
            refreshStatus();
        });

        logView = new TextView(this);
        logView.setTextSize(12);
        logView.setPadding(0, 24, 0, 0);
        logView.setMovementMethod(new ScrollingMovementMethod());
        root.addView(logView);

        return scroll;
    }

    private void addButton(LinearLayout parent, String label, Runnable action) {
        Button button = new Button(this);
        button.setText(label);
        button.setTextSize(17);
        button.setAllCaps(false);
        button.setFocusable(true);
        button.setOnClickListener(v -> action.run());
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
                LinearLayout.LayoutParams.MATCH_PARENT,
                LinearLayout.LayoutParams.WRAP_CONTENT);
        params.topMargin = 12;
        parent.addView(button, params);
    }

    private void openSettings(String action) {
        open(new Intent(action));
    }

    private void open(Intent intent) {
        try {
            startActivity(intent);
        } catch (Exception e) {
            PoCState.log("打开设置失败：" + e);
        }
    }

    private void refreshStatus() {
        StringBuilder sb = new StringBuilder();
        sb.append("前台应用：").append(PoCState.foregroundApp).append('\n');
        sb.append("MediaSession：").append(MediaControl.describeSessions(this)).append('\n');
        sb.append("权限：")
                .append("悬浮窗 ").append(Settings.canDrawOverlays(this) ? "✓" : "✗")
                .append(" ｜ 无障碍 ").append(accessibilityEnabled() ? "✓" : "✗")
                .append(" ｜ 通知使用权 ").append(notificationAccessEnabled() ? "✓" : "✗")
                .append(" ｜ 使用情况 ").append(usageAccessEnabled() ? "✓" : "✗");
        statusView.setText(sb.toString());
        logView.setText(PoCState.logs());
    }

    private boolean accessibilityEnabled() {
        String enabled = Settings.Secure.getString(
                getContentResolver(), Settings.Secure.ENABLED_ACCESSIBILITY_SERVICES);
        return enabled != null && enabled.contains(getPackageName())
                && enabled.contains("GuardAccessibilityService");
    }

    private boolean notificationAccessEnabled() {
        String enabled = Settings.Secure.getString(
                getContentResolver(), "enabled_notification_listeners");
        return enabled != null && enabled.contains(getPackageName());
    }

    private boolean usageAccessEnabled() {
        AppOpsManager appOps = (AppOpsManager) getSystemService(APP_OPS_SERVICE);
        if (appOps == null) {
            return false;
        }
        int mode = appOps.checkOpNoThrow(
                AppOpsManager.OPSTR_GET_USAGE_STATS, Process.myUid(), getPackageName());
        return mode == AppOpsManager.MODE_ALLOWED;
    }

    private void requestNotificationPermission() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU
                && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS)
                != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 1);
        }
    }
}
