package com.openquizgo.guard.data.local

import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNotEquals
import org.junit.Assert.assertTrue
import org.junit.Test

class QuestionBankTest {

    private fun assetText(): String =
        File("src/main/assets/questions.json").readText(Charsets.UTF_8)

    @Test
    fun `内置题库不少于 100 道且结构合法`() {
        val bank = QuestionBank.fromJson(assetText())
        assertTrue("题库数量不足：${bank.size}", bank.size >= 100)

        val ids = mutableSetOf<String>()
        bank.questions.forEach { question ->
            assertTrue("题目 ID 重复：${question.id}", ids.add(question.id))
            assertEquals("选项数量错误：${question.id}", 4, question.options.size)
            assertEquals(
                "选项文本重复：${question.id}",
                4,
                question.options.map { it.text }.toSet().size,
            )
            assertTrue(
                "正确答案不在选项中：${question.id}",
                question.options.any { it.id == question.answer },
            )
            assertTrue("题干为空：${question.id}", question.stem.isNotBlank())
            assertTrue("解析为空：${question.id}", question.explain.isNotBlank())
        }
    }

    @Test
    fun `随机出题不会连续重复`() {
        val bank = QuestionBank.fromJson(assetText())
        val last = bank.questions.first().id
        repeat(100) {
            val question = bank.randomQuestion(last)
            assertNotNull(question)
            assertNotEquals(last, question!!.id)
        }
    }

    @Test
    fun `空题库不崩溃`() {
        val bank = QuestionBank.fromJson("""{"questions":[]}""")
        assertEquals(0, bank.size)
        assertEquals(null, bank.randomQuestion())
    }

    @Test
    fun `无效题目被跳过`() {
        val bank = QuestionBank.fromJson(
            """
            {"questions":[
              {"id":"bad","stem":"x","options":[],"answer":"a"},
              {"id":"ok","stem":"1+1=?","options":[
                {"id":"a","text":"1"},{"id":"b","text":"2"},
                {"id":"c","text":"3"},{"id":"d","text":"4"}],
                "answer":"b","explain":"2"}
            ]}
            """.trimIndent(),
        )
        assertEquals(1, bank.size)
        assertEquals("ok", bank.questions.first().id)
    }
}
