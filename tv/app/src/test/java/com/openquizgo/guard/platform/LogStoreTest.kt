package com.openquizgo.guard.platform

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File

class LogStoreTest {

    @get:Rule
    val temp = TemporaryFolder()

    @Test
    fun writesIntoPrivateLogFile() {
        val dir = temp.newFolder("logs")
        val store = LogStore(dir)

        store.write("I", "app", "started")
        store.write("W", "heartbeat", "failed")

        val file = File(dir, LogStore.FILE_NAME)
        assertTrue(file.exists())
        val text = file.readText(Charsets.UTF_8)
        assertTrue(text.contains(" I/app started"))
        assertTrue(text.contains(" W/heartbeat failed"))
    }

    @Test
    fun rotatesAfterOneMegabyteAndKeepsSingleBackup() {
        val dir = temp.newFolder("logs")
        val store = LogStore(dir)
        val payload = "x".repeat(2 * 1024)

        // 约 1.2MB，确保跨过 1MB 阈值。
        repeat(600) { store.write("I", "flood", payload) }

        val current = File(dir, LogStore.FILE_NAME)
        val rotated = File(dir, LogStore.ROTATED_FILE_NAME)
        assertTrue(rotated.exists())
        assertTrue("current=${current.length()}", current.length() <= LogStore.MAX_BYTES)
        assertTrue("rotated=${rotated.length()}", rotated.length() <= LogStore.MAX_BYTES)
        assertTrue(dir.listFiles().orEmpty().none { it.name == "guard.log.2" })
    }

    @Test
    fun concurrentWritesKeepEveryLine() {
        val dir = temp.newFolder("logs")
        val store = LogStore(dir)
        val threads = (0 until 4).map { index ->
            Thread {
                repeat(50) { line ->
                    store.write("I", "thread$index", "line$line")
                }
            }
        }

        threads.forEach { it.start() }
        threads.forEach { it.join() }

        val lines = File(dir, LogStore.FILE_NAME)
            .readLines(Charsets.UTF_8)
            .filter { it.isNotBlank() }
        assertEquals(200, lines.size)
    }

    @Test
    fun writeFailureDoesNotThrow() {
        val notADirectory = temp.newFile("logs")
        val store = LogStore(notADirectory)

        store.write("E", "test", "should not throw")

        assertTrue(notADirectory.isFile)
    }
}
