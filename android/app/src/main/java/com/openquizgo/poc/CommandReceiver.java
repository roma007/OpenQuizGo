package com.openquizgo.poc;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

/** 便于 ADB 驱动的测试指令入口。 */
public class CommandReceiver extends BroadcastReceiver {
    public static final String ACTION = "com.openquizgo.poc.CMD";

    @Override
    public void onReceive(Context context, Intent intent) {
        String command = intent.getStringExtra("cmd");
        if (command == null) {
            PoCState.log("收到空指令");
            return;
        }
        PoCState.log("ADB 指令：" + command);
        switch (command) {
            case "pause_session":
                PoCState.log(MediaControl.pauseViaSession(context));
                break;
            case "play_session":
                PoCState.log(MediaControl.playViaSession(context));
                break;
            case "focus":
                PoCState.log(MediaControl.requestFocus(context));
                break;
            case "abandon":
                PoCState.log(MediaControl.abandonFocus(context));
                break;
            case "key_pause":
                PoCState.log(MediaControl.mediaKey(context, true));
                break;
            case "key_play":
                PoCState.log(MediaControl.mediaKey(context, false));
                break;
            case "overlay_show":
                OverlayService.show(context);
                break;
            case "overlay_hide":
                OverlayService.hide(context);
                break;
            case "acc_overlay_show":
                PoCState.log(withService(service -> service.showAccessibilityOverlay()));
                break;
            case "acc_overlay_hide":
                PoCState.log(withService(service -> {
                    service.hideAccessibilityOverlay();
                    return "无障碍遮罩已关闭";
                }));
                break;
            case "open_quiz":
                Intent quiz = new Intent(context, QuizActivity.class)
                        .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK | Intent.FLAG_ACTIVITY_CLEAR_TOP);
                context.startActivity(quiz);
                PoCState.log("已请求打开答题界面");
                break;
            case "arm_quiz":
                PoCState.log(withService(GuardAccessibilityService::armQuiz));
                break;
            case "disarm_quiz":
                PoCState.log(withService(GuardAccessibilityService::disarmQuiz));
                break;
            case "block_on":
                PoCState.log(withService(service -> service.setBlocking(true)));
                break;
            case "block_off":
                PoCState.log(withService(service -> service.setBlocking(false)));
                break;
            case "acc_open_quiz":
                PoCState.log(withService(GuardAccessibilityService::openQuiz));
                break;
            case "click_video_center":
                PoCState.log(withService(service -> service.clickAt(640, 360)));
                break;
            case "tap_center":
                PoCState.log(withService(service -> service.tap(640, 360)));
                break;
            case "tap_video":
                PoCState.log(withService(service -> service.tap(320, 180)));
                break;
            case "dump_windows":
                PoCState.log(withService(GuardAccessibilityService::dumpWindows));
                break;
            case "dump_nodes":
                PoCState.log(withService(GuardAccessibilityService::dumpNodes));
                break;
            case "acc_pause":
                PoCState.log(withService(service -> service.clickNode("暂停")));
                break;
            case "acc_play":
                PoCState.log(withService(service -> service.clickNode("播放")));
                break;
            default:
                PoCState.log("未知指令：" + command);
        }
    }

    private String withService(java.util.function.Function<GuardAccessibilityService, String> action) {
        GuardAccessibilityService service = GuardAccessibilityService.getInstance();
        if (service == null) {
            return "无障碍服务未连接";
        }
        return action.apply(service);
    }
}
