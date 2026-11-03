package com.openquizgo.poc;

import android.app.Activity;
import android.os.Bundle;
import android.view.Gravity;
import android.view.ViewGroup;
import android.widget.Button;
import android.widget.LinearLayout;
import android.widget.TextView;

/** 全屏答题界面：顶掉视频，退出后视频自动恢复。 */
public class QuizActivity extends Activity {
    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        PoCState.log("答题界面已打开");

        LinearLayout root = new LinearLayout(this);
        root.setOrientation(LinearLayout.VERTICAL);
        root.setGravity(Gravity.CENTER);
        root.setBackgroundColor(0xFF0B0B0B);
        root.setPadding(96, 48, 96, 48);

        TextView title = new TextView(this);
        title.setText("答题时间");
        title.setTextSize(48);
        title.setTextColor(0xFFFFFFFF);
        title.setGravity(Gravity.CENTER);
        root.addView(title);

        TextView question = new TextView(this);
        question.setText("3 + 4 = ?");
        question.setTextSize(36);
        question.setTextColor(0xFFE0E0E0);
        question.setGravity(Gravity.CENTER);
        question.setPadding(0, 32, 0, 48);
        root.addView(question);

        String[] options = {"5", "7", "9"};
        for (String option : options) {
            Button button = new Button(this);
            button.setText(option);
            button.setTextSize(28);
            button.setFocusable(true);
            button.setOnClickListener(v -> {
                PoCState.log("选择答案：" + option);
                showDone(root);
            });
            LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
                    ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
            params.topMargin = 12;
            root.addView(button, params);
        }

        setContentView(root);
    }

    private void showDone(LinearLayout root) {
        root.removeAllViews();
        TextView done = new TextView(this);
        done.setText("回答正确\n观看时长 +20 分钟");
        done.setTextSize(40);
        done.setTextColor(0xFF7CFF9B);
        done.setGravity(Gravity.CENTER);
        root.addView(done);

        Button close = new Button(this);
        close.setText("继续观看");
        close.setTextSize(28);
        close.setFocusable(true);
        close.requestFocus();
        close.setOnClickListener(v -> finish());
        LinearLayout.LayoutParams params = new LinearLayout.LayoutParams(
                ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.WRAP_CONTENT);
        params.topMargin = 48;
        root.addView(close, params);
    }
}
