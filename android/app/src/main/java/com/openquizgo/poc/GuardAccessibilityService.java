package com.openquizgo.poc;

import android.accessibilityservice.AccessibilityService;
import android.view.accessibility.AccessibilityEvent;

/** 记录前台应用切换，用于验证识别延迟与权限存活。 */
public class GuardAccessibilityService extends AccessibilityService {

    @Override
    protected void onServiceConnected() {
        super.onServiceConnected();
        PoCState.log("无障碍服务已连接");
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
