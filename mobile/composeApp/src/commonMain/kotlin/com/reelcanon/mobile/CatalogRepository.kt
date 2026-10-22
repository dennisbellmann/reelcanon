package com.reelcanon.mobile

import io.ktor.client.HttpClient
import io.ktor.client.call.body
import io.ktor.client.plugins.HttpTimeout
import io.ktor.client.plugins.contentnegotiation.ContentNegotiation
import io.ktor.client.request.get
import io.ktor.http.URLBuilder
import io.ktor.http.encodedPath
import io.ktor.serialization.kotlinx.json.json
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import okio.FileSystem
import okio.Path.Companion.toPath
import okio.ByteString.Companion.encodeUtf8
import kotlin.time.Clock

fun catalogClient() = HttpClient {
    expectSuccess = true
    install(ContentNegotiation) { json(catalogJson) }
    install(HttpTimeout) { requestTimeoutMillis = 15_000; connectTimeoutMillis = 10_000 }
}

class CatalogRepository(
    private val database: CatalogDatabase,
    private val client: HttpClient,
    private val apiBase: String,
    private val imageDirectory: String,
    sessionStore: SecureSessionStore? = null,
    googleAuthenticator: GoogleAuthenticator? = null,
) {
    private val mutex = Mutex()
    private val dao = database.catalogDao()
    val catalogs = dao.observeCatalog().map { rows ->
        rows.map { catalogJson.decodeFromString<CanonCatalog>(it.payload) }
            .sortedWith(compareBy<CanonCatalog> { it.canon.displayPosition }.thenBy { it.canon.name }.thenBy { it.canon.slug })
    }
    val account = MobileAccount(database, client, apiBase, sessionStore, googleAuthenticator)
    val progress = account.progress

    suspend fun refresh(onCollection: suspend (Int, Int) -> Unit = { _, _ -> }) = mutex.withLock {
        val canons: List<Canon> = client.get("${apiBase.trimEnd('/')}/canons").body()
        canons.forEachIndexed { index, canon ->
            require(Regex("[a-z0-9-]+").matches(canon.slug)) { "Ungültige Sammlung." }
            val catalog: CanonCatalog = client.get("${apiBase.trimEnd('/')}/canons/${canon.slug}").body()
            catalog.validate(canon.slug)
            // A complete edition replaces the old snapshot in a single SQLite transaction.
            dao.saveCatalog(CachedCanon(canon.slug, catalogJson.encodeToString(catalog), Clock.System.now().toString()))
            onCollection(index + 1, canons.size)
        }
        // Retain cached collections when absent from discovery: progress and offline access survive outages.
        canons.size
    }

    suspend fun setWatched(imdbId: String, watched: Boolean) = account.change(imdbId, "watched", watched)
    suspend fun setWatchlist(imdbId: String, watchlist: Boolean) = account.change(imdbId, "watchlist", watchlist)
    suspend fun exportBackup(): String = account.exportBackup()
    suspend fun importBackup(text: String): Int = account.importBackup(text)

    private fun posterPath(movie: Movie): String? = movie.posterUrl?.let { url ->
        // Include the full URL in the key so a changed poster never silently reuses an old image.
        val hash = url.encodeUtf8().sha256().hex()
        "$imageDirectory/$hash.jpg"
    }

    fun collectionPosterModel(canon: Canon): String =
        if (canon.posterUrl.startsWith("/")) URLBuilder(apiBase).apply {
            encodedPath = canon.posterUrl
            parameters.clear()
            fragment = ""
        }.buildString() else canon.posterUrl

    fun posterModel(movie: Movie): String? {
        val path = posterPath(movie)
        return if (path != null && FileSystem.SYSTEM.exists(path.toPath())) path else movie.posterUrl
    }

    suspend fun downloadedPosters(catalog: CanonCatalog): Int = withContext(Dispatchers.IO) {
        catalog.movies.count { movie -> posterPath(movie)?.let { FileSystem.SYSTEM.exists(it.toPath()) } == true }
    }

    suspend fun downloadPosters(catalog: CanonCatalog, onProgress: suspend (Int, Int) -> Unit): Int = mutex.withLock {
        withContext(Dispatchers.IO) {
            val movies = catalog.movies.filter { it.posterUrl != null }
            FileSystem.SYSTEM.createDirectories(imageDirectory.toPath())
            var failed = 0
            movies.forEachIndexed { index, movie ->
                val target = requireNotNull(posterPath(movie)).toPath()
                if (!FileSystem.SYSTEM.exists(target)) {
                    val temporary = "$target.part".toPath()
                    try {
                        require(movie.posterUrl!!.startsWith("https://")) { "Unsicherer Poster-Link." }
                        val response = client.get(movie.posterUrl)
                        val type = response.headers["Content-Type"].orEmpty()
                        require(type.startsWith("image/")) { "Ungültiges Poster." }
                        val bytes: ByteArray = response.body()
                        require(bytes.isNotEmpty() && bytes.size <= 10_000_000) { "Poster ist zu groß oder leer." }
                        FileSystem.SYSTEM.write(temporary) { write(bytes) }
                        FileSystem.SYSTEM.atomicMove(temporary, target)
                    } catch (error: CancellationException) {
                        FileSystem.SYSTEM.delete(temporary, mustExist = false)
                        throw error
                    } catch (_: Exception) {
                        FileSystem.SYSTEM.delete(temporary, mustExist = false)
                        failed++
                    }
                }
                onProgress(index + 1, movies.size)
            }
            failed
        }
    }

    fun close() { client.close(); database.close() }
}
