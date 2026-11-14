package com.openquizgo.guard.data.remote

import org.junit.Assert.assertEquals
import org.junit.Test

class ApiFactoryTest {

    @Test
    fun addsTrailingSlash() {
        assertEquals("http://localhost:8080/", ApiFactory.normalizeBaseUrl("http://localhost:8080"))
    }

    @Test
    fun keepsExistingSlash() {
        assertEquals("http://localhost:8080/", ApiFactory.normalizeBaseUrl("http://localhost:8080/"))
    }

    @Test
    fun trimsWhitespace() {
        assertEquals("http://localhost:8080/", ApiFactory.normalizeBaseUrl("  http://localhost:8080  "))
    }
}
