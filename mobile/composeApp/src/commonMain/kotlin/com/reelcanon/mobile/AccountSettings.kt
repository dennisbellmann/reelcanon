package com.reelcanon.mobile

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

@Composable
fun AccountSettings(account: MobileAccount, movies: List<Movie>) {
    if (!account.supported) return
    val t = LocalStrings.current
    val state by account.state.collectAsState()
    val conflicts by account.conflicts.collectAsState(emptyList())
    val scope = rememberCoroutineScope()
    var confirmImport by remember { mutableStateOf(false) }
    var failed by remember { mutableStateOf(false) }
    fun action(block: suspend () -> Unit) { scope.launch {
        failed = false
        try { block() }
        catch (error: CancellationException) { throw error }
        catch (_: Exception) { failed = true }
    } }
    Text(t(Message.account_title), style = MaterialTheme.typography.titleLarge)
    state.user?.let { Text(it.email, modifier = Modifier.testTag("account-email")) }
    if (state.busy) LinearProgressIndicator(Modifier.fillMaxWidth())
    if (state.user == null || state.expired) {
        OutlinedButton(onClick = { action { account.signIn() } }, enabled = !state.busy, modifier = Modifier.fillMaxWidth().testTag("google-sign-in")) { Text(t(Message.account_google)) }
    }
    if (state.user != null) {
        OutlinedButton(onClick = { action { account.sync() } }, enabled = !state.busy, modifier = Modifier.fillMaxWidth().testTag("account-sync")) { Text(t(Message.mobile_retry)) }
        TextButton(onClick = { action { account.logout() } }, enabled = !state.busy, modifier = Modifier.testTag("account-logout")) { Text(t(Message.account_logout)) }
        OutlinedButton(onClick = { confirmImport = true }, enabled = !state.busy && !state.expired, modifier = Modifier.fillMaxWidth()) { Text(t(Message.account_importGuest)) }
    }
    state.notice?.let { Text(t(it)) }
    if (failed) Text(t(Message.mobile_syncOffline))
    conflicts.filter { it.owner == state.user?.id }.forEach { operation ->
        val title = movies.firstOrNull { it.imdbId == operation.imdbId }?.title ?: operation.imdbId
        Text(t(Message.mobile_conflictEntry, "title" to title, "field" to t(if (operation.field == "watched") Message.filter_watched else Message.filter_watchlist), "value" to if (operation.value) "✓" else "—"))
        OutlinedButton(onClick = { action { account.resolve(operation, true) } }, enabled = !state.busy) { Text(t(Message.mobile_keepLocal)) }
        TextButton(onClick = { action { account.resolve(operation, false) } }, enabled = !state.busy) { Text(t(Message.mobile_keepServer)) }
    }
    HorizontalDivider()
    if (confirmImport) AlertDialog(
        onDismissRequest = { confirmImport = false }, text = { Text(t(Message.mobile_importGuestConfirm)) },
        confirmButton = { TextButton(onClick = { confirmImport = false; action { account.importGuest() } }) { Text(t(Message.account_importAction)) } },
        dismissButton = { TextButton(onClick = { confirmImport = false }) { Text(t(Message.common_cancel)) } },
    )
}
