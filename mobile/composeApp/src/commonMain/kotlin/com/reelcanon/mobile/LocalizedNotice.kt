package com.reelcanon.mobile

data class LocalizedNotice(val key: Message, val arguments: List<Pair<String, Any?>> = emptyList())
fun notice(key: Message, vararg arguments: Pair<String, Any?>) = LocalizedNotice(key, arguments.toList())
fun Strings.render(value: LocalizedNotice) = invoke(value.key, *value.arguments.toTypedArray())
