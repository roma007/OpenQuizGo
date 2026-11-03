package com.openquizgo.poc;

import android.accessibilityservice.AccessibilityService;
import android.graphics.PixelFormat;
import android.os.Build;
import android.view.Gravity;
import android.view.View;
import android.view.WindowManager;
import android.view.accessibility.AccessibilityEvent;
import android.view.accessibility.AccessibilityNodeInfo;
import android.widget.Button;
import android.widget.FrameLayout;
import android.widget.LinearLayout;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.List;

/** 记录前台应用切换，用于验证识别延迟与权限存活。 */
public class GuardAccessibilityService extends AccessibilityService {
    private static GuardAccessibilityService instance;
    private View overlayView;

    public static GuardAccessibilityService getInstance() {
        return instance;
    }

    @Override
    protected void onServiceConnected() {
        super.onServiceConnected();
        instance = this;
        PoCState.log("无障碍服务已连接");
    }

    @Override
    public void onDestroy() {
        instance = null;
        hideAccessibilityOverlay();
        super.onDestroy();
    }

    /** 使用 TYPE_ACCESSIBILITY_OVERLAY，绕过普通悬浮窗白名单。 */
    public String showAccessibilityOverlay() {
        if (overlayView != null) {
            return "无障碍遮罩已存在";
        }
        WindowManager wm = (WindowManager) getSystemService(WINDOW_SERVICE);
        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(0xF20B0B0B);
        root.setFocusable(true);

        LinearLayout panel = new LinearLayout(this);
        panel.setOrientation(LinearLayout.VERTICAL);
        panel.setGravity(Gravity.CENTER);
        FrameLayout.LayoutParams panelParams = new FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT);
        panel.setLayoutParams(panelParams);

        TextView title = new TextView(this);
        title.setText("答题时间\n（无障碍悬浮层）");
        title.setTextSize(42);
        title.setGravity(Gravity.CENTER);
        title.setTextColor(0xFFFFFFFF);
        panel.addView(title);

        Button close = new Button(this);
        close.setText("关闭遮罩");
        close.setTextSize(22);
        close.setFocusable(true);
        close.requestFocus();
        close.setOnClickListener(v -> hideAccessibilityOverlay());
        panel.addView(close);
        root.addView(panel);

        WindowManager.LayoutParams params = new WindowManager.LayoutParams(
                WindowManager.LayoutParams.MATCH_PARENT,
                WindowManager.LayoutParams.MATCH_PARENT,
                Build.VERSION.SDK_INT >= Build.VERSION_CODES.LOLLIPOP_MR1
                        ? WindowManager.LayoutParams.TYPE_ACCESSIBILITY_OVERLAY
                        : WindowManager.LayoutParams.TYPE_SYSTEM_ALERT,
                WindowManager.LayoutParams.FLAG_LAYOUT_IN_SCREEN
                        | WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS
                        | WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON,
                PixelFormat.TRANSLUCENT);
        try {
            wm.addView(root, params);
            overlayView = root;
            PoCState.log("无障碍遮罩已显示");
            return "无障碍遮罩已显示";
        } catch (Exception e) {
            PoCState.log("无障碍遮罩失败：" + e);
            return "无障碍遮罩失败：" + e;
        }
    }

    public void hideAccessibilityOverlay() {
        if (overlayView == null) {
            return;
        }
        WindowManager wm = (WindowManager) getSystemService(WINDOW_SERVICE);
        try {
            wm.removeView(overlayView);
            PoCState.log("无障碍遮罩已关闭");
        } catch (Exception ignored) {
        }
        overlayView = null;
    }

    /** 遍历当前窗口，尝试点击包含指定关键词的控件。 */
    public String clickNode(String keyword) {
        AccessibilityNodeInfo root = getRootInActiveWindow();
        if (root == null) {
            return "无法读取当前窗口节点";
        }
        List<String> labels = new ArrayList<>();
        boolean clicked = walk(root, keyword.toLowerCase(), labels);
        String summary = "节点样本：" + labels.subList(0, Math.min(labels.size(), 12));
        PoCState.log(summary);
        return clicked ? "已点击包含「" + keyword + "」的控件" : "未找到包含「" + keyword + "」的控件";
    }

    private boolean walk(AccessibilityNodeInfo node, String keyword, List<String> labels) {
        if (node == null) {
            return false;
        }
        CharSequence text = node.getText();
        CharSequence desc = node.getContentDescription();
        String label = ((text == null ? "" : text) + " " + (desc == null ? "" : desc)).trim();
        if (!label.isEmpty() && labels.size() < 40) {
            labels.add(label);
        }
        String lower = label.toLowerCase();
        if (lower.contains(keyword)) {
            if (node.isClickable()) {
                node.performAction(AccessibilityNodeInfo.ACTION_CLICK);
                return true;
            }
            AccessibilityNodeInfo parent = node.getParent();
            if (parent != null && parent.isClickable()) {
                parent.performAction(AccessibilityNodeInfo.ACTION_CLICK);
                return true;
            }
        }
        for (int i = 0; i < node.getChildCount(); i++) {
            if (walk(node.getChild(i), keyword, labels)) {
                return true;
            }
        }
        return false;
    }

    @Override
    public void onAccessibilityEvent(AccessibilityEvent event) {
        if (event == null || event.getEventType() != AccessibilityEvent.TYPE_WINDOW_STATE_CHANGED) {
            return;
        }
        CharSequence pkg = event.getPackageName();
        CharSequence cls = event.getClassName();
        if (pkg == null || pkg.toString().equals(getPackageName())) {
            return;
        }
        String value = pkg.toString();
        if (!value.equals(PoCState.foregroundApp)) {
            PoCState.foregroundApp = value;
            PoCState.log("前台应用：" + value + " / " + (cls == null ? "-" : cls));
        }
    }

    @Override
    public void onInterrupt() {
        PoCState.log("无障碍服务被中断");
    }
}
