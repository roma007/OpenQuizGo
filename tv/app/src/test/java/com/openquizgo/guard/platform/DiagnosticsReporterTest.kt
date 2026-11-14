package com.openquizgo.guard.platform

import java.time.Instant
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class DiagnosticsReporterTest {
    @Test
    fun clockSkewComputesDelta() {
        val now = Instant.parse("2026-11-05T10:06:30Z").toEpochMilli()
        assertEquals(
            390_000L,
            DiagnosticsReporter.clockSkewMillis("2026-11-05T10:00:00Z", now),
        )
    }

    @Test
    fun clockSkewIgnoresMissingOrInvalidTime() {
        assertNull(DiagnosticsReporter.clockSkewMillis(null, 0))
        assertNull(DiagnosticsReporter.clockSkewMillis("not-a-time", 0))
    }
}
