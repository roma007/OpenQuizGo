package com.openquizgo.guard.ui.pad

import android.media.MediaPlayer
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.openquizgo.guard.data.quiz.QuizOutcome
import com.openquizgo.guard.data.remote.AnswerResult
import com.openquizgo.guard.data.remote.SessionQuestion

/**
 * 084：平板答题界面（触摸）。
 *
 * 题型分工：
 * - choice / image：点选项即提交
 * - fill / voice：输入文字后提交
 * - sort：按顺序点，再提交
 * - match：服务端要求 map 答案，现有接口不支持，给退出通道
 */

private val TEXT_TYPES = setOf("fill", "voice")

@Composable
internal fun PadQuestionScreen(
    question: SessionQuestion,
    index: Int,
    total: Int,
    result: AnswerResult?,
    submitting: Boolean,
    onAnswer: (List<String>) -> Unit,
    onNext: () -> Unit,
    onCancel: () -> Unit,
) {
    PadPage(
        horizontalAlignment = Alignment.Start,
    ) {
        Text(
            text = "第 $index / $total 题",
            fontSize = 13.sp,
            color = PadTextSecondary,
            modifier = Modifier.fillMaxWidth(),
            textAlign = TextAlign.Center,
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = question.stem,
            fontSize = 22.sp,
            fontWeight = FontWeight.Bold,
            color = PadTextPrimary,
        )
        question.stemMedia?.url?.takeIf { it.isNotBlank() }?.let { path ->
            Spacer(Modifier.height(12.dp))
            PadRemoteImage(padResolveMediaUrl(path), contentDescription = "题干图片")
        }
        Spacer(Modifier.height(16.dp))

        if (result == null) {
            when (question.type) {
                in TEXT_TYPES -> PadTextAnswer(submitting = submitting, onSubmit = onAnswer)
                "sort" -> PadSortAnswer(question = question, submitting = submitting, onSubmit = onAnswer)
                "match" -> {
                    PadBanner(text = "这道题需要在电视上作答，平板暂时做不了。")
                    Spacer(Modifier.height(10.dp))
                    PadSecondaryButton(
                        text = "退出答题",
                        onClick = onCancel,
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                else -> PadChoiceAnswer(question = question, submitting = submitting, onAnswer = onAnswer)
            }
            if (submitting) {
                Spacer(Modifier.height(10.dp))
                Text("提交中…", fontSize = 13.sp, color = PadTextSecondary)
            }
        } else {
            Spacer(Modifier.height(8.dp))
            Text(
                text = if (result.correct) "答对了" else "答错了",
                fontSize = 24.sp,
                fontWeight = FontWeight.Bold,
                color = if (result.correct) PadGood else PadBad,
            )
            val correctAnswer = result.correctAnswer.orEmpty().filter { it.isNotBlank() }
            if (!result.correct && correctAnswer.isNotEmpty()) {
                Spacer(Modifier.height(6.dp))
                Text(
                    text = "正确答案：${correctAnswer.joinToString("、")}",
                    fontSize = 14.sp,
                    color = PadTextSecondary,
                )
            }
            result.explain?.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(8.dp))
                Text(text = it, fontSize = 14.sp, color = Color(0xFFD0D0D0))
            }
            Spacer(Modifier.height(6.dp))
            Text(
                text = "已答 ${result.answered} 题，还剩 ${result.remaining} 题",
                fontSize = 12.sp,
                color = PadAccent,
            )
            Spacer(Modifier.height(18.dp))
            PadPrimaryButton(
                text = if (result.remaining > 0) "下一题" else "查看结果",
                onClick = onNext,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@Composable
private fun PadChoiceAnswer(
    question: SessionQuestion,
    submitting: Boolean,
    onAnswer: (List<String>) -> Unit,
) {
    val options = question.options.orEmpty()
    if (options.isEmpty()) {
        PadBanner(text = "这道题没有可选项，请让家长解锁。")
        return
    }
    var audioPlayer by remember { mutableStateOf<MediaPlayer?>(null) }
    DisposableEffect(Unit) {
        onDispose {
            audioPlayer?.release()
            audioPlayer = null
        }
    }

    options.forEach { option ->
        val images = option.images.orEmpty()
            .filter { it.isNotBlank() }
            .ifEmpty { listOfNotNull(option.image?.takeIf { it.isNotBlank() }) }
            .take(3)
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .background(Color(0xFF161616), MaterialTheme.shapes.medium)
                .clickable(enabled = !submitting) { onAnswer(listOf(option.id)) }
                .padding(14.dp),
        ) {
            Text(
                text = "${option.id}. ${option.text}",
                fontSize = 17.sp,
                color = PadTextPrimary,
            )
            if (images.isNotEmpty()) {
                Spacer(Modifier.height(8.dp))
                images.forEach { path ->
                    PadRemoteImage(padResolveMediaUrl(path), maxHeight = 120.dp, contentDescription = "选项图片")
                }
            }
            option.audio?.takeIf { it.isNotBlank() }?.let { path ->
                Spacer(Modifier.height(6.dp))
                Text(
                    text = "▶ 播放选项音频",
                    fontSize = 13.sp,
                    color = PadAccent,
                    modifier = Modifier.clickable {
                        audioPlayer?.release()
                        audioPlayer = MediaPlayer().apply {
                            runCatching {
                                setDataSource(padResolveMediaUrl(path))
                                setOnPreparedListener { it.start() }
                                setOnErrorListener { _, _, _ -> true }
                                prepareAsync()
                            }.onFailure { release() }
                        }
                    },
                )
            }
        }
        Spacer(Modifier.height(10.dp))
    }
}

@Composable
private fun PadTextAnswer(submitting: Boolean, onSubmit: (List<String>) -> Unit) {
    var text by remember { mutableStateOf("") }
    OutlinedTextField(
        value = text,
        onValueChange = { text = it },
        label = { Text("在这里输入答案") },
        modifier = Modifier.fillMaxWidth(),
        minLines = 1,
        maxLines = 3,
        singleLine = false,
    )
    Spacer(Modifier.height(12.dp))
    PadPrimaryButton(
        text = "提交答案",
        onClick = { onSubmit(listOf(text.trim())) },
        enabled = !submitting && text.isNotBlank(),
        modifier = Modifier.fillMaxWidth(),
    )
}

@Composable
private fun PadSortAnswer(
    question: SessionQuestion,
    submitting: Boolean,
    onSubmit: (List<String>) -> Unit,
) {
    val options = question.options.orEmpty()
    if (options.isEmpty()) {
        PadBanner(text = "这道题没有可选项，请让家长解锁。")
        return
    }
    val picked = remember(question.token) { mutableStateListOf<String>() }
    Text(
        text = "按正确顺序依次点选（点错可以重来）",
        fontSize = 13.sp,
        color = PadTextSecondary,
    )
    Spacer(Modifier.height(8.dp))
    options.forEach { option ->
        val order = picked.indexOf(option.id)
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .background(
                    if (order >= 0) PadAccent.copy(alpha = 0.16f) else Color(0xFF161616),
                    MaterialTheme.shapes.medium,
                )
                .clickable(enabled = !submitting) {
                    if (order >= 0) picked.removeAt(order) else picked.add(option.id)
                }
                .padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Text(
                text = if (order >= 0) "${order + 1}" else "·",
                fontSize = 16.sp,
                fontWeight = FontWeight.Bold,
                color = if (order >= 0) PadAccent else PadTextSecondary,
            )
            Spacer(Modifier.width(12.dp))
            Text(text = "${option.id}. ${option.text}", fontSize = 17.sp, color = PadTextPrimary)
        }
        Spacer(Modifier.height(10.dp))
    }
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
        PadPrimaryButton(
            text = "提交顺序",
            onClick = { onSubmit(picked.toList()) },
            enabled = !submitting && picked.size == options.size,
            modifier = Modifier.weight(1f),
        )
        PadSecondaryButton(
            text = "重选",
            onClick = { picked.clear() },
            enabled = !submitting && picked.isNotEmpty(),
        )
    }
}

@Composable
internal fun PadQuizResultScreen(outcome: QuizOutcome, onClose: () -> Unit) {
    val summary = outcome.summary
    PadPage {
        Spacer(Modifier.height(40.dp))
        Text(
            text = if (outcome.passed) "挑战成功" else "挑战失败",
            fontSize = 30.sp,
            fontWeight = FontWeight.Bold,
            color = if (outcome.passed) PadGood else PadBad,
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        Spacer(Modifier.height(12.dp))
        Text(
            text = "答对 ${summary.correctCount} / ${summary.questionCount} 题 · 通过需要 ${summary.passQuestions} 题",
            fontSize = 15.sp,
            color = Color(0xFFD0D0D0),
            textAlign = TextAlign.Center,
            modifier = Modifier.fillMaxWidth(),
        )
        if (outcome.passed) {
            Spacer(Modifier.height(12.dp))
            Text(
                text = "本次获得 ${outcome.grantMinutes} 分钟观看时间",
                fontSize = 18.sp,
                color = Color(0xFFFFD54F),
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth(),
            )
        } else {
            summary.grantError?.takeIf { it.isNotBlank() }?.let {
                Spacer(Modifier.height(10.dp))
                Text(
                    text = it,
                    fontSize = 13.sp,
                    color = PadWarn,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
            Spacer(Modifier.height(10.dp))
            Text(
                text = "休息一下，稍后再来挑战吧",
                fontSize = 14.sp,
                color = PadTextSecondary,
                textAlign = TextAlign.Center,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        Spacer(Modifier.height(28.dp))
        PadPrimaryButton(
            text = if (outcome.passed) "继续观看" else "结束答题",
            onClick = onClose,
            modifier = Modifier.fillMaxWidth(),
        )
    }
}
