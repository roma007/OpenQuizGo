package com.openquizgo.guard.ui

import android.os.Bundle
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import com.openquizgo.guard.GuardApplication

/**
 * 半透明全屏答题界面。
 *
 * 关键：Activity 使用半透明主题，播放器只进入 paused 而不是被 stop；
 * 用户先按确认键暂停，App 监听到后再拉起本界面，因此无广告、进度保留。
 */
class QuizActivity : ComponentActivity() {
    private var passed = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val engine = (application as GuardApplication).container.guardEngine
        engine.onQuizStarted()

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    QuizScreen(
                        onCorrect = { passed = true },
                        onFinish = { finish() },
                    )
                }
            }
        }
    }

    override fun finish() {
        if (passed) {
            Toast.makeText(this, "按确认键继续观看", Toast.LENGTH_LONG).show()
        }
        super.finish()
    }

    override fun onDestroy() {
        val engine = (application as GuardApplication).container.guardEngine
        if (passed) engine.onQuizPassed() else engine.onQuizCancelled()
        super.onDestroy()
    }
}

@Composable
private fun QuizScreen(onCorrect: () -> Unit, onFinish: () -> Unit) {
    var done by remember { mutableStateOf(false) }
    var message by remember { mutableStateOf<String?>(null) }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(72.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        if (!done) {
            Text(text = "答题时间", fontSize = 30.sp, color = Color(0xFFB0B0B0))
            Spacer(Modifier.height(16.dp))
            Text(text = "3 + 4 = ?", fontSize = 56.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Spacer(Modifier.height(40.dp))
            listOf("5", "7", "9").forEach { option ->
                Button(onClick = {
                    if (option == "7") {
                        done = true
                        onCorrect()
                    } else {
                        message = "再试一次"
                    }
                }) {
                    Text(option, fontSize = 28.sp)
                }
                Spacer(Modifier.height(12.dp))
            }
            message?.let {
                Spacer(Modifier.height(12.dp))
                Text(text = it, fontSize = 22.sp, color = Color(0xFFFF7A7A))
            }
        } else {
            Text(text = "回答正确", fontSize = 44.sp, color = Color(0xFF7CFF9B))
            Spacer(Modifier.height(16.dp))
            Text(text = "观看时长 +20 分钟", fontSize = 28.sp, color = Color.White)
            Spacer(Modifier.height(40.dp))
            Button(onClick = onFinish) {
                Text("继续观看", fontSize = 26.sp)
            }
        }
    }
}
