package com.reelcanon.app

import android.database.sqlite.SQLiteDatabase
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import androidx.test.platform.app.InstrumentationRegistry
import com.reelcanon.mobile.*
import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.*
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.http.*
import io.ktor.serialization.kotlinx.json.json
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.*
import org.junit.Assert.*
import org.junit.Test
import java.io.File
import java.io.IOException
import java.util.UUID

private class MemorySecrets : SecureSessionStore {
    var value = SessionEnvelope()
    override suspend fun read() = value
    override suspend fun write(value: SessionEnvelope) { this.value = value }
}
private class SyncFixture {
    val context = InstrumentationRegistry.getInstrumentation().targetContext
    val directory = File(context.cacheDir, "account-test-${UUID.randomUUID()}").apply { mkdirs() }
    val secrets = MemorySecrets()
    var account = "A"
    var online = true
    var loseWriteResponse = false
    var loseRefreshResponse = false
    var refreshRequests = 0
    var revisionRequests = mutableListOf<Pair<String, Long>>()
    val remote = mutableMapOf<String, ServerProgress>()
    val receipts = mutableMapOf<String, ServerProgress>()
    val provider = object : GoogleAuthenticator {
        override suspend fun idToken(serverClientId: String, nonce: String) = "test-id-token"
        override suspend fun clear() {}
    }
    private fun session() = AppSession("access-$account", "refresh-$account", 900, AccountProfile(account, "$account@example.test"))
    fun open(): CatalogRepository {
        val database = Room.databaseBuilder<CatalogDatabase>(context, File(directory, "catalog.db").absolutePath)
            .addMigrations(PROGRESS_MIGRATION).setDriver(BundledSQLiteDriver()).setQueryCoroutineContext(Dispatchers.IO).build()
        val client = HttpClient(MockEngine { request ->
            if (!online) throw IOException("Offline")
            val path = request.url.encodedPath
            var status = HttpStatusCode.OK
            val body = when {
                path.endsWith("/challenge") -> """{"challengeId":"challenge","nonce":"nonce","serverClientId":"public-web-client"}"""
                path.endsWith("/google") -> catalogJson.encodeToString(session())
                path.endsWith("/logout") -> "{}"
                path.endsWith("/refresh") -> {
                    refreshRequests++
                    if (loseRefreshResponse) throw IOException("Refresh response lost")
                    catalogJson.encodeToString(session())
                }
                request.method == HttpMethod.Get -> catalogJson.encodeToString(listOfNotNull(remote[request.headers[HttpHeaders.Authorization]?.removePrefix("Bearer access-")]))
                else -> {
                    val owner = request.headers[HttpHeaders.Authorization]!!.removePrefix("Bearer access-")
                    val operation = request.headers["Idempotency-Key"]!!
                    val expected = request.headers["If-Match"]!!.trim('"').toLong()
                    revisionRequests.add(operation to expected)
                    val field = path.substringAfterLast('/')
                    val value = catalogJson.parseToJsonElement((request.body as io.ktor.http.content.TextContent).text).jsonObject["value"]!!.jsonPrimitive.boolean
                    val current = remote[owner] ?: ServerProgress("tt0068646", false, false, null, "2026-10-07T00:00:00Z", 0, 0, 0)
                    if (receipts.containsKey(operation)) catalogJson.encodeToString(receipts.getValue(operation))
                    else if (current.revision != expected) {
                        status = HttpStatusCode.PreconditionFailed
                        """{"current":${catalogJson.encodeToString(current)}}"""
                    } else {
                        val before = current.value(field)
                        val next = if (value == before) current else current.copy(
                            watched = if (field == "watched") value else current.watched,
                            watchlist = if (field == "watchlist") value else current.watchlist,
                            watchedAt = if (field == "watched") (if (value) "2026-10-07T00:00:00Z" else null) else current.watchedAt,
                            revision = current.revision + 1,
                            watchedRevision = if (field == "watched") current.revision + 1 else current.watchedRevision,
                            watchlistRevision = if (field == "watchlist") current.revision + 1 else current.watchlistRevision,
                        )
                        remote[owner] = next; receipts[operation] = next
                        if (loseWriteResponse) { loseWriteResponse = false; throw IOException("Response lost after commit") }
                        catalogJson.encodeToString(next)
                    }
                }
            }
            respond(body, status, headersOf(HttpHeaders.ContentType, "application/json"))
        }) { expectSuccess = true; install(ContentNegotiation) { json(catalogJson) } }
        return CatalogRepository(database, client, "https://test/api/v1", File(directory, "posters").absolutePath, secrets, provider)
    }
    fun cleanup() { directory.deleteRecursively() }
}

class AccountSyncDeviceTest {
    @Test fun isolatesGuestAndAccountsAndRestoresServerProgress() = runBlocking {
        val fixture = SyncFixture(); var repo = fixture.open()
        try {
            repo.setWatchlist("tt0111161", true) // Guest survives account switches.
            fixture.remote["A"] = ServerProgress("tt0068646", true, false, "2026-10-07T00:00:00Z", "2026-10-07T00:00:00Z", 1, 1, 0)
            repo.account.signIn()
            assertEquals("A", repo.account.state.value.user?.id)
            assertTrue(repo.progress.first().getValue("tt0068646").watched)
            assertFalse(repo.progress.first().containsKey("tt0111161"))
            repo.setWatchlist("tt0068646", true)
            assertTrue(fixture.remote.getValue("A").watchlist)
            repo.account.logout()
            assertTrue(repo.progress.first().getValue("tt0111161").watchlist)
            assertFalse(repo.progress.first().containsKey("tt0068646"))
            fixture.account = "B"; repo.account.signIn()
            assertTrue(repo.progress.first().isEmpty())
            repo.account.logout(); fixture.account = "A"; repo.account.signIn()
            repo.close(); repo = fixture.open(); repo.account.start()
            assertTrue(repo.progress.first().getValue("tt0068646").watchlist)
        } finally { repo.close(); fixture.cleanup() }
    }
    @Test fun durableOutboxRetriesSameOperationAfterLostResponseAndProcessRestart() = runBlocking {
        val fixture = SyncFixture(); var repo = fixture.open()
        try {
            repo.account.signIn(); fixture.online = false
            repo.setWatched("tt0068646", true)
            assertTrue(repo.progress.first().getValue("tt0068646").watched)
            repo.close(); repo = fixture.open(); repo.account.start()
            fixture.online = true; fixture.loseWriteResponse = true; repo.account.sync()
            assertEquals(1L, fixture.remote.getValue("A").revision)
            val original = fixture.revisionRequests.last()
            // A web change after the lost response must survive replaying an older receipt.
            fixture.remote["A"] = fixture.remote.getValue("A").copy(watchlist = true, revision = 2, watchlistRevision = 2)
            repo.close(); repo = fixture.open(); repo.account.sync()
            assertEquals(original, fixture.revisionRequests.last())
            assertEquals(2L, fixture.remote.getValue("A").revision)
            assertTrue(repo.progress.first().getValue("tt0068646").watchlist)
            assertTrue(repo.progress.first().getValue("tt0068646").watched)
        } finally { repo.close(); fixture.cleanup() }
    }
    @Test fun rebasesIndependentFieldsAndOffersDecisionForSameFieldConflict() = runBlocking {
        val fixture = SyncFixture(); val repo = fixture.open()
        try {
            repo.account.signIn(); fixture.online = false; repo.setWatched("tt0068646", true)
            // Web changes the independent watchlist field.
            fixture.remote["A"] = ServerProgress("tt0068646", false, true, null, "2026-10-07T00:00:00Z", 1, 0, 1)
            fixture.online = true; repo.account.sync()
            assertEquals(2L, fixture.remote.getValue("A").revision)
            assertTrue(fixture.remote.getValue("A").watched && fixture.remote.getValue("A").watchlist)
            assertNotEquals(fixture.revisionRequests[0].first, fixture.revisionRequests[1].first)
            fixture.online = false; repo.setWatched("tt0068646", false)
            // Web changed watched twice and ended on true: only field revisions reveal it.
            fixture.remote["A"] = fixture.remote.getValue("A").copy(revision = 4, watchedRevision = 4)
            fixture.online = true; repo.account.sync()
            val conflict = repo.account.conflicts.first { it.isNotEmpty() }.single()
            assertTrue(fixture.remote.getValue("A").watched)
            repo.account.resolve(conflict, keepLocal = false)
            assertTrue(repo.progress.first().getValue("tt0068646").watched)
            assertTrue(repo.account.conflicts.first().isEmpty())
            // Multiple offline edits to the same field preserve device order without false conflicts.
            fixture.online = false; repo.setWatched("tt0068646", false); repo.setWatched("tt0068646", true)
            fixture.online = true; repo.account.sync()
            assertTrue(repo.account.conflicts.first().isEmpty())
            assertTrue(fixture.remote.getValue("A").watched)
        } finally { repo.close(); fixture.cleanup() }
    }
    @Test fun ambiguousRefreshIsNotReplayedAndOfflineLogoutHidesAccount() = runBlocking {
        val fixture = SyncFixture(); var repo = fixture.open()
        try {
            repo.account.signIn()
            fixture.secrets.value = fixture.secrets.value.copy(session = fixture.secrets.value.session!!.copy(accessExpiresAt = 0))
            repo.close(); repo = fixture.open(); fixture.loseRefreshResponse = true
            repo.account.sync(); assertTrue(repo.account.state.value.expired)
            repo.close(); repo = fixture.open(); repo.account.sync()
            assertEquals(1, fixture.refreshRequests)
            fixture.online = false; repo.account.logout()
            assertNull(repo.account.state.value.user)
            assertTrue(fixture.secrets.value.pendingLogout.isNotEmpty())
            assertTrue(repo.progress.first().isEmpty())
            fixture.online = true; repo.account.sync()
            assertTrue(fixture.secrets.value.pendingLogout.isEmpty())
        } finally { repo.close(); fixture.cleanup() }
    }
    @Test fun migratesVersionOneProgressToGuestAndEncryptsSessions() = runBlocking {
        val fixture = SyncFixture()
        val path = File(fixture.directory, "catalog.db")
        SQLiteDatabase.openOrCreateDatabase(path, null).use { db ->
            db.execSQL("CREATE TABLE cached_canons(slug TEXT NOT NULL PRIMARY KEY,payload TEXT NOT NULL,savedAt TEXT NOT NULL)")
            db.execSQL("CREATE TABLE progress(imdbId TEXT NOT NULL PRIMARY KEY,watched INTEGER NOT NULL,watchlist INTEGER NOT NULL,watchedAt TEXT,updatedAt TEXT NOT NULL)")
            db.execSQL("INSERT INTO progress VALUES('tt0068646',0,1,NULL,'2026-10-07T00:00:00Z')")
            db.version = 1
        }
        val repo = fixture.open()
        val base = "https://secret-test-${UUID.randomUUID()}/api/v1"
        val store = AndroidSessionStore(fixture.context, base)
        try {
            assertTrue(repo.progress.first().getValue("tt0068646").watchlist)
            repo.account.signIn(); assertTrue(repo.progress.first().isEmpty())
            val token = UUID.randomUUID().toString()
            val envelope = SessionEnvelope(AppSession(token, "refresh-$token", 900, AccountProfile("A", "a@example.test")))
            store.write(envelope)
            assertEquals(envelope, AndroidSessionStore(fixture.context, base).read())
            fixture.context.noBackupFilesDir.listFiles()!!.filter { it.name.startsWith("account-") }.forEach { assertFalse(it.readBytes().toString(Charsets.UTF_8).contains(token)) }
        } finally { store.write(SessionEnvelope()); repo.close(); fixture.cleanup() }
    }
}
