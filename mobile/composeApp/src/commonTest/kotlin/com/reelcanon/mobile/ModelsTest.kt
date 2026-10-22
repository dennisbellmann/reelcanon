package com.reelcanon.mobile

import kotlin.test.*

class ModelsTest {
    private val film = Movie("tt0068646", "Der Pate", "The Godfather", 1972, imdbRating = 9.2,
        directors = listOf("Francis Ford Coppola"), genres = listOf("Drama"), position = 2)
    private val second = Movie("tt0111161", "Die Verurteilten", releaseYear = 1994, position = 1)
    private val state = MovieProgress(film.imdbId, true, true, "2026-10-06T06:00:00Z", "2026-10-06T06:00:00Z")

    @Test fun searchMatchesOriginalTitleAndDirector() {
        assertEquals(listOf(film), filterMovies(listOf(second, film), emptyMap(), MovieFilter(query = "godfather")))
        assertEquals(listOf(film), filterMovies(listOf(film), emptyMap(), MovieFilter(query = "coppola")))
        assertEquals(listOf(film), filterMovies(listOf(film), emptyMap(), MovieFilter(query = "  PATE  ")))
    }
    @Test fun combinesProgressDecadeAndRating() {
        val filter = MovieFilter(progress = ProgressFilter.WATCHED, decade = 1970, minimumRating = 9.0)
        assertEquals(listOf(film), filterMovies(listOf(film, second), mapOf(film.imdbId to state), filter))
        assertTrue(filterMovies(listOf(film), mapOf(film.imdbId to state), filter.copy(decade = 1990)).isEmpty())
        assertEquals(listOf(second), filterMovies(listOf(film, second), mapOf(film.imdbId to state), MovieFilter(progress = ProgressFilter.UNWATCHED)))
    }
    @Test fun hidesWatchedMoviesWithoutChangingOtherFilters() {
        assertEquals(
            listOf(second),
            filterMovies(
                listOf(film, second),
                mapOf(film.imdbId to state),
                MovieFilter(hideWatched = true),
            ),
        )
        assertEquals(
            listOf(second),
            filterMovies(
                listOf(film, second),
                mapOf(film.imdbId to state),
                MovieFilter(decade = 1990, hideWatched = true),
            ),
        )
    }
    @Test fun unknownRatingSortsLast() {
        assertEquals(listOf(film, second), filterMovies(listOf(second, film), emptyMap(), MovieFilter(sort = MovieSort.RATING)))
    }
    @Test fun incompleteAndDuplicateEditionsAreRejected() {
        val canon = Canon("test", "Test", filmCount = 2, currentEdition = "v1")
        assertFails { CanonCatalog(canon, listOf(film)).validate("test") }
        assertFails { CanonCatalog(canon, listOf(film, film)).validate("test") }
        assertFails { CanonCatalog(canon, listOf(film, second)).validate("other") }
        CanonCatalog(canon, listOf(film, second)).validate("test")
    }
    @Test fun readsWebBackupWithoutChangingTimestamps() {
        val parsed = parseBackup("""{"format":"reel-canon-progress/v1","exportedAt":"2026-10-06T06:00:00Z","progress":[{"imdbId":"tt0068646","watched":true,"watchlist":true,"watchedAt":"2026-10-06T06:00:00Z","updatedAt":"2026-10-06T06:00:00Z"}]}""")
        assertEquals(listOf(state), parsed)
    }
    @Test fun rejectsInvalidBackupsBeforeWritingAnything() {
        val row = """{"imdbId":"tt0068646","watched":false,"watchlist":true,"watchedAt":null,"updatedAt":"2026-10-06T06:00:00Z"}"""
        fun backup(rows: String) = """{"format":"reel-canon-progress/v1","exportedAt":"2026-10-06T06:00:00Z","progress":[$rows]}"""
        assertFails { parseBackup(backup("$row,$row")) }
        assertFails { parseBackup(backup(row.replace("tt0068646", "bad"))) }
        assertFails { parseBackup(backup(row.replace("false", "\"false\""))) }
        assertFails { parseBackup(backup(row.replace("2026-10-06T06:00:00Z", "invalid"))) }
        assertFails { parseBackup(backup(row).replace("progress/v1", "progress/v2")) }
    }
}
