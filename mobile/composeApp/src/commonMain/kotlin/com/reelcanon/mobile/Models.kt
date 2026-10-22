package com.reelcanon.mobile

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.booleanOrNull
import kotlin.time.Instant

val catalogJson = Json { ignoreUnknownKeys = true; coerceInputValues = true; encodeDefaults = true }
private val imdbIdentity = Regex("tt[0-9]{7,10}")

@Serializable
data class Canon(
    val slug: String,
    val name: String,
    val description: String = "",
    val source: String = "",
    val filmCount: Int,
    val currentEdition: String? = null,
    val imdbIds: List<String> = emptyList(),
    val displayPosition: Int = 0,
    val posterUrl: String = "/artwork/collection-03-filmwelten.png",
    val posterTitleEmbedded: Boolean = false,
)

@Serializable
data class Movie(
    val imdbId: String,
    val title: String,
    val originalTitle: String? = null,
    val releaseYear: Int? = null,
    val runtimeMinutes: Int? = null,
    val overview: String = "",
    val posterUrl: String? = null,
    val imdbRating: Double? = null,
    val directors: List<String> = emptyList(),
    val genres: List<String> = emptyList(),
    val countries: List<String> = emptyList(),
    val position: Int = 0,
) {
    val decade: Int? get() = releaseYear?.let { it / 10 * 10 }
}

@Serializable
data class CanonCatalog(val canon: Canon, val movies: List<Movie>) {
    fun validate(expectedSlug: String) {
        require(canon.slug == expectedSlug && canon.currentEdition != null) { "Ungültige Sammlung." }
        require(canon.filmCount > 0 && movies.size == canon.filmCount) { "Sammlung ist unvollständig." }
        require(movies.all { imdbIdentity.matches(it.imdbId) && it.title.isNotBlank() }) { "Ungültige Filmdaten." }
        require(movies.map { it.imdbId }.toSet().size == movies.size) { "Doppelte Filme." }
    }
}

@Serializable
data class MovieProgress(
    val imdbId: String,
    val watched: Boolean = false,
    val watchlist: Boolean = false,
    val watchedAt: String? = null,
    val updatedAt: String,
)

@Serializable
data class ProgressBackup(
    val format: String = "reel-canon-progress/v1",
    val exportedAt: String,
    val progress: List<MovieProgress>,
)

fun parseBackup(text: String): List<MovieProgress> {
    require(text.length <= 5_000_000) { "Die Sicherung ist zu groß (maximal 5 MB)." }
    // Strict decoding: never coerce invalid booleans or malformed fields in user data.
    val strictJson = Json { ignoreUnknownKeys = true }
    val root = strictJson.parseToJsonElement(text).jsonObject
    require(root.keys.containsAll(listOf("format", "progress"))) { "Unbekanntes Sicherungsformat." }
    val entries = root.getValue("progress").jsonArray
    require(entries.size <= 20_000) { "Zu viele Filme in der Sicherung." }
    entries.forEach {
        val row = it.jsonObject
        require(row.keys.containsAll(listOf("imdbId", "watched", "watchlist", "watchedAt", "updatedAt"))) { "Unvollständiger Fortschritt." }
        listOf("watched", "watchlist").forEach { field ->
            val value = row.getValue(field).jsonPrimitive
            require(!value.isString && value.booleanOrNull != null) { "Ungültiger Filmstatus." }
        }
    }
    val backup = strictJson.decodeFromString<ProgressBackup>(text)
    require(backup.format == "reel-canon-progress/v1" && backup.progress.size <= 20_000) {
        "Unbekanntes Sicherungsformat."
    }
    require(backup.progress.map { it.imdbId }.toSet().size == backup.progress.size) { "Doppelte Film-IDs." }
    backup.progress.forEach {
        require(imdbIdentity.matches(it.imdbId)) { "Ungültige Film-ID." }
        Instant.parse(it.updatedAt)
        if (it.watched) Instant.parse(requireNotNull(it.watchedAt)) else require(it.watchedAt == null)
    }
    return backup.progress
}

enum class ProgressFilter(val message: Message) {
    ALL(Message.mobile_all), UNWATCHED(Message.filter_unwatched), WATCHED(Message.filter_watched), WATCHLIST(Message.filter_watchlist)
}
enum class MovieSort(val message: Message) { RANK(Message.filter_rank), TITLE(Message.filter_title), YEAR(Message.filter_year), RATING(Message.filter_imdb) }

data class MovieFilter(
    val query: String = "",
    val progress: ProgressFilter = ProgressFilter.ALL,
    val decade: Int? = null,
    val year: Int? = null,
    val genre: String? = null,
    val director: String? = null,
    val minimumRating: Double = 0.0,
    val sort: MovieSort = MovieSort.RANK,
    val hideWatched: Boolean = false,
)

fun filterMovies(movies: List<Movie>, progress: Map<String, MovieProgress>, filter: MovieFilter): List<Movie> {
    val query = filter.query.trim()
    val selected = movies.filter { movie ->
        val state = progress[movie.imdbId]
        (query.isEmpty() || listOf(movie.title, movie.originalTitle.orEmpty(), movie.directors.joinToString())
            .any { it.contains(query, ignoreCase = true) }) &&
            when (filter.progress) {
                ProgressFilter.ALL -> true
                ProgressFilter.UNWATCHED -> state?.watched != true
                ProgressFilter.WATCHED -> state?.watched == true
                ProgressFilter.WATCHLIST -> state?.watchlist == true
            } &&
            (!filter.hideWatched || state?.watched != true) &&
            (filter.decade == null || movie.decade == filter.decade) &&
            (filter.year == null || movie.releaseYear == filter.year) &&
            (filter.genre == null || filter.genre in movie.genres) &&
            (filter.director == null || filter.director in movie.directors) &&
            (filter.minimumRating == 0.0 || (movie.imdbRating ?: 0.0) >= filter.minimumRating)
    }
    return when (filter.sort) {
        MovieSort.RANK -> selected.sortedBy { it.position }
        MovieSort.TITLE -> selected.sortedBy { it.title.lowercase() }
        MovieSort.YEAR -> selected.sortedWith(compareBy<Movie> { it.releaseYear ?: Int.MAX_VALUE }.thenBy { it.title })
        MovieSort.RATING -> selected.sortedWith(compareByDescending<Movie> { it.imdbRating ?: -1.0 }.thenBy { it.position })
    }
}
