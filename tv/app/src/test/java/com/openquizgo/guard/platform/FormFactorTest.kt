package com.openquizgo.guard.platform

import org.junit.Assert.assertEquals
import org.junit.Test

class FormFactorTest {

    @Test
    fun `leanback 设备判定为电视`() {
        assertEquals(FormFactor.Television, FormFactors.detect(uiModeTv = false, leanback = true))
    }

    @Test
    fun `ui mode 为电视也判定为电视`() {
        assertEquals(FormFactor.Television, FormFactors.detect(uiModeTv = true, leanback = false))
    }

    @Test
    fun `普通平板与手机走触摸界面`() {
        assertEquals(FormFactor.Touch, FormFactors.detect(uiModeTv = false, leanback = false))
    }
}
