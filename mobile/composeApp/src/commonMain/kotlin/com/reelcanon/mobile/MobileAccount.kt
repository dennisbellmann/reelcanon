@file:OptIn(kotlin.uuid.ExperimentalUuidApi::class, kotlinx.coroutines.ExperimentalCoroutinesApi::class)
package com.reelcanon.mobile

import androidx.room.immediateTransaction
import androidx.room.useWriterConnection
import io.ktor.client.plugins.expectSuccess
import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.request.*
import io.ktor.client.statement.HttpResponse
import io.ktor.http.*
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.flow.*
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.Serializable
import kotlin.time.Clock
import kotlin.uuid.Uuid

@Serializable data class AccountProfile(val id: String, val email: String)
@Serializable data class AppSession(val accessToken: String, val refreshToken: String, val expiresIn: Int, val user: AccountProfile, val accessExpiresAt: Long = 0)
@Serializable data class SessionEnvelope(val session: AppSession? = null, val pendingLogout: List<String> = emptyList(), val refreshUncertain: Boolean = false)
interface SecureSessionStore {
    suspend fun read(): SessionEnvelope
    suspend fun write(value: SessionEnvelope)
}
interface GoogleAuthenticator {
    suspend fun idToken(serverClientId: String, nonce: String): String
    suspend fun clear()
}
class SignInCancelled : Exception()
@Serializable private data class Challenge(val challengeId: String, val nonce: String, val serverClientId: String)
@Serializable data class ServerProgress(val imdbId: String, val watched: Boolean, val watchlist: Boolean, val watchedAt: String?, val updatedAt: String, val revision: Long, val watchedRevision: Long = revision, val watchlistRevision: Long = revision) {
    fun entity(owner: String) = ProgressEntity(imdbId, watched, watchlist, watchedAt, updatedAt, owner, revision, watched, watchlist)
    fun value(field: String) = if (field == "watched") watched else watchlist
}
@Serializable private data class ConflictResponse(val current: ServerProgress)
data class OwnedProgress(val owner: String, val rows: Map<String, MovieProgress>)
data class AccountState(val user: AccountProfile? = null, val busy: Boolean = false, val notice: Message? = null, val expired: Boolean = false)

/** Shared auth, ownership and sync. Only provider UI and secret storage are platform-specific. */
class MobileAccount(
    private val database: CatalogDatabase,
    private val client: HttpClient,
    apiBase: String,
    private val store: SecureSessionStore? = null,
    private val google: GoogleAuthenticator? = null,
) {
    private val base = apiBase.trimEnd('/')
    private val dao = database.catalogDao()
    private val lock = Mutex()
    private var envelope = SessionEnvelope()
    private var started = false
    private val owner = MutableStateFlow("guest")
    private val mutableState = MutableStateFlow(AccountState())
    val state = mutableState.asStateFlow()
    val supported get() = store != null && google != null
    val ownedProgress = owner.flatMapLatest { account -> dao.observeProgress(account).map { rows -> OwnedProgress(account, rows.associate { it.imdbId to it.model() }) } }
    val progress = ownedProgress.map { it.rows }
    val conflicts = owner.flatMapLatest { account -> dao.observeConflicts(account) }

    private suspend fun transaction(block: suspend () -> Unit) = database.useWriterConnection { connection -> connection.immediateTransaction { block() } }
    private suspend fun persist(value: SessionEnvelope) { store?.write(value); envelope = value }
    private fun activate(session: AppSession?) {
        owner.value = session?.user?.id ?: "guest"
        mutableState.value = AccountState(user = session?.user)
    }
    suspend fun start() = lock.withLock {
        if (!started) {
            envelope = store?.read() ?: SessionEnvelope()
            activate(envelope.session)
            if (envelope.refreshUncertain) mutableState.value = state.value.copy(expired = true, notice = Message.mobile_sessionExpired)
            started = true
        }
    }
    suspend fun signIn() {
        start()
        lock.withLock {
            if (!supported) return
            mutableState.value = state.value.copy(busy = true, notice = null)
            try {
                flushLogout()
                val challenge: Challenge = client.post("$base/auth/mobile/google/challenge") { contentType(ContentType.Application.Json); setBody(emptyMap<String, String>()); expectSuccess = false }.checked().body()
                val token = google!!.idToken(challenge.serverClientId, challenge.nonce)
                val session: AppSession = client.post("$base/auth/mobile/google") { contentType(ContentType.Application.Json); setBody(mapOf("challengeId" to challenge.challengeId, "idToken" to token)); expectSuccess = false }.checked().body()
                // Retain/revoke the previous family even when reauthentication switches accounts.
                val pending = envelope.pendingLogout + listOfNotNull(envelope.session?.refreshToken)
                persist(SessionEnvelope(session = session.timed(), pendingLogout = pending))
                activate(envelope.session)
                try { flushLogout(); syncLocked() }
                catch (error: CancellationException) { throw error }
                catch (_: Exception) { mutableState.value = state.value.copy(notice = Message.mobile_syncOffline) }
            } catch (_: SignInCancelled) { mutableState.value = state.value.copy(notice = Message.mobile_signInCancelled) }
            catch (error: CancellationException) { throw error }
            catch (_: Exception) { mutableState.value = state.value.copy(notice = Message.account_googleError) }
            finally { mutableState.value = state.value.copy(busy = false) }
        }
    }
    private fun AppSession.timed() = copy(accessExpiresAt = Clock.System.now().epochSeconds + expiresIn)
    private fun HttpResponse.checked(): HttpResponse { if (status.value !in 200..299) throw AccountHttpError(status.value); return this }
    private suspend fun flushLogout() {
        for (token in envelope.pendingLogout.toList()) {
            client.post("$base/auth/mobile/logout") { contentType(ContentType.Application.Json); setBody(mapOf("refreshToken" to token)); expectSuccess = false }.checked()
            persist(envelope.copy(pendingLogout = envelope.pendingLogout - token))
        }
    }
    suspend fun logout() {
        start()
        lock.withLock {
            val pending = envelope.pendingLogout + listOfNotNull(envelope.session?.refreshToken)
            // Clear visible account immediately, but retain a protected revocation queue offline.
            persist(SessionEnvelope(pendingLogout = pending))
            activate(null)
            try { google?.clear(); flushLogout() }
            catch (error: CancellationException) { throw error }
            catch (_: Exception) { mutableState.value = state.value.copy(notice = Message.mobile_logoutPending) }
        }
    }
    private suspend fun access(): AppSession {
        val session = envelope.session ?: throw AccountHttpError(401)
        if (envelope.refreshUncertain) throw AccountHttpError(401)
        if (Clock.System.now().epochSeconds < session.accessExpiresAt - 30) return session
        // Persist before sending: a crash or lost response must never replay a spent refresh.
        persist(envelope.copy(refreshUncertain = true))
        val next: AppSession = client.post("$base/auth/mobile/refresh") { contentType(ContentType.Application.Json); setBody(mapOf("refreshToken" to session.refreshToken)); expectSuccess = false }.checked().body()
        require(next.user.id == session.user.id)
        persist(envelope.copy(session = next.timed(), refreshUncertain = false))
        return envelope.session!!
    }
    private suspend fun authenticated(block: suspend (AppSession) -> HttpResponse): HttpResponse {
        val response = block(access())
        if (response.status.value == 401) throw AccountHttpError(401)
        return response
    }
    suspend fun sync() {
        start()
        lock.withLock {
            mutableState.value = state.value.copy(busy = true)
            try { flushLogout(); if (envelope.session != null) syncLocked() }
            catch (error: CancellationException) { throw error }
            catch (error: AccountHttpError) { mutableState.value = state.value.copy(expired = error.status == 401 || envelope.refreshUncertain, notice = if (error.status == 401 || envelope.refreshUncertain) Message.mobile_sessionExpired else Message.mobile_syncOffline) }
            catch (_: Exception) { mutableState.value = state.value.copy(expired = envelope.refreshUncertain, notice = if (envelope.refreshUncertain) Message.mobile_sessionExpired else Message.mobile_syncOffline) }
            finally { mutableState.value = state.value.copy(busy = false) }
        }
    }
    private suspend fun syncLocked() {
        val account = owner.value
        val remote: List<ServerProgress> = authenticated { session -> client.get("$base/mobile/me/progress") { bearerAuth(session.accessToken); expectSuccess = false } }.checked().body()
        transaction {
            val pending = dao.outbox(account)
            val pendingRows = dao.allProgress(account).filter { row -> pending.any { it.imdbId == row.imdbId } }
            dao.clearProgress(account)
            pendingRows.forEach { dao.saveProgress(it) }
            remote.forEach { mergeRemote(it, account) }
        }
        var conflicts = false
        while (true) {
            val operation = dao.outbox(account).firstOrNull { !it.conflict && dao.outbox(account).none { earlier -> earlier.imdbId == it.imdbId && earlier.sequence < it.sequence && earlier.conflict } } ?: break
            val response = authenticated { session -> client.put("$base/mobile/me/progress/${operation.imdbId}/${operation.field}") {
                bearerAuth(session.accessToken); header("If-Match", "\"${operation.revision}\""); header("Idempotency-Key", operation.operationId)
                contentType(ContentType.Application.Json); setBody(mapOf("value" to operation.value)); expectSuccess = false
            } }
            when (response.status.value) {
                200 -> { val row: ServerProgress = response.body(); transaction {
                    dao.deleteOperation(operation.sequence)
                    // Later offline edits to this field know that this device's write succeeded.
                    // Keep other fields' baselines so external changes still surface as conflicts.
                    dao.outbox(account).filter { it.imdbId == operation.imdbId && it.field == operation.field && it.sequence > operation.sequence && it.revision <= operation.revision }.forEach {
                        dao.saveOperation(it.copy(operationId = Uuid.random().toString(), revision = row.revision))
                    }
                    mergeRemote(row, account)
                } }
                412 -> {
                    val row = response.body<ConflictResponse>().current
                    transaction {
                        when {
                            row.value(operation.field) == operation.value -> dao.deleteOperation(operation.sequence)
                            (if (operation.field == "watched") row.watchedRevision else row.watchlistRevision) <= operation.revision -> dao.saveOperation(operation.copy(operationId = Uuid.random().toString(), revision = row.revision))
                            else -> { dao.saveOperation(operation.copy(conflict = true)); conflicts = true }
                        }
                        mergeRemote(row, account)
                    }
                }
                else -> { response.checked() }
            }
        }
        mutableState.value = state.value.copy(expired = false, notice = if (conflicts || dao.outbox(account).any { it.conflict }) Message.mobile_syncConflict else Message.mobile_synced)
    }
    private suspend fun mergeRemote(remote: ServerProgress, account: String) {
        val existing = dao.progress(account, remote.imdbId)
        // A replay receipt may predate the full pull. Acknowledge without rolling back newer data.
        var row = if (existing != null && existing.revision > remote.revision) existing.copy(watched = existing.serverWatched, watchlist = existing.serverWatchlist) else remote.entity(account)
        dao.outbox(account).filter { it.imdbId == row.imdbId }.forEach { op ->
            row = if (op.field == "watched") row.copy(watched = op.value, watchedAt = if (op.value) row.watchedAt ?: Clock.System.now().toString() else null) else row.copy(watchlist = op.value)
        }
        dao.saveProgress(row)
    }
    suspend fun change(imdbId: String, field: String, value: Boolean) {
        start()
        require(Regex("tt[0-9]{7,10}").matches(imdbId) && field in listOf("watched", "watchlist"))
        lock.withLock {
            val account = owner.value
            transaction {
                val now = Clock.System.now().toString()
                val old = dao.progress(account, imdbId) ?: ProgressEntity(imdbId, updatedAt = now, owner = account)
                val before = if (field == "watched") old.watched else old.watchlist
                if (before != value) {
                    val updated = if (field == "watched") old.copy(watched = value, watchedAt = if (value) old.watchedAt ?: now else null, updatedAt = now) else old.copy(watchlist = value, updatedAt = now)
                    dao.saveProgress(updated)
                    if (account != "guest") dao.saveOperation(OutboxEntity(operationId = Uuid.random().toString(), owner = account, imdbId = imdbId, field = field, value = value, baseValue = before, revision = old.revision))
                }
            }
        }
        if (owner.value != "guest") sync()
    }
    suspend fun resolve(operation: OutboxEntity, keepLocal: Boolean) {
        lock.withLock {
            require(operation.owner == owner.value)
            transaction {
                val current = dao.outbox(owner.value).firstOrNull { it.sequence == operation.sequence && it.conflict } ?: return@transaction
                val row = requireNotNull(dao.progress(owner.value, current.imdbId))
                if (keepLocal) dao.saveOperation(current.copy(operationId = Uuid.random().toString(), revision = row.revision, baseValue = if (current.field == "watched") row.serverWatched else row.serverWatchlist, conflict = false))
                else { dao.outbox(owner.value).filter { it.imdbId == current.imdbId && it.field == current.field && it.sequence >= current.sequence }.forEach { dao.deleteOperation(it.sequence) } }
                mergeRemote(ServerProgress(row.imdbId, row.serverWatched, row.serverWatchlist, if (row.serverWatched) row.watchedAt ?: Clock.System.now().toString() else null, row.updatedAt, row.revision), owner.value)
            }
        }
        sync()
    }
    suspend fun exportBackup(): String {
        start()
        return lock.withLock { catalogJson.encodeToString(ProgressBackup.serializer(), ProgressBackup(exportedAt = Clock.System.now().toString(), progress = dao.allProgress(owner.value).map { it.model() })) }
    }
    suspend fun importBackup(text: String): Int {
        start()
        return lock.withLock {
            require(owner.value == "guest") // Account import always goes through server missing-row semantics.
            val rows = parseBackup(text)
            transaction { rows.forEach { row ->
                val old = dao.progress("guest", row.imdbId)
                if (old == null || kotlin.time.Instant.parse(row.updatedAt) > kotlin.time.Instant.parse(old.updatedAt)) dao.saveProgress(ProgressEntity(row.imdbId, row.watched, row.watchlist, row.watchedAt, row.updatedAt))
            } }
            rows.size
        }
    }
    suspend fun importGuest() {
        start()
        lock.withLock {
            require(owner.value != "guest")
            val backup = ProgressBackup(exportedAt = Clock.System.now().toString(), progress = dao.allProgress("guest").map { it.model() })
            authenticated { session -> client.post("$base/mobile/me/progress/import") { bearerAuth(session.accessToken); contentType(ContentType.Application.Json); setBody(backup); expectSuccess = false } }.checked()
        }
        sync()
    }
}
private class AccountHttpError(val status: Int) : Exception()
