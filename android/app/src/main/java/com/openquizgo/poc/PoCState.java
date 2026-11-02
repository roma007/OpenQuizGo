package com.openquizgo.poc;

import android.util.Log;

import java.text.SimpleDateFormat;
import java.util.ArrayDeque;
import java.util.Date;
import java.util.Deque;
import java.util.Locale;

/** PoC 的共享状态：前台包名、日志环形缓冲。 */
public final class PoCState {
    private static final int MAX_LOGS = 40;
    private static final Deque<String> LOGS = new ArrayDeque<>();
    private static final SimpleDateFormat TIME = new SimpleDateFormat("HH:mm:ss", Locale.getDefault());

    public static volatile String foregroundApp = "-";

    private PoCState() {
    }

    public static synchronized void log(String message) {
        String line = TIME.format(new Date()) + "  " + message;
        LOGS.addLast(line);
        while (LOGS.size() > MAX_LOGS) {
            LOGS.removeFirst();
        }
        Log.i("OQG_POC", message);
    }

    public static synchronized String logs() {
        StringBuilder sb = new StringBuilder();
        for (String line : LOGS) {
            sb.append(line).append('\n');
        }
        return sb.length() == 0 ? "（暂无日志）\n" : sb.toString();
    }

    public static synchronized void clearLogs() {
        LOGS.clear();
    }
}
