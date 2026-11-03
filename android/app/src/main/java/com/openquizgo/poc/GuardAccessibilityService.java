package com.openquizgo.poc;

import android.accessibilityservice.AccessibilityService;
import android.graphics.PixelFormat;
import android.graphics.Path;
import android.graphics.Rect;
import android.os.Build;
import android.view.Gravity;
import android.accessibilityservice.GestureDescription;
import android.view.View;
import android.view.WindowManager;
import android.view.accessibility.AccessibilityEvent;
import android.accessibilityservice.AccessibilityServiceInfo;
import android.view.accessibility.AccessibilityNodeInfo;
import android.view.accessibility.AccessibilityWindowInfo;
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
    private volatile boolean blocking;
    private volatile long lastLaunchAt;

    public static GuardAccessibilityService getInstance() {
        return instance;
    }

    @Override
    protected void onServiceConnected() {
        super.onServiceConnected();
        instance = this;
        AccessibilityServiceInfo info = getServiceInfo();
        if (info != null) {
            info.flags |= AccessibilityServiceInfo.FLAG_RETRIEVE_INTERACTIVE_WINDOWS;
            setServiceInfo(info);
            PoCState.log("无障碍服务已连接，flags=" + info.flags);
        } else {
            PoCState.log("无障碍服务已连接（无 serviceInfo）");
        }
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

    /** 开启/关闭拦截模式：非本应用前台时自动拉起答题界面。 */
    public String setBlocking(boolean enabled) {
        blocking = enabled;
        PoCState.log("拦截模式：" + (enabled ? "开启" : "关闭"));
        return "拦截模式：" + (enabled ? "开启" : "关闭");
    }

    /** 在屏幕坐标注入一次点击，用于触发播放器的点击暂停/播放。 */
    public String tap(int x, int y) {
        Path path = new Path();
        path.moveTo(x, y);
        GestureDescription gesture = new GestureDescription.Builder()
                .addStroke(new GestureDescription.StrokeDescription(path, 0, 60))
                .build();
        boolean ok = dispatchGesture(gesture, null, null);
        PoCState.log("手势点击 (" + x + "," + y + ") -> " + ok);
        return ok ? "已点击 (" + x + "," + y + ")" : "手势注入失败";
    }

    /** 在指定坐标寻找最深层可点击节点并点击，模拟确认键。 */
    public String clickAt(int x, int y) {
        AccessibilityNodeInfo root = getRootInActiveWindow();
        if (root == null) {
            return "无法读取当前窗口节点";
        }
        AccessibilityNodeInfo target = findClickable(root, x, y);
        if (target == null) {
            return "该坐标没有可点击节点";
        }
        boolean ok = target.performAction(AccessibilityNodeInfo.ACTION_CLICK);
        PoCState.log("点击节点 " + target.getClassName() + " -> " + ok);
        return ok ? "已点击视频区域" : "点击失败";
    }

    private AccessibilityNodeInfo findClickable(AccessibilityNodeInfo node, int x, int y) {
        if (node == null) {
            return null;
        }
        Rect rect = new Rect();
        node.getBoundsInScreen(rect);
        if (!rect.contains(x, y)) {
            return null;
        }
        for (int i = 0; i < node.getChildCount(); i++) {
            AccessibilityNodeInfo found = findClickable(node.getChild(i), x, y);
            if (found != null) {
                return found;
            }
        }
        return node.isClickable() ? node : null;
    }

    /** 由无障碍服务直接拉起全屏答题界面。 */
    public String openQuiz() {
        startActivity(new android.content.Intent(this, QuizActivity.class)
                .addFlags(android.content.Intent.FLAG_ACTIVITY_NEW_TASK
                        | android.content.Intent.FLAG_ACTIVITY_CLEAR_TOP));
        PoCState.log("无障碍服务已拉起答题界面");
        return "无障碍服务已拉起答题界面";
    }

    /** 遍历当前窗口，尝试点击包含指定关键词的控件。 */
    public String clickNode(String keyword) {
        AccessibilityNodeInfo root = getRootInActiveWindow();
        if (root == null) {
            return "无法读取当前窗口节点";
        }
        List<String> labels = new ArrayList<>();
        boolean clicked = walk(root, keyword.toLowerCase(), labels);
        PoCState.log("节点样本：" + labels.subList(0, Math.min(labels.size(), 12)));
        return clicked ? "已点击包含「" + keyword + "」的控件" : "未找到包含「" + keyword + "」的控件";
    }

    /** 遍历所有无障碍窗口，播放器控制条可能在独立窗口里。 */
    public String dumpWindows() {
        List<AccessibilityWindowInfo> windows = getWindows();
        int index = 0;
        for (AccessibilityWindowInfo window : windows) {
            AccessibilityNodeInfo root = window.getRoot();
            String pkg = root == null ? "-" : String.valueOf(root.getPackageName());
            PoCState.log("window#" + index + " type=" + window.getType()
                    + " layer=" + window.getLayer() + " pkg=" + pkg);
            if (root != null) {
                List<String> lines = new ArrayList<>();
                collect(root, lines, 0);
                for (String line : lines) {
                    PoCState.log("w" + index + " " + line);
                }
            }
            index++;
        }
        return "窗口数：" + index;
    }

    /** 打印当前窗口的可访问节点，便于定位播放/暂停按钮。 */
    public String dumpNodes() {
        AccessibilityNodeInfo root = getRootInActiveWindow();
        if (root == null) {
            return "无法读取当前窗口节点";
        }
        List<String> lines = new ArrayList<>();
        collect(root, lines, 0);
        for (String line : lines) {
            PoCState.log(line);
        }
        return "节点数：" + lines.size();
    }

    private void collect(AccessibilityNodeInfo node, List<String> lines, int depth) {
        if (node == null || lines.size() >= 120) {
            return;
        }
        CharSequence text = node.getText();
        CharSequence desc = node.getContentDescription();
        String viewId = node.getViewIdResourceName();
        String className = String.valueOf(node.getClassName());
        boolean interesting = (text != null && text.length() > 0)
                || (desc != null && desc.length() > 0)
                || (viewId != null && viewId.length() > 0)
                || node.isClickable()
                || className.contains("Surface")
                || className.contains("Video")
                || className.contains("Image")
                || className.contains("SeekBar");
        if (interesting) {
            Rect rect = new Rect();
            node.getBoundsInScreen(rect);
            AccessibilityNodeInfo.RangeInfo range = node.getRangeInfo();
            String rangeText = range == null ? "" : " range=" + range.getCurrent() + "/" + range.getMax();
            lines.add("d" + depth + " " + className
                    + " id=[" + viewId + "] txt=[" + text + "] desc=[" + desc + "]"
                    + " click=" + node.isClickable() + rangeText + " bounds=" + rect);
        }
        for (int i = 0; i < node.getChildCount(); i++) {
            collect(node.getChild(i), lines, depth + 1);
        }
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
        if (blocking && !value.equals(getPackageName())
                && System.currentTimeMillis() - lastLaunchAt > 1500) {
            lastLaunchAt = System.currentTimeMillis();
            PoCState.log("拦截模式：拉起答题界面");
            openQuiz();
        }
    }

    @Override
    public void onInterrupt() {
        PoCState.log("无障碍服务被中断");
    }
}
