package com.openquizgo.guard.data.local

import android.content.Context
import com.openquizgo.guard.platform.GuardLog
import org.json.JSONArray
import org.json.JSONObject
import kotlin.random.Random

data class LocalOption(
    val id: String,
    val text: String,
)

data class LocalQuestion(
    val id: String,
    val subject: String,
    val stage: String,
    val difficulty: Int,
    val stem: String,
    val options: List<LocalOption>,
    val answer: String,
    val explain: String,
)

/**
 * 内置题库：随 APK 打包，不依赖网络与学龄筛选。
 *
 * 解析失败的题目会被跳过；全部失败时题库为空，由界面提示，不崩溃。
 */
class QuestionBank private constructor(
    val questions: List<LocalQuestion>,
    private val random: Random = Random.Default,
) {
    val size: Int get() = questions.size

    /** 随机取一题；尽量排除上一轮题目，避免连续重复。 */
    fun randomQuestion(excludeId: String? = null): LocalQuestion? {
        if (questions.isEmpty()) return null
        if (questions.size == 1 || excludeId.isNullOrBlank()) {
            return questions[random.nextInt(questions.size)]
        }
        val pool = questions.filterNot { it.id == excludeId }
        if (pool.isEmpty()) return questions[random.nextInt(questions.size)]
        return pool[random.nextInt(pool.size)]
    }

    companion object {
        const val ASSET_NAME = "questions.json"

        fun load(context: Context): QuestionBank {
            val text = runCatching {
                context.assets.open(ASSET_NAME).bufferedReader(Charsets.UTF_8).use { it.readText() }
            }.getOrElse { error ->
                GuardLog.e("questions", "read $ASSET_NAME failed", error)
                return QuestionBank(emptyList())
            }
            return fromJson(text)
        }

        fun fromJson(text: String): QuestionBank {
            val questions = runCatching { parse(text) }.getOrElse { error ->
                GuardLog.e("questions", "parse questions failed", error)
                emptyList()
            }
            GuardLog.i("questions", "loaded ${questions.size} local questions")
            return QuestionBank(questions)
        }

        private fun parse(text: String): List<LocalQuestion> {
            val root = JSONObject(text)
            val array = root.optJSONArray("questions") ?: JSONArray()
            val result = ArrayList<LocalQuestion>(array.length())
            for (index in 0 until array.length()) {
                val item = array.optJSONObject(index) ?: continue
                val question = parseQuestion(item) ?: continue
                result += question
            }
            return result
        }

        private fun parseQuestion(item: JSONObject): LocalQuestion? {
            val id = item.optString("id").trim()
            val stem = item.optString("stem").trim()
            val answer = item.optString("answer").trim()
            if (id.isEmpty() || stem.isEmpty() || answer.isEmpty()) return null

            val optionsJson = item.optJSONArray("options") ?: return null
            val options = ArrayList<LocalOption>(optionsJson.length())
            for (index in 0 until optionsJson.length()) {
                val option = optionsJson.optJSONObject(index) ?: continue
                val optionId = option.optString("id").trim()
                val optionText = option.optString("text").trim()
                if (optionId.isEmpty() || optionText.isEmpty()) continue
                options += LocalOption(optionId, optionText)
            }
            if (options.size != 4) return null
            if (options.none { it.id == answer }) return null
            if (options.map { it.text }.toSet().size != options.size) return null

            return LocalQuestion(
                id = id,
                subject = item.optString("subject", "general"),
                stage = item.optString("stage", "low"),
                difficulty = item.optInt("difficulty", 1).coerceIn(1, 5),
                stem = stem,
                options = options,
                answer = answer,
                explain = item.optString("explain").trim(),
            )
        }
    }
}
