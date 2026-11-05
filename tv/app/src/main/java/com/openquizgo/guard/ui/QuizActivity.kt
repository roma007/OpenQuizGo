package com.openquizgo.guard.ui

import android.graphics.BitmapFactory
import android.media.MediaPlayer
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import com.openquizgo.guard.BuildConfig
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.data.quiz.QuizApiException
import com.openquizgo.guard.data.quiz.QuizOutcome
import com.openquizgo.guard.data.quiz.QuizRepository
import com.openquizgo.guard.data.quiz.QuizSession
import com.openquizgo.guard.data.remote.AnswerResult
import com.openquizgo.guard.data.remote.SessionQuestion
import com.openquizgo.guard.domain.GuardEngine
import com.openquizgo.guard.domain.GrantInfo
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.net.URL

/**
 * 半透明全屏答题界面（P2-T6）。
 *
 * 关键：Activity 使用半透明主题，播放器只进入 paused 而不是被 stop；
 * 用户先按确认键暂停，App 监听到后再拉起本界面，因此无广告、进度保留。
 *
 * 题目来自服务端答题会话：创建会话 → 逐题提交 → 结算并签发授权。
 */
class QuizActivity : ComponentActivity() {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private lateinit var repository: QuizRepository
    private lateinit var engine: GuardEngine

    private var session: QuizSession? = null
    private var pending: List<SessionQuestion> = emptyList()
    private var answeredBefore = 0
    private var questionIndex = 0

    /** 是否已把结果写回状态机（避免 onDestroy 覆盖）。 */
    private var outcomeApplied = false

    private var loading by mutableStateOf(true)
    private var errorMessage by mutableStateOf<String?>(null)
    private var question by mutableStateOf<SessionQuestion?>(null)
    private var answerResult by mutableStateOf<AnswerResult?>(null)
    private var submitting by mutableStateOf(false)
    private var outcome by mutableStateOf<QuizOutcome?>(null)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        repository = (application as GuardApplication).container.quizRepository
        engine = (application as GuardApplication).container.guardEngine
        engine.onQuizStarted()

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    val currentOutcome = outcome
                    val currentError = errorMessage
                    val currentQuestion = question
                    when {
                        currentOutcome != null -> ResultScreen(
                            outcome = currentOutcome,
                            onClose = { finish() },
                        )
                        currentError != null -> MessageScreen(
                            title = "网络异常",
                            detail = currentError,
                            primary = "重试" to {
                                errorMessage = null
                                loadSession()
                            },
                            secondary = "退出" to { finish() },
                        )
                        loading -> MessageScreen(
                            title = "正在获取题目…",
                            detail = "请稍候",
                            primary = null,
                            secondary = null,
                        )
                        currentQuestion != null -> QuestionScreen(
                            question = currentQuestion,
                            index = answeredBefore + questionIndex + 1,
                            total = session?.questions?.size ?: (questionIndex + 1),
                            result = answerResult,
                            submitting = submitting,
                            onSelect = { optionId -> submitAnswer(optionId) },
                            onNext = { advance() },
                        )
                        else -> EmptyScreen("暂无题目")
                    }
                }
            }
        }
        loadSession()
    }

    private fun loadSession() {
        loading = true
        question = null
        answerResult = null
        scope.launch {
            runCatching { repository.startSession() }
                .onSuccess { started ->
                    session = started
                    pending = started.questions.filterNot { it.answered }
                    answeredBefore = started.questions.size - pending.size
                    questionIndex = 0
                    GuardLog.i(
                        "quiz",
                        "session=${started.sessionId} questions=${started.questions.size} " +
                            "pending=${pending.size}",
                    )
                    loading = false
                    if (pending.isEmpty()) {
                        settle()
                    } else {
                        showCurrent()
                    }
                }
                .onFailure { error ->
                    GuardLog.w("quiz", "session failed: ${error.message}")
                    loading = false
                    errorMessage = readable(error)
                }
        }
    }

    private fun showCurrent() {
        answerResult = null
        submitting = false
        question = pending.getOrNull(questionIndex)
    }

    private fun submitAnswer(optionId: String) {
        val current = session ?: return
        val currentQuestion = pending.getOrNull(questionIndex) ?: return
        if (submitting || answerResult != null) return
        submitting = true
        scope.launch {
            runCatching {
                repository.submitAnswer(current.sessionId, currentQuestion.token, listOf(optionId))
            }
                .onSuccess { result ->
                    submitting = false
                    answerResult = result
                    GuardLog.i(
                        "quiz",
                        "answer pos=$questionIndex correct=${result.correct} " +
                            "score=${result.score}/${result.threshold}",
                    )
                }
                .onFailure { error ->
                    submitting = false
                    GuardLog.w("quiz", "answer failed: ${error.message}")
                    errorMessage = readable(error)
                }
        }
    }

    private fun advance() {
        if (questionIndex + 1 < pending.size) {
            questionIndex += 1
            showCurrent()
        } else {
            question = null
            settle()
        }
    }

    private fun settle() {
        val current = session ?: return
        loading = true
        scope.launch {
            runCatching { repository.finish(current.sessionId) }
                .onSuccess { summary ->
                    val passed = summary.state == "passed" && summary.grant != null
                    GuardLog.i(
                        "quiz",
                        "finished state=${summary.state} score=${summary.score}/${summary.passThreshold} " +
                            "grant=${summary.grant?.minutes ?: 0}",
                    )
                    outcomeApplied = true
                    if (passed) {
                        val grant = summary.grant
                        val info = grant?.grantId?.let { id ->
                            GrantInfo(
                                grantId = id,
                                minutes = grant.minutes,
                                remainingSeconds = grant.remainingSeconds ?: (grant.minutes * 60),
                                validUntilMillis = GuardEngine.parseMillis(grant.validUntil)
                                    ?: (System.currentTimeMillis() + grant.minutes * 60_000L),
                            )
                        }
                        engine.onQuizPassed(info)
                    } else {
                        engine.onQuizFailed()
                    }
                    loading = false
                    outcome = QuizOutcome(summary = summary, passed = passed)
                }
                .onFailure { error ->
                    GuardLog.w("quiz", "finish failed: ${error.message}")
                    loading = false
                    errorMessage = readable(error)
                }
        }
    }

    private fun readable(error: Throwable): String = when (error) {
        is QuizApiException -> error.message
        else -> error.message ?: "未知错误"
    }

    override fun onDestroy() {
        if (!outcomeApplied) {
            engine.onQuizCancelled()
        }
        scope.cancel()
        super.onDestroy()
    }
}

private fun resolveMediaUrl(path: String): String {
    return if (path.startsWith("http")) path else BuildConfig.API_BASE_URL.trimEnd('/') + path
}

@Composable
private fun RemoteImage(
    url: String,
    maxHeight: Dp = 240.dp,
    modifier: Modifier = Modifier.fillMaxWidth(),
    contentDescription: String = "图片",
) {
    var bitmap by remember(url) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    var failed by remember(url) { mutableStateOf(false) }
    LaunchedEffect(url) {
        val loaded = withContext(Dispatchers.IO) {
            runCatching {
                URL(url).openStream().use { BitmapFactory.decodeStream(it)?.asImageBitmap() }
            }.getOrNull()
        }
        if (loaded == null) failed = true else bitmap = loaded
    }
    val image = bitmap
    if (image != null) {
        Image(
            bitmap = image,
            contentDescription = contentDescription,
            modifier = modifier.heightIn(max = maxHeight),
            contentScale = ContentScale.Fit,
        )
    } else if (failed) {
        Text("图片加载失败", fontSize = 20.sp, color = Color(0xFFFFC107))
    } else {
        Text("图片加载中…", fontSize = 20.sp, color = Color(0xFFB0B0B0))
    }
}

@Composable
private fun QuestionScreen(
    question: SessionQuestion,
    index: Int,
    total: Int,
    result: AnswerResult?,
    submitting: Boolean,
    onSelect: (String) -> Unit,
    onNext: () -> Unit,
) {
    val context = LocalContext.current
    var audioPlayer by remember { mutableStateOf<MediaPlayer?>(null) }

    fun playAudio(path: String) {
        audioPlayer?.release()
        audioPlayer = MediaPlayer().apply {
            try {
                setDataSource(resolveMediaUrl(path))
                setOnPreparedListener { it.start() }
                setOnErrorListener { _, _, _ -> true }
                prepareAsync()
            } catch (_: Exception) {
                release()
                audioPlayer = null
            }
        }
    }

    DisposableEffect(Unit) {
        onDispose {
            audioPlayer?.release()
            audioPlayer = null
        }
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "答题时间 · 第 $index / $total 题",
            fontSize = 26.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(20.dp))
        Text(
            text = question.stem,
            fontSize = 44.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        question.stemMedia?.url?.takeIf { it.isNotBlank() }?.let { path ->
            Spacer(Modifier.height(16.dp))
            RemoteImage(resolveMediaUrl(path), contentDescription = "题干图片")
        }
        Spacer(Modifier.height(32.dp))

        if (result == null) {
            val options = question.options.orEmpty()
            if (options.isEmpty()) {
                Text("暂不支持该题型", fontSize = 24.sp, color = Color(0xFFFFC107))
            } else {
                options.forEach { option ->
                    val images = option.images.orEmpty()
                        .filter { it.isNotBlank() }
                        .ifEmpty { listOfNotNull(option.image?.takeIf { it.isNotBlank() }) }
                        .take(3)
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        option.audio?.takeIf { it.isNotBlank() }?.let { path ->
                            Button(
                                onClick = { playAudio(path) },
                                enabled = !submitting,
                            ) {
                                Text(text = "▶ 播放选项 ${option.id} 音频", fontSize = 22.sp)
                            }
                            Spacer(Modifier.height(8.dp))
                        }
                        Button(
                            onClick = { onSelect(option.id) },
                            enabled = !submitting,
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                if (images.isNotEmpty()) {
                                    Row(
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        images.forEach { path ->
                                            RemoteImage(
                                                url = resolveMediaUrl(path),
                                                maxHeight = 90.dp,
                                                modifier = Modifier
                                                    .heightIn(max = 90.dp)
                                                    .widthIn(max = 200.dp),
                                                contentDescription = "选项图片",
                                            )
                                        }
                                    }
                                    Spacer(Modifier.height(6.dp))
                                }
                                Text(text = "${option.id}. ${option.text}", fontSize = 26.sp)
                            }
                        }
                    }
                    Spacer(Modifier.height(12.dp))
                }
            }
            if (submitting) {
                Spacer(Modifier.height(12.dp))
                Text("提交中…", fontSize = 20.sp, color = Color(0xFFB0B0B0))
            }
        } else {
            Text(
                text = if (result.correct) "回答正确" else "回答错误",
                fontSize = 40.sp,
                fontWeight = FontWeight.Bold,
                color = if (result.correct) Color(0xFF7CFF9B) else Color(0xFFFF7A7A),
            )
            result.explain?.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(12.dp))
                Text(text = it, fontSize = 24.sp, color = Color(0xFFD0D0D0))
            }
            Spacer(Modifier.height(8.dp))
            Text(
                text = "当前得分 ${formatScore(result.score)} / 通过 ${formatScore(result.threshold)}",
                fontSize = 20.sp,
                color = Color(0xFF7CC7FF),
            )
            Spacer(Modifier.height(28.dp))
            Button(onClick = onNext) {
                Text(
                    text = if (result.remaining > 0) "下一题" else "查看结果",
                    fontSize = 26.sp,
                )
            }
        }
    }
}

@Composable
private fun ResultScreen(outcome: QuizOutcome, onClose: () -> Unit) {
    val summary = outcome.summary
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = if (outcome.passed) "挑战成功" else "挑战失败",
            fontSize = 52.sp,
            fontWeight = FontWeight.Bold,
            color = if (outcome.passed) Color(0xFF7CFF9B) else Color(0xFFFF7A7A),
        )
        Spacer(Modifier.height(20.dp))
        Text(
            text = "答对 ${summary.correctCount} / ${summary.questionCount} 题 " +
                "· 得分 ${formatScore(summary.score)} / ${formatScore(summary.passThreshold)}",
            fontSize = 26.sp,
            color = Color(0xFFD0D0D0),
        )
        if (outcome.passed) {
            Spacer(Modifier.height(16.dp))
            Text(
                text = "本次获得 ${outcome.grantMinutes} 分钟观看时间",
                fontSize = 30.sp,
                color = Color(0xFFFFD54F),
            )
        } else {
            summary.grantError?.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(12.dp))
                Text(text = it, fontSize = 20.sp, color = Color(0xFFFFC107))
            }
            Spacer(Modifier.height(12.dp))
            Text(
                text = "休息一下，稍后再来挑战吧",
                fontSize = 24.sp,
                color = Color(0xFFB0B0B0),
            )
        }
        Spacer(Modifier.height(40.dp))
        Row(horizontalArrangement = Arrangement.Center) {
            Button(onClick = onClose, modifier = Modifier.widthIn(min = 220.dp)) {
                Text(
                    text = if (outcome.passed) "继续观看" else "结束答题",
                    fontSize = 28.sp,
                )
            }
        }
    }
}

@Composable
private fun MessageScreen(
    title: String,
    detail: String,
    primary: Pair<String, () -> Unit>?,
    secondary: Pair<String, () -> Unit>?,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B))
            .padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = title, fontSize = 40.sp, fontWeight = FontWeight.Bold, color = Color.White)
        Spacer(Modifier.height(16.dp))
        Text(text = detail, fontSize = 24.sp, color = Color(0xFFB0B0B0))
        Spacer(Modifier.height(40.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(24.dp)) {
            primary?.let { (label, action) ->
                Button(onClick = action) { Text(label, fontSize = 26.sp) }
            }
            secondary?.let { (label, action) ->
                Button(onClick = action) { Text(label, fontSize = 26.sp) }
            }
        }
    }
}

@Composable
private fun EmptyScreen(text: String) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF0B0B0B)),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = text, fontSize = 32.sp, color = Color(0xFFB0B0B0))
    }
}

private fun formatScore(value: Double): String =
    if (value == value.toInt().toDouble()) value.toInt().toString() else "%.1f".format(value)
