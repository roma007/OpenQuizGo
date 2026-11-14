package com.openquizgo.guard.ui

import android.media.MediaPlayer
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
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
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import com.openquizgo.guard.GuardApplication
import com.openquizgo.guard.data.battle.BattleRepository
import com.openquizgo.guard.data.remote.BattleAnswer
import com.openquizgo.guard.data.remote.BattleQuestion
import com.openquizgo.guard.data.remote.BattleState
import com.openquizgo.guard.platform.GuardLog
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

/**
 * TV 端加入亲子对战房间（孩子侧）。
 *
 * 家长在家长端创建 room 后，电视输入 6 位房间码加入并完成孩子答题；
 * 提交后通过 SSE 接收状态变化（失败时退化为轮询），展示最终比分。
 */
class BattleActivity : ComponentActivity() {
    private lateinit var repository: BattleRepository
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)
    private var liveJob: Job? = null
    private var lastVersion = 0L

    private var screen by mutableStateOf(Screen.JOIN)
    private var code by mutableStateOf("")
    private var battle by mutableStateOf<BattleState?>(null)
    private var selections by mutableStateOf<Map<String, String>>(emptyMap())
    private var error by mutableStateOf<String?>(null)
    private var busy by mutableStateOf(false)

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        repository = (application as GuardApplication).container.battleRepository
        GuardLog.i("battle", "join screen opened")
        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    when (screen) {
                        Screen.JOIN -> JoinScreen(
                            code = code,
                            error = error,
                            busy = busy,
                            onChar = { if (code.length < 6) code += it },
                            onDelete = { if (code.isNotEmpty()) code = code.dropLast(1) },
                            onClear = { code = "" },
                            onJoin = ::join,
                            onBack = { finish() },
                        )
                        Screen.PLAY -> PlayScreen(
                            state = battle,
                            selections = selections,
                            busy = busy,
                            error = error,
                            onSelect = { questionId, optionId ->
                                selections = selections + (questionId to optionId)
                            },
                            onSubmit = ::submit,
                        )
                        Screen.WAIT -> WaitScreen(battle = battle)
                        Screen.RESULT -> ResultScreen(state = battle) { finish() }
                    }
                }
            }
        }
    }

    private fun join() {
        if (code.length != 6 || busy) return
        busy = true
        error = null
        scope.launch {
            runCatching { repository.join(code) }
                .onSuccess { state ->
                    GuardLog.i("battle", "joined id=${state.id} status=${state.status}")
                    battle = state
                    selections = emptyMap()
                    screen = if (state.status == "finished") Screen.RESULT else Screen.PLAY
                }
                .onFailure { error = it.message ?: "加入失败" }
            busy = false
        }
    }

    private fun submit() {
        val state = battle ?: return
        val questions = state.questions.orEmpty()
        val answers = questions.mapNotNull { question ->
            val selected = selections[question.id] ?: return@mapNotNull null
            BattleAnswer(questionId = question.id, answer = listOf(selected))
        }
        if (answers.size != questions.size) {
            error = "请完成全部题目后再提交"
            return
        }
        busy = true
        error = null
        scope.launch {
            runCatching { repository.submit(state.id, answers) }
                .onSuccess { next ->
                    battle = next
                    screen = if (next.status == "finished") Screen.RESULT else Screen.WAIT
                    if (screen == Screen.WAIT) startLiveUpdates(next.id)
                }
                .onFailure { error = it.message ?: "提交失败" }
            busy = false
        }
    }

    /**
     * 优先使用 SSE：版本号变化时拉取最新状态；
     * 流断开后立即刷新一次并按 3 秒重连，重连期间等价于轮询兜底。
     */
    private fun startLiveUpdates(battleId: String) {
        liveJob?.cancel()
        lastVersion = 0L
        liveJob = scope.launch {
            while (isActive) {
                try {
                    repository.events(battleId).collect { version ->
                        if (version > lastVersion) {
                            lastVersion = version
                            if (refresh(battleId)) {
                                liveJob?.cancel()
                            }
                        }
                    }
                } catch (e: CancellationException) {
                    return@launch
                } catch (e: Exception) {
                    error = e.message ?: "事件流中断"
                }
                if (!isActive || screen != Screen.WAIT) return@launch
                if (refresh(battleId)) return@launch
                delay(3_000)
            }
        }
    }

    private suspend fun refresh(battleId: String): Boolean {
        val next = runCatching { repository.state(battleId) }.getOrElse {
            error = it.message ?: "状态刷新失败"
            return false
        }
        battle = next
        if (next.status == "finished") {
            screen = Screen.RESULT
            GuardLog.i("battle", "finished child=${next.childScore} parent=${next.parentScore}")
            return true
        }
        return false
    }

    override fun onDestroy() {
        liveJob?.cancel()
        scope.cancel()
        super.onDestroy()
    }

    private enum class Screen { JOIN, PLAY, WAIT, RESULT }
}

@Composable
private fun JoinScreen(
    code: String,
    error: String?,
    busy: Boolean,
    onChar: (String) -> Unit,
    onDelete: () -> Unit,
    onClear: () -> Unit,
    onJoin: () -> Unit,
    onBack: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0B0B0B)).padding(48.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("加入亲子对战", fontSize = 40.sp, fontWeight = FontWeight.Bold, color = Color.White)
        Spacer(Modifier.height(8.dp))
        Text("输入家长端创建的 6 位房间码", fontSize = 20.sp, color = Color(0xFFB0B0B0))
        Spacer(Modifier.height(16.dp))
        Text(
            text = code.padEnd(6, '·').toCharArray().joinToString(" "),
            fontSize = 40.sp,
            color = Color(0xFF7CC7FF),
        )
        error?.let {
            Spacer(Modifier.height(8.dp))
            Text(it, fontSize = 18.sp, color = Color(0xFFFFC107))
        }
        Spacer(Modifier.height(16.dp))
        for (row in listOf("23456789", "ABCDEFGH", "JKLMNPQR", "STUVWXYZ")) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                row.forEach { char ->
                    Button(onClick = { onChar(char.toString()) }, modifier = Modifier.widthIn(min = 52.dp)) {
                        Text(char.toString(), fontSize = 22.sp)
                    }
                }
            }
            Spacer(Modifier.height(8.dp))
        }
        Spacer(Modifier.height(8.dp))
        Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            Button(onClick = onDelete, modifier = Modifier.widthIn(min = 100.dp)) { Text("删除") }
            Button(onClick = onClear, modifier = Modifier.widthIn(min = 100.dp)) { Text("清空") }
            Button(onClick = onJoin, enabled = code.length == 6 && !busy, modifier = Modifier.widthIn(min = 180.dp)) {
                Text(if (busy) "加入中…" else "加入", fontSize = 20.sp)
            }
            Button(onClick = onBack, modifier = Modifier.widthIn(min = 100.dp)) { Text("返回") }
        }
    }
}

@Composable
private fun PlayScreen(
    state: BattleState?,
    selections: Map<String, String>,
    busy: Boolean,
    error: String?,
    onSelect: (String, String) -> Unit,
    onSubmit: () -> Unit,
) {
    val questions = state?.questions.orEmpty()
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
        modifier = Modifier.fillMaxSize().background(Color(0xFF0B0B0B)).padding(40.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("你的回合", fontSize = 34.sp, fontWeight = FontWeight.Bold, color = Color.White)
        Text("完成全部题目后提交，等待家长作答", fontSize = 18.sp, color = Color(0xFFB0B0B0))
        error?.let { Text(it, fontSize = 18.sp, color = Color(0xFFFFC107)) }
        Spacer(Modifier.height(12.dp))
        LazyColumn(
            modifier = Modifier.fillMaxWidth().weight(1f),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            items(questions) { question ->
                QuestionBlock(
                    question = question,
                    selected = selections[question.id],
                    onSelect = { onSelect(question.id, it) },
                    onPlayAudio = ::playAudio,
                )
            }
        }
        Spacer(Modifier.height(12.dp))
        Button(onClick = onSubmit, enabled = !busy, modifier = Modifier.widthIn(min = 240.dp)) {
            Text(if (busy) "提交中…" else "提交答案", fontSize = 22.sp)
        }
    }
}

@Composable
private fun QuestionBlock(
    question: BattleQuestion,
    selected: String?,
    onSelect: (String) -> Unit,
    onPlayAudio: (String) -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxWidth().background(Color(0xFF161616)).padding(16.dp),
    ) {
        Text(question.stem, fontSize = 24.sp, color = Color.White)
        question.stemMedia?.url?.takeIf { it.isNotBlank() }?.let { path ->
            Spacer(Modifier.height(8.dp))
            RemoteImage(resolveMediaUrl(path), contentDescription = "题干图片")
        }
        Spacer(Modifier.height(8.dp))
        question.options.orEmpty().forEach { option ->
            val mark = if (selected == option.id) "✓" else "○"
            val images = option.images.orEmpty()
                .filter { it.isNotBlank() }
                .ifEmpty { listOfNotNull(option.image?.takeIf { it.isNotBlank() }) }
                .take(3)
            option.audio?.takeIf { it.isNotBlank() }?.let { path ->
                Button(onClick = { onPlayAudio(path) }, modifier = Modifier.fillMaxWidth()) {
                    Text("▶ 播放选项 ${option.id} 音频", fontSize = 20.sp)
                }
                Spacer(Modifier.height(6.dp))
            }
            Button(
                onClick = { onSelect(option.id) },
                modifier = Modifier.fillMaxWidth(),
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
                    Text("$mark ${option.id}. ${option.text}", fontSize = 20.sp)
                }
            }
            Spacer(Modifier.height(6.dp))
        }
    }
}

@Composable
private fun WaitScreen(battle: BattleState?) {
    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0B0B0B)).padding(48.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("等待家长作答…", fontSize = 36.sp, color = Color(0xFF7CC7FF))
        Spacer(Modifier.height(12.dp))
        Text("孩子得分：${battle?.childScore ?: 0}", fontSize = 22.sp, color = Color.White)
    }
}

@Composable
private fun ResultScreen(state: BattleState?, onBack: () -> Unit) {
    val result = state?.result
    val child = result?.childScore ?: state?.childScore ?: 0
    val parent = result?.parentScore ?: state?.parentScore ?: 0
    val title = when (result?.winner) {
        "child" -> "孩子获胜！"
        "parent" -> "家长获胜"
        "draw" -> "平局"
        else -> "对战结束"
    }
    Column(
        modifier = Modifier.fillMaxSize().background(Color(0xFF0B0B0B)).padding(48.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(title, fontSize = 44.sp, fontWeight = FontWeight.Bold, color = Color(0xFF7CFF9B))
        Spacer(Modifier.height(16.dp))
        Text("孩子 $child : $parent 家长", fontSize = 32.sp, color = Color.White)
        Spacer(Modifier.height(24.dp))
        Button(onClick = onBack, modifier = Modifier.widthIn(min = 180.dp)) {
            Text("返回", fontSize = 22.sp)
        }
    }
}
