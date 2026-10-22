package com.reelcanon.mobile

import kotlin.test.Test
import kotlin.test.assertEquals

class LocalizedStringsTest {
    @Test
    fun namedParametersCanBeReorderedAndAreNotInterpolatedAgain() {
        val strings = Strings(mapOf(Message.collection_progressSummary to "{{total}} total, {{watched}} watched"))
        assertEquals("250 total, {{total}} watched", strings(Message.collection_progressSummary, "watched" to "{{total}}", "total" to 250))
    }
}
