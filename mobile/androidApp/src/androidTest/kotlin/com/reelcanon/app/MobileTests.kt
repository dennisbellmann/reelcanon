package com.reelcanon.app

import android.content.Context
import android.app.LocaleManager
import android.os.Build
import android.os.LocaleList
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import androidx.test.filters.SdkSuppress
import androidx.test.platform.app.InstrumentationRegistry
import com.reelcanon.mobile.*
import io.ktor.client.HttpClient
import io.ktor.client.engine.mock.MockEngine
import io.ktor.client.engine.mock.respond
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.http.*
import io.ktor.serialization.kotlinx.json.json
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.encodeToString
import org.junit.*
import org.junit.rules.RuleChain
import org.junit.rules.TestRule
import org.junit.runner.Description
import org.junit.runners.model.Statement
import java.io.File
import java.io.IOException
import java.util.UUID

private class TestCatalog {
    val context: Context = InstrumentationRegistry.getInstrumentation().targetContext
    val directory = File(context.cacheDir, "mobile-test-${UUID.randomUUID()}").apply { mkdirs() }
    var online = true
    var incomplete = false
    val films = listOf(
        Movie("tt0068646", "Der Pate", "The Godfather", 1972, overview = "Eine Familiengeschichte.",
            posterUrl = "https://images.test/pate.png", imdbRating = 9.2, directors = listOf("Francis Ford Coppola"), position = 1),
        Movie("tt0111161", "Die Verurteilten", releaseYear = 1994, position = 2),
    )
    val canon = Canon("test-canon", "Test-Kanon", filmCount = 2, currentEdition = "fixture-v1")
    fun open(): CatalogRepository {
        val database = Room.databaseBuilder<CatalogDatabase>(context, File(directory, "catalog.db").absolutePath)
            .setDriver(BundledSQLiteDriver()).setQueryCoroutineContext(Dispatchers.IO).build()
        val client = HttpClient(MockEngine { request ->
            if (!online) throw IOException("Offline test")
            if (request.url.host == "images.test") {
                val png = android.util.Base64.decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aD1sAAAAASUVORK5CYII=", 0)
                respond(png, headers = headersOf(HttpHeaders.ContentType, "image/png"))
            } else {
                val body = if (request.url.encodedPath == "/api/v1/canons") catalogJson.encodeToString(listOf(canon))
                    else catalogJson.encodeToString(CanonCatalog(canon, if (incomplete) films.take(1) else films))
                respond(body, headers = headersOf(HttpHeaders.ContentType, "application/json"))
            }
        }) { expectSuccess = true; install(ContentNegotiation) { json(catalogJson) } }
        return CatalogRepository(database, client, "https://catalog.test/api/v1", File(directory, "posters").absolutePath)
    }
    fun cleanup() { directory.deleteRecursively() }
}

class RepositoryDeviceTest {
    @Test fun persistsOfflineCatalogPostersAndIndependentProgressFields() = runBlocking {
        val fixture = TestCatalog()
        var repository = fixture.open()
        try {
            repository.refresh()
            coroutineScope {
                listOf(async { repository.setWatched("tt0068646", true) }, async { repository.setWatchlist("tt0068646", true) }).awaitAll()
            }
            repository.downloadPosters(repository.catalogs.first().single()) { _, _ -> }
            Assert.assertTrue(File(repository.posterModel(fixture.films.first())!!).exists())
            repository.close()
            fixture.online = false
            repository = fixture.open()
            Assert.assertEquals(2, repository.catalogs.first().single().movies.size)
            val progress = repository.progress.first().getValue("tt0068646")
            Assert.assertTrue(progress.watched && progress.watchlist)
            Assert.assertEquals(1, repository.downloadedPosters(repository.catalogs.first().single()))
            repository.setWatched("tt0068646", false)
            Assert.assertTrue(repository.progress.first().getValue("tt0068646").watchlist)
            Assert.assertNull(repository.progress.first().getValue("tt0068646").watchedAt)
            try { repository.refresh(); Assert.fail("Offline refresh must fail") } catch (_: IOException) { }
            Assert.assertEquals(2, repository.catalogs.first().single().movies.size)
            repository.close()
            repository = fixture.open()
            Assert.assertFalse(repository.progress.first().getValue("tt0068646").watched)
        } finally { repository.close(); fixture.cleanup() }
    }

    @Test fun rejectsPartialEditionAndMergesBackupWithoutErasingOtherFilms() = runBlocking {
        val fixture = TestCatalog()
        val repository = fixture.open()
        try {
            repository.refresh()
            fixture.incomplete = true
            try { repository.refresh(); Assert.fail("Partial edition must fail") } catch (_: IllegalArgumentException) { }
            Assert.assertEquals(2, repository.catalogs.first().single().movies.size)
            repository.setWatchlist("tt0111161", true)
            val newer = MovieProgress("tt0068646", true, false, "2090-01-01T00:00:00Z", "2090-01-01T00:00:00Z")
            repository.importBackup(catalogJson.encodeToString(ProgressBackup(exportedAt = "2026-10-06T00:00:00Z", progress = listOf(newer))))
            Assert.assertTrue(repository.progress.first().getValue("tt0111161").watchlist)
            Assert.assertTrue(repository.progress.first().getValue("tt0068646").watched)
            val older = newer.copy(watched = false, watchedAt = null, updatedAt = "2020-01-01T00:00:00Z")
            repository.importBackup(catalogJson.encodeToString(ProgressBackup(exportedAt = "2026-10-06T00:00:00Z", progress = listOf(older))))
            Assert.assertTrue(repository.progress.first().getValue("tt0068646").watched)
            Assert.assertEquals(2, parseBackup(repository.exportBackup()).size)
        } finally { repository.close(); fixture.cleanup() }
    }
}

class MobileUiDeviceTest {
    private val compose = createComposeRule()
    @get:Rule val rules: TestRule = RuleChain.outerRule(AppLocaleRule()).around(compose)

    @SdkSuppress(minSdkVersion = 33)
    @Test fun localizedUiStartsInGerman() = assertLocalizedUi("Ein Leben voller Film.", "1 / 2 gesehen")

    @SdkSuppress(minSdkVersion = 33)
    @Test fun localizedUiStartsInEnglish() = assertLocalizedUi("A life full of film.", "1 / 2 watched")

    @SdkSuppress(minSdkVersion = 33)
    @Test fun localizedUiStartsInSpanish() = assertLocalizedUi("Una vida de cine.", "1 / 2 vistas")

    @SdkSuppress(minSdkVersion = 33)
    @Test fun localizedUiStartsInFrench() = assertLocalizedUi("Une vie de cinéma.", "1 / 2 vus")

    private fun assertLocalizedUi(tagline: String, progressSummary: String) {
        val fixture = TestCatalog()
        val repository = fixture.open()
        try {
            compose.setContent { ReelCanonApp(repository) }
            compose.waitUntil(15_000) { compose.onAllNodesWithTag("canon-test-canon").fetchSemanticsNodes().size == 1 }
            compose.waitUntil(5_000) { compose.onAllNodesWithText(tagline).fetchSemanticsNodes().size == 1 }
            compose.onNodeWithTag("canon-test-canon").performClick()
            runBlocking { repository.setWatched("tt0068646", true) }
            compose.waitUntil(5_000) { compose.onAllNodesWithText(progressSummary).fetchSemanticsNodes().isNotEmpty() }
            Assert.assertTrue(runBlocking { repository.progress.first().getValue("tt0068646").watched })
        } finally { repository.close(); fixture.cleanup() }
    }

    @Test fun collectionPosterBadgeUsesPersonalProgress() {
        val fixture = TestCatalog()
        val repository = fixture.open()
        try {
            Assert.assertEquals("https://catalog.test/artwork/collection-03-filmwelten.png", repository.collectionPosterModel(fixture.canon))
            compose.setContent { ReelCanonApp(repository) }
            compose.waitUntil(15_000) { compose.onAllNodesWithTag("poster-badge", useUnmergedTree = true).fetchSemanticsNodes().size == 1 }
            compose.onNodeWithTag("poster-badge", useUnmergedTree = true).assertTextEquals("✓  0/2")
            runBlocking { repository.setWatched("tt0068646", true) }
            compose.waitUntil(5_000) { compose.onAllNodesWithText("✓  1/2", useUnmergedTree = true).fetchSemanticsNodes().size == 1 }
            runBlocking { repository.setWatched("tt0068646", false) }
            compose.waitUntil(5_000) { compose.onAllNodesWithText("✓  0/2", useUnmergedTree = true).fetchSemanticsNodes().size == 1 }
        } finally { repository.close(); fixture.cleanup() }
    }

    @Test fun browseSearchMarkWatchlistAndNavigateDecadeAlbum() {
        val fixture = TestCatalog()
        val repository = fixture.open()
        try {
            compose.setContent { ReelCanonApp(repository) }
            compose.waitUntil(15_000) { compose.onAllNodesWithTag("canon-test-canon").fetchSemanticsNodes().size == 1 }
            compose.onNodeWithTag("canon-test-canon").performClick()
            compose.onNodeWithTag("search").performTextInput("godfather")
            compose.onNodeWithTag("film-count").assertTextEquals("Filme: 1")
            compose.onNodeWithTag("movie-tt0068646").performClick()
            compose.onNodeWithTag("detail-title").assertTextEquals("Der Pate")
            compose.onNodeWithTag("detail-watched").performScrollTo().performClick()
            compose.waitUntil(5_000) { compose.onAllNodesWithText("✓ Gesehen").fetchSemanticsNodes().isNotEmpty() }
            compose.onNodeWithTag("detail-watchlist").performScrollTo().performClick()
            compose.waitUntil(5_000) { compose.onAllNodesWithText("✓ Merkliste").fetchSemanticsNodes().isNotEmpty() }
            compose.onNodeWithTag("back").performClick()
            compose.onNodeWithTag("progress-count").assertTextEquals("1 / 2 gesehen")
            compose.onNodeWithTag("search").performTextClearance()
            compose.onNodeWithTag("album-toggle").performClick()
            compose.onNodeWithTag("decade-1970").performClick()
            compose.onNodeWithTag("film-count").assertTextEquals("Filme: 1")
            compose.onNodeWithTag("album-tt0068646").assertExists()
            compose.onNodeWithTag("tab-watchlist").performClick()
            compose.onNodeWithTag("movie-tt0068646").assertExists()
        } finally { repository.close(); fixture.cleanup() }
    }

    @Test fun hideWatchedToggleFiltersAndRemembersCollectionPreference() {
        val fixture = TestCatalog()
        val repository = fixture.open()
        val preferences = fixture.context.getSharedPreferences(
            "hide-watched-test-${UUID.randomUUID()}",
            Context.MODE_PRIVATE,
        )
        try {
            runBlocking {
                repository.refresh()
                repository.setWatched("tt0068646", true)
            }
            compose.setContent {
                ReelCanonApp(
                    repository,
                    hideWatchedPreference = { slug ->
                        preferences.getBoolean("hide-watched:$slug", false)
                    },
                    saveHideWatchedPreference = { slug, hidden ->
                        preferences.edit().putBoolean("hide-watched:$slug", hidden).apply()
                    },
                )
            }
            compose.waitUntil(15_000) {
                compose.onAllNodesWithTag("canon-test-canon").fetchSemanticsNodes().size == 1
            }
            compose.onNodeWithTag("canon-test-canon").performClick()
            val hideWatchedToggle = compose.onNodeWithTag("hide-watched-toggle")
            hideWatchedToggle.assertIsOff()
            hideWatchedToggle.performClick()
            hideWatchedToggle.assertIsOn()
            compose.waitUntil(5_000) {
                compose.onAllNodesWithText("Filme: 1").fetchSemanticsNodes().isNotEmpty()
            }
            compose.onNodeWithTag("movie-tt0068646").assertDoesNotExist()
            compose.onNodeWithTag("movie-tt0111161").assertExists()
            Assert.assertTrue(preferences.getBoolean("hide-watched:test-canon", false))

            compose.onNodeWithTag("back").performClick()
            compose.onNodeWithTag("canon-test-canon").performClick()
            compose.onNodeWithTag("hide-watched-toggle").assertIsOn()
            compose.onNodeWithTag("film-count").assertTextEquals("Filme: 1")
        } finally {
            repository.close()
            preferences.edit().clear().commit()
            fixture.cleanup()
        }
    }
}

private class AppLocaleRule : TestRule {
    override fun apply(base: Statement, description: Description): Statement = object : Statement() {
        override fun evaluate() {
            if (Build.VERSION.SDK_INT < 33) {
                base.evaluate()
                return
            }
            val context = InstrumentationRegistry.getInstrumentation().targetContext
            val manager = context.getSystemService(LocaleManager::class.java)
            val original = manager.applicationLocales
            val locale = when (description.methodName) {
                "localizedUiStartsInEnglish" -> "en"
                "localizedUiStartsInSpanish" -> "es"
                "localizedUiStartsInFrench" -> "fr"
                else -> "de"
            }
            manager.applicationLocales = LocaleList.forLanguageTags(locale)
            try { base.evaluate() } finally { manager.applicationLocales = original }
        }
    }
}

class LiveCatalogSmokeTest {
    @Test fun readsExistingGoApi() {
        Assume.assumeTrue(InstrumentationRegistry.getArguments().getString("liveCatalog") == "true")
        runBlocking {
            val context = InstrumentationRegistry.getInstrumentation().targetContext
            val directory = File(context.cacheDir, "live-catalog-test-${UUID.randomUUID()}").apply { mkdirs() }
            val database = Room.databaseBuilder<CatalogDatabase>(context, File(directory, "catalog.db").absolutePath)
                .setDriver(BundledSQLiteDriver()).setQueryCoroutineContext(Dispatchers.IO).build()
            val repository = CatalogRepository(database, catalogClient(), BuildConfig.API_BASE_URL, File(directory, "posters").absolutePath)
            try {
                Assert.assertTrue(repository.refresh() > 0)
                val catalog = repository.catalogs.first().first { it.canon.slug == "imdb-top-250" }
                Assert.assertEquals(250, catalog.movies.size)
                Assert.assertTrue(catalog.movies.any { it.imdbId == "tt0111161" })
            } finally { repository.close(); directory.deleteRecursively() }
        }
    }
}
