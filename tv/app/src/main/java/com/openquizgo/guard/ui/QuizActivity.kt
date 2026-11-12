package com.openquizgo.guard.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.compose.setContent
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
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
import com.openquizgo.guard.data.local.LocalQuestion
import com.openquizgo.guard.data.local.QuestionBank
import com.openquizgo.guard.domain.LocalGuardEngine

/**
 * 本地答题界面（077）。
 *
 * 每轮 1 题；答对后清零计时并允许退出，答错停留本题直到答对。
 * 返回键被拦截，Home 键切走后由守护服务看门狗重新拉起。
 */
class QuizActivity : ComponentActivity() {

    private lateinit var engine: LocalGuardEngine
    private lateinit var bank: QuestionBank

    private var question by mutableStateOf<LocalQuestion?>(null)
    private var wrongOptionId by mutableStateOf<String?>(null)
    private var solved by mutableStateOf(false)
    private var attempts by mutableIntStateOf(0)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = (application as GuardApplication).container
        engine = container.guardEngine
        bank = container.questionBank
        isShowing = true
        engine.onQuizStarted()

        question = bank.randomQuestion(engine.lastQuestionId.value)?.also {
            engine.rememberQuestion(it.id)
        }

        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() {
                // 答对前不允许离开答题界面。
            }
        })

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    QuizScreen(
                        question = question,
                        wrongOptionId = wrongOptionId,
                        attempts = attempts,
                        solved = solved,
                        onSelect = ::select,
                        onContinue = { finish() },
                        onDisable = {
                            engine.setEnabled(false)
                            finish()
                        },
                    )
                }
            }
        }
    }

    private fun select(optionId: String) {
        val current = question ?: return
        if (solved) return
        if (optionId == current.answer) {
            solved = true
            wrongOptionId = null
            engine.onQuizPassed(current.id)
        } else {
            attempts += 1
            wrongOptionId = optionId
        }
    }

    override fun onResume() {
        super.onResume()
        isShowing = true
    }

    override fun onPause() {
        isShowing = false
        super.onPause()
    }

    override fun onDestroy() {
        isShowing = false
        if (!solved) {
            engine.onQuizCancelled()
        }
        super.onDestroy()
    }

    companion object {
        @Volatile
        var isShowing: Boolean = false
            private set
    }
}

@Composable
private fun QuizScreen(
    question: LocalQuestion?,
    wrongOptionId: String?,
    attempts: Int,
    solved: Boolean,
    onSelect: (String) -> Unit,
    onContinue: () -> Unit,
    onDisable: () -> Unit,
) {
    val firstOptionFocus = remember { FocusRequester() }
    val continueFocus = remember { FocusRequester() }
    LaunchedEffect(question?.id, solved) {
        if (question != null) {
            runCatching {
                if (solved) continueFocus.requestFocus() else firstOptionFocus.requestFocus()
            }
        }
    }
    if (question == null) {
        Column(
            modifier = Modifier
                .fillMaxSize()
                .background(Color(0xFF0B0B0B))
                .padding(64.dp),
            verticalArrangement = Arrangement.Center,
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Text("本地题库为空", fontSize = 40.sp, color = Color(0xFFFF7A7A))
            Spacer(Modifier.height(16.dp))
            Text(
                "请安装包含题库的版本，或先关闭定时答题",
                fontSize = 22.sp,
                color = Color(0xFFB0B0B0),
            )
            Spacer(Modifier.height(32.dp))
            Button(onClick = onDisable) { Text("关闭定时答题", fontSize = 24.sp) }
        }
        return
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(horizontal = 72.dp, vertical = 24.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "答题时间到",
            fontSize = 22.sp,
            color = Color(0xFFFFC107),
        )
        Spacer(Modifier.height(4.dp))
        Text(
            text = "答对后继续观看",
            fontSize = 16.sp,
            color = Color(0xFF909090),
        )
        Spacer(Modifier.height(16.dp))
        Text(
            text = question.stem,
            fontSize = 34.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        Spacer(Modifier.height(20.dp))

        if (!solved) {
            question.options.forEachIndexed { index, option ->
                val wrong = option.id == wrongOptionId
                Button(
                    onClick = { onSelect(option.id) },
                    modifier = if (index == 0) {
                        Modifier.focusRequester(firstOptionFocus)
                    } else {
                        Modifier
                    },
                ) {
                    Text(
                        text = "${option.id.uppercase()}. ${option.text}",
                        fontSize = 24.sp,
                        color = if (wrong) Color(0xFFFF7A7A) else Color.Unspecified,
                    )
                }
                Spacer(Modifier.height(6.dp))
            }
            if (wrongOptionId != null) {
                Spacer(Modifier.height(4.dp))
                Text(
                    text = if (attempts >= 2) "再读一遍题目，答对才能继续" else "答错了，再试一次",
                    fontSize = 20.sp,
                    color = Color(0xFFFF7A7A),
                )
            }
        } else {
            Text(
                text = "回答正确！",
                fontSize = 34.sp,
                fontWeight = FontWeight.Bold,
                color = Color(0xFF7CFF9B),
            )
            question.explain.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(8.dp))
                Text(text = it, fontSize = 20.sp, color = Color(0xFFD0D0D0))
            }
            Spacer(Modifier.height(20.dp))
            Button(
                onClick = onContinue,
                modifier = Modifier.focusRequester(continueFocus),
            ) {
                Text("继续观看", fontSize = 24.sp)
            }
        }
    }
}
