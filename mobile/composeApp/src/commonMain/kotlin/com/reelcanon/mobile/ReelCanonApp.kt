package com.reelcanon.mobile

import androidx.compose.foundation.*
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.ColorMatrix
import androidx.compose.ui.graphics.drawscope.clipRect
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalUriHandler
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import coil3.compose.AsyncImage
import coil3.ImageLoader
import coil3.disk.DiskCache
import coil3.memory.MemoryCache
import coil3.compose.setSingletonImageLoaderFactory
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.launch

private val Paper = Color(0xFFF4F1EA)
private val Ink = Color(0xFF171717)
private val Muted = Color(0xFF77736C)
private val Line = Color(0xFFD8D3C9)
private val Accent = Color(0xFFA03027)
private val Serif = FontFamily.Serif

@Composable
fun ReelCanonApp(
    repository: CatalogRepository,
    backHandler: @Composable (Boolean, () -> Unit) -> Unit = { _, _ -> },
    backupExport: ((String) -> Unit)? = null,
    backupImport: (((String) -> Unit) -> Unit)? = null,
    hideWatchedPreference: (String) -> Boolean = { false },
    saveHideWatchedPreference: (String, Boolean) -> Unit = { _, _ -> },
) {
    val t = useStrings()
    setSingletonImageLoaderFactory { context ->
        ImageLoader.Builder(context)
            .memoryCache { MemoryCache.Builder().maxSizePercent(context, 0.20).build() }
            .diskCache { DiskCache.Builder().directory(imageCacheDirectory(context)).maxSizeBytes(150L * 1024 * 1024).build() }
            .build()
    }
    val catalogs by repository.catalogs.collectAsState(emptyList())
    val account by repository.account.state.collectAsState()
    val ownedProgress by repository.account.ownedProgress.collectAsState(null)
    val progress = ownedProgress?.takeIf { it.owner == (account.user?.id ?: "guest") }?.rows ?: emptyMap()
    var selectedSlug by rememberSaveable { mutableStateOf<String?>(null) }
    var selectedMovie by rememberSaveable { mutableStateOf<String?>(null) }
    var hideWatched by remember(selectedSlug) {
        mutableStateOf(selectedSlug?.let(hideWatchedPreference) ?: false)
    }
    var tab by rememberSaveable { mutableStateOf("collections") }
    var album by rememberSaveable { mutableStateOf(false) }
    var query by rememberSaveable { mutableStateOf("") }
    var progressFilter by remember { mutableStateOf(ProgressFilter.ALL) }
    var decade by remember { mutableStateOf<Int?>(null) }
    var year by remember { mutableStateOf<Int?>(null) }
    var genre by remember { mutableStateOf<String?>(null) }
    var director by remember { mutableStateOf<String?>(null) }
    var minimumRating by remember { mutableStateOf(0.0) }
    var sort by remember { mutableStateOf(MovieSort.RANK) }
    var showFilters by remember { mutableStateOf(false) }
    var syncing by remember { mutableStateOf(false) }
    var saving by remember { mutableStateOf(false) }
    var downloading by remember { mutableStateOf(false) }
    var downloadVersion by remember { mutableStateOf(0) }
    var status by remember { mutableStateOf(notice(Message.mobile_loading)) }
    var failed by remember { mutableStateOf(false) }
    var backupText by remember { mutableStateOf("") }
    val scope = rememberCoroutineScope()
    val catalog = catalogs.firstOrNull { it.canon.slug == selectedSlug }
    val allMovies = catalogs.flatMap { it.movies }.distinctBy { it.imdbId }
    val movie = allMovies.firstOrNull { it.imdbId == selectedMovie }

    fun refresh() {
        if (syncing || downloading) return
        scope.launch {
            syncing = true
            failed = false
            status = notice(Message.mobile_refreshing)
            try {
                val count = repository.refresh()
                repository.account.sync()
                status = if (count == 0) notice(Message.mobile_empty) else notice(Message.mobile_current)
            } catch (error: CancellationException) { throw error }
            catch (_: Exception) {
                failed = true
                status = notice(Message.mobile_offline)
            } finally { syncing = false }
        }
    }
    fun save(action: suspend () -> Unit) {
        if (saving) return
        scope.launch {
            saving = true
            try { action() }
            catch (error: CancellationException) { throw error }
            catch (_: Exception) { status = notice(Message.mobile_saveError); failed = true }
            finally { saving = false }
        }
    }
    fun back() {
        when {
            selectedMovie != null -> selectedMovie = null
            selectedSlug != null -> { selectedSlug = null; album = false; query = ""; decade = null }
            tab != "collections" -> tab = "collections"
        }
    }
    backHandler(selectedMovie != null || selectedSlug != null || tab != "collections", ::back)
    LaunchedEffect(repository) { repository.account.start(); refresh() }

    CompositionLocalProvider(LocalStrings provides t) {
    MaterialTheme(
        colorScheme = lightColorScheme(primary = Accent, background = Paper, surface = Paper, onSurface = Ink,
            onBackground = Ink, outline = Line, secondary = Ink, secondaryContainer = Line,
            onSecondaryContainer = Ink, primaryContainer = Color(0xFFEEDAD3), onPrimaryContainer = Ink,
            surfaceVariant = Color(0xFFE9E4DA)),
        typography = Typography(
            headlineLarge = androidx.compose.ui.text.TextStyle(fontFamily = Serif, fontSize = 36.sp, color = Ink),
            headlineMedium = androidx.compose.ui.text.TextStyle(fontFamily = Serif, fontSize = 28.sp, color = Ink),
            titleLarge = androidx.compose.ui.text.TextStyle(fontFamily = Serif, fontSize = 22.sp, color = Ink),
        ),
    ) {
        Scaffold(
            containerColor = Paper,
            topBar = {
                Column(Modifier.statusBarsPadding().fillMaxWidth()) {
                    Row(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 10.dp), verticalAlignment = Alignment.CenterVertically) {
                        if (selectedSlug != null || selectedMovie != null) {
                            TextButton(onClick = ::back, modifier = Modifier.testTag("back")) { Text(t(Message.common_back)) }
                        }
                        Text("Reel Canon", fontFamily = Serif, fontSize = 25.sp, modifier = Modifier.weight(1f))
                        TextButton(onClick = ::refresh, enabled = !syncing && !downloading, modifier = Modifier.testTag("refresh")) { Text("↻", fontSize = 26.sp) }
                    }
                    HorizontalDivider(color = Ink, thickness = 1.dp)
                }
            },
            bottomBar = {
                Column(Modifier.navigationBarsPadding().fillMaxWidth()) {
                    HorizontalDivider(color = Line)
                    Row(Modifier.fillMaxWidth()) {
                        listOf("collections", "watchlist", "settings").forEach { item ->
                            TextButton(
                                onClick = { tab = item; selectedSlug = null; selectedMovie = null; query = ""; decade = null; year = null; genre = null; director = null; minimumRating = 0.0; progressFilter = ProgressFilter.ALL },
                                modifier = Modifier.weight(1f).heightIn(min = 56.dp).testTag("tab-$item"),
                            ) { Text(t(when (item) { "collections" -> Message.nav_collections; "watchlist" -> Message.filter_watchlist; else -> Message.nav_settings }), color = if (tab == item) Accent else Muted, fontWeight = if (tab == item) FontWeight.Bold else FontWeight.Normal) }
                        }
                    }
                }
            },
        ) { insets ->
            Column(Modifier.padding(insets).fillMaxSize()) {
                if (account.notice != null && account.notice != Message.mobile_synced) {
                    Text(t(account.notice!!), color = Accent, fontSize = 12.sp, modifier = Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 8.dp).testTag("account-notice"))
                }
                if (failed || syncing || downloading) {
                    Text(t.render(status), color = if (failed) Accent else Muted, fontSize = 12.sp,
                        modifier = Modifier.fillMaxWidth().background(Color(0xFFE9E4DA)).padding(horizontal = 20.dp, vertical = 10.dp).testTag("status"))
                }
                when {
                    movie != null -> MovieDetail(movie, catalogs, progress[movie.imdbId], repository, saving,
                        onWatched = { save { repository.setWatched(movie.imdbId, progress[movie.imdbId]?.watched != true) } },
                        onWatchlist = { save { repository.setWatchlist(movie.imdbId, progress[movie.imdbId]?.watchlist != true) } },
                    )
                    tab == "settings" -> {
                        Column(Modifier.verticalScroll(rememberScrollState()).padding(24.dp), verticalArrangement = Arrangement.spacedBy(18.dp)) {
                            
                            Text(t(Message.nav_settings), style = MaterialTheme.typography.headlineLarge)
                            AccountSettings(repository.account, allMovies)
                            Text(t(Message.language_system), color = Muted)
                            Text(t(Message.mobile_backupIntro), color = Muted)
                            Text(t(Message.mobile_backupSummary, "watched" to progress.values.count { it.watched }, "watchlist" to progress.values.count { it.watchlist }))
                            OutlinedButton(onClick = {
                                save {
                                    val text = repository.exportBackup()
                                    if (backupExport != null) backupExport(text) else backupText = text
                                }
                            }, enabled = !saving, modifier = Modifier.fillMaxWidth()) { Text(t(Message.backup_export)) }
                            if (backupImport != null && account.user == null) {
                                OutlinedButton(onClick = {
                                    backupImport { text -> save {
                                        val count = repository.importBackup(text)
                                        status = notice(Message.mobile_backupImported, "count" to count); failed = true
                                    } }
                                }, enabled = !saving, modifier = Modifier.fillMaxWidth()) { Text(t(Message.backup_import)) }
                            } else if (account.user == null) {
                                OutlinedTextField(value = backupText, onValueChange = { backupText = it }, label = { Text(t(Message.mobile_jsonBackup)) }, modifier = Modifier.fillMaxWidth().heightIn(max = 200.dp))
                                OutlinedButton(onClick = { save { repository.importBackup(backupText); backupText = "" } }, enabled = !saving && backupText.isNotBlank()) { Text(t(Message.backup_read)) }
                            }
                            HorizontalDivider(color = Line)
                            Text(t(Message.mobile_preview), style = MaterialTheme.typography.titleLarge)
                            Text(t(Message.mobile_previewHint), color = Muted)
                            Text(t(Message.mobile_tmdb), color = Muted, fontSize = 12.sp)
                            Text(t(Message.mobile_archive), color = Muted, fontSize = 12.sp)
                        }
                    }
                    catalog == null && tab == "collections" -> {
                        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(24.dp), verticalArrangement = Arrangement.spacedBy(24.dp)) {
                            item {
                                
                                Spacer(Modifier.height(10.dp))
                                Text(t(Message.index_title), style = MaterialTheme.typography.headlineLarge, fontSize = 42.sp, lineHeight = 46.sp)
                                Spacer(Modifier.height(12.dp))
                                Text(t(Message.index_intro), color = Muted, lineHeight = 23.sp)
                            }
                            if (catalogs.isEmpty()) item {
                                Text(if (syncing) t(Message.index_loading) else t(Message.mobile_emptyHint), color = Muted)
                                OutlinedButton(onClick = ::refresh, enabled = !syncing) { Text(t(Message.mobile_retry)) }
                            }
                            items(catalogs, key = { it.canon.slug }) { entry ->
                                CanonCard(entry, progress, repository) {
                                    selectedSlug = entry.canon.slug; progressFilter = ProgressFilter.ALL
                                    query = ""; decade = null; year = null; genre = null; director = null; minimumRating = 0.0; sort = MovieSort.RANK
                                }
                            }
                            item { Text(t.render(status), fontSize = 12.sp, color = Muted) }
                        }
                    }
                    else -> {
                        val movies = catalog?.movies ?: allMovies
                        val activeProgressFilter = if (tab == "watchlist") ProgressFilter.WATCHLIST else progressFilter
                        val filter = MovieFilter(
                            query = query,
                            progress = activeProgressFilter,
                            decade = decade,
                            year = year,
                            genre = genre,
                            director = director,
                            minimumRating = minimumRating,
                            sort = sort,
                            hideWatched = catalog != null && hideWatched,
                        )
                        val filtered = filterMovies(movies, progress, filter)
                        Column(Modifier.fillMaxWidth().padding(horizontal = 20.dp, vertical = 12.dp)) {
                            
                            Text(catalog?.canon?.name ?: t(Message.filter_watchlist), style = MaterialTheme.typography.headlineMedium)
                            if (catalog != null) {
                                val watched = movies.count { progress[it.imdbId]?.watched == true }
                                ProgressLine(watched, movies.size)
                                Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                    TextButton(onClick = { album = !album; decade = null }, modifier = Modifier.testTag("album-toggle")) { Text(if (album) t(Message.mobile_list) else t(Message.mobile_album)) }
                                    Spacer(Modifier.weight(1f))
                                    TextButton(onClick = {
                                        scope.launch {
                                            downloading = true; failed = false
                                            try {
                                                val errors = repository.downloadPosters(catalog) { done, total -> status = notice(Message.mobile_downloading, "done" to done, "total" to total) }
                                                status = if (errors == 0) notice(Message.mobile_postersSaved) else notice(Message.mobile_postersMissing, "count" to errors)
                                                failed = true; downloadVersion++
                                            } catch (error: CancellationException) { throw error }
                                            catch (_: Exception) { status = notice(Message.mobile_postersError); failed = true }
                                            finally { downloading = false }
                                        }
                                    }, enabled = !downloading && !syncing, modifier = Modifier.testTag("download")) { Text(t(Message.mobile_offlineAction)) }
                                }
                                var stored by remember(catalog, downloadVersion) { mutableStateOf(0) }
                                LaunchedEffect(catalog, downloadVersion) { stored = repository.downloadedPosters(catalog) }

                            }
                            Spacer(Modifier.height(8.dp))
                            OutlinedTextField(value = query, onValueChange = { query = it }, placeholder = { Text(t(Message.collection_searchHint)) }, singleLine = true,
                                modifier = Modifier.fillMaxWidth().testTag("search"))
                            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                                Text(t(Message.mobile_filmCount, "count" to filtered.size), fontSize = 12.sp, color = Muted, modifier = Modifier.weight(1f).testTag("film-count"))
                                TextButton(onClick = { showFilters = true }, modifier = Modifier.testTag("filters")) { Text(t(Message.mobile_filterLabel, "sort" to t(sort.message))) }
                            }
                            if (catalog != null) {
                                Row(
                                    Modifier.fillMaxWidth().testTag("hide-watched-row"),
                                    verticalAlignment = Alignment.CenterVertically,
                                ) {
                                    Text(
                                        t(Message.collection_hideWatched),
                                        fontSize = 14.sp,
                                        modifier = Modifier.weight(1f),
                                    )
                                    Switch(
                                        checked = hideWatched,
                                        onCheckedChange = { hidden ->
                                            hideWatched = hidden
                                            saveHideWatchedPreference(catalog.canon.slug, hidden)
                                        },
                                        modifier = Modifier
                                            .testTag("hide-watched-toggle")
                                            .semantics {
                                                contentDescription = t(Message.collection_hideWatched)
                                            },
                                    )
                                }
                            }
                            if (album && catalog != null) {
                                val decades = movies.mapNotNull { it.decade }.distinct().sorted()
                                Row(Modifier.horizontalScroll(rememberScrollState()), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    FilterChip(selected = decade == null, onClick = { decade = null }, label = { Text(t(Message.mobile_all)) })
                                    decades.forEach { value -> FilterChip(selected = decade == value, onClick = { decade = value }, label = { Text(t(Message.collection_decade, "year" to value)) }, modifier = Modifier.testTag("decade-$value")) }
                                }
                            }
                        }
                        if (filtered.isEmpty()) {
                            Text(t(Message.mobile_emptyResults), color = Muted, modifier = Modifier.padding(24.dp).testTag("empty-results"))
                        } else if (album && catalog != null) {
                            LazyVerticalGrid(columns = GridCells.Adaptive(100.dp), modifier = Modifier.weight(1f), contentPadding = PaddingValues(horizontal = 20.dp, vertical = 8.dp),
                                horizontalArrangement = Arrangement.spacedBy(12.dp), verticalArrangement = Arrangement.spacedBy(20.dp)) {
                                items(filtered, key = { it.imdbId }) { film ->
                                    AlbumTile(film, progress[film.imdbId]?.watched == true, repository, downloadVersion) { selectedMovie = film.imdbId }
                                }
                            }
                        } else {
                            LazyColumn(Modifier.weight(1f), contentPadding = PaddingValues(horizontal = 20.dp, vertical = 8.dp)) {
                                items(filtered, key = { it.imdbId }) { film ->
                                    MovieRow(film, progress[film.imdbId], repository, saving,
                                        onClick = { selectedMovie = film.imdbId },
                                        onWatched = { save { repository.setWatched(film.imdbId, progress[film.imdbId]?.watched != true) } },
                                    )
                                }
                            }
                        }
                        if (showFilters) {
                            AlertDialog(onDismissRequest = { showFilters = false }, title = { Text(t(Message.mobile_selection), fontFamily = Serif) },
                                text = {
                                    Column(Modifier.verticalScroll(rememberScrollState()), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                                        if (tab != "watchlist") Choice(t(Message.mobile_progress), ProgressFilter.entries, progressFilter, { t(it.message) }) { progressFilter = it }
                                        Choice(t(Message.mobile_sort), MovieSort.entries, sort, { t(it.message) }) { sort = it }
                                        Choice(t(Message.mobile_decade), listOf(null) + movies.mapNotNull { it.decade }.distinct().sorted(), decade, { it?.let { value -> t(Message.collection_decade, "year" to value) } ?: t(Message.mobile_all) }) { decade = it }
                                        Choice(t(Message.filter_year), listOf(null) + movies.mapNotNull { it.releaseYear }.distinct().sorted(), year, { it?.toString() ?: t(Message.mobile_all) }) { year = it }
                                        Choice(t(Message.filter_genre), listOf(null) + movies.flatMap { it.genres }.distinct().sorted(), genre, { it ?: t(Message.mobile_all) }) { genre = it }
                                        Choice(t(Message.filter_director), listOf(null) + movies.flatMap { it.directors }.distinct().sorted(), director, { it ?: t(Message.mobile_all) }) { director = it }
                                        Choice(t(Message.mobile_minRating), listOf(0.0, 7.0, 8.0, 8.5, 9.0), minimumRating, { if (it == 0.0) t(Message.mobile_all) else it.toString() }) { minimumRating = it }
                                    }
                                },
                                confirmButton = { TextButton(onClick = { showFilters = false }) { Text(t(Message.mobile_apply)) } },
                                dismissButton = { TextButton(onClick = { progressFilter = ProgressFilter.ALL; decade = null; year = null; genre = null; director = null; minimumRating = 0.0; sort = MovieSort.RANK; query = ""; showFilters = false }) { Text(t(Message.mobile_reset)) } },
                            )
                        }
                    }
                }
            }
        }
    }
}

}

@Composable
private fun Eyebrow(text: String) { Text(text, color = Accent, fontSize = 10.sp, letterSpacing = 1.5.sp, fontWeight = FontWeight.Bold) }

@Composable
private fun ProgressLine(watched: Int, total: Int) {
    val t = LocalStrings.current
    val ratio = if (total == 0) 0f else watched.toFloat() / total
    Row(Modifier.fillMaxWidth().padding(top = 10.dp, bottom = 6.dp)) {
        Text(t(Message.collection_progressSummary, "watched" to watched, "total" to total), fontSize = 12.sp, color = Muted, modifier = Modifier.weight(1f).testTag("progress-count"))
        Text("${(ratio * 100).toInt()} %", fontSize = 12.sp, color = Accent)
    }
    LinearProgressIndicator(progress = { ratio }, modifier = Modifier.fillMaxWidth().height(3.dp), color = Accent, trackColor = Line)
}

@Composable
private fun CanonCard(catalog: CanonCatalog, progress: Map<String, MovieProgress>, repository: CatalogRepository, onClick: () -> Unit) {
    val t = LocalStrings.current
    val watched = catalog.movies.count { progress[it.imdbId]?.watched == true }
    Column(Modifier.fillMaxWidth().border(1.dp, Line).clickable(onClick = onClick).testTag("canon-${catalog.canon.slug}")) {
        CollectionPoster(catalog.canon, watched, repository)
        Column(Modifier.padding(18.dp)) {
            Eyebrow(t(Message.mobile_collectionCount, "count" to catalog.canon.filmCount))
            Text(catalog.canon.name, style = MaterialTheme.typography.headlineMedium, modifier = Modifier.padding(top = 8.dp))

            ProgressLine(watched, catalog.movies.size)
            Text(t(Message.mobile_openCollection), color = Accent, fontSize = 13.sp, modifier = Modifier.padding(top = 16.dp))
        }
    }
}

@Composable
private fun CollectionPoster(canon: Canon, watched: Int, repository: CatalogRepository) {
    val t = LocalStrings.current
    val ratio = if (canon.filmCount > 0) (watched.toFloat() / canon.filmCount).coerceIn(0f, 1f) else 0f
    val model = repository.collectionPosterModel(canon)
    Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).background(Paper)) {
        Text(canon.name, fontFamily = Serif, fontSize = 30.sp, modifier = Modifier.align(Alignment.Center).padding(24.dp))
        AsyncImage(model = model, contentDescription = t(Message.movie_poster, "title" to canon.name), modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop,
            colorFilter = ColorFilter.colorMatrix(ColorMatrix().apply { setToSaturation(0f) }))
        AsyncImage(model = model, contentDescription = null, modifier = Modifier.fillMaxSize().drawWithContent {
            clipRect(top = size.height * (1f - ratio)) { this@drawWithContent.drawContent() }
        }, contentScale = ContentScale.Crop)
        if (!canon.posterTitleEmbedded) Text(canon.name, fontFamily = Serif, fontSize = 30.sp, color = Ink,
            modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().background(Paper.copy(alpha = 0.94f)).padding(start = 24.dp, end = 24.dp, top = 16.dp, bottom = 60.dp))
        PosterBadge(watched, canon.filmCount, Modifier.align(Alignment.BottomEnd).padding(12.dp))
    }
}

@Composable
private fun PosterBadge(watched: Int, total: Int, modifier: Modifier = Modifier) {
    val t = LocalStrings.current
    Text("✓  $watched/$total", color = Color.White, fontSize = 12.sp,
        modifier = modifier.background(Accent).padding(horizontal = 12.dp, vertical = 8.dp)
            .semantics { contentDescription = t(Message.collection_progressSummary, "watched" to watched, "total" to total) }.testTag("poster-badge"))
}

@Composable
private fun Poster(movie: Movie, repository: CatalogRepository, modifier: Modifier, muted: Boolean = false, version: Int = 0) {
    val t = LocalStrings.current
    val model = remember(movie.posterUrl, version) { repository.posterModel(movie) }
    Box(modifier.background(Color(0xFFE4DFD5)), contentAlignment = Alignment.Center) {
        Text(movie.title, fontFamily = Serif, fontSize = 14.sp, color = Muted, maxLines = 4, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(8.dp))
        if (model != null) AsyncImage(model = model, contentDescription = t(Message.movie_poster, "title" to movie.title), modifier = Modifier.fillMaxSize(), contentScale = ContentScale.Crop,
            colorFilter = if (muted) ColorFilter.colorMatrix(ColorMatrix().apply { setToSaturation(0f) }) else null)
    }
}

@Composable
private fun MovieRow(movie: Movie, progress: MovieProgress?, repository: CatalogRepository, saving: Boolean, onClick: () -> Unit, onWatched: () -> Unit) {
    val t = LocalStrings.current
    Row(Modifier.fillMaxWidth().padding(vertical = 12.dp), horizontalArrangement = Arrangement.spacedBy(14.dp), verticalAlignment = Alignment.CenterVertically) {
        Poster(movie, repository, Modifier.width(62.dp).height(93.dp).clickable(onClick = onClick))
        Column(Modifier.weight(1f).clickable(onClick = onClick).testTag("movie-${movie.imdbId}")) {
            Text("#${movie.position.toString().padStart(3, '0')}", fontSize = 10.sp, letterSpacing = 1.sp, color = Accent)
            Text(movie.title, fontFamily = Serif, fontSize = 19.sp, maxLines = 2, overflow = TextOverflow.Ellipsis)
            Text(listOfNotNull(movie.releaseYear?.toString(), movie.runtimeMinutes?.let { t(Message.movie_runtime, "minutes" to it) }, movie.imdbRating?.let { "★ $it" }).joinToString(" · "), fontSize = 11.sp, color = Muted)
            if (movie.directors.isNotEmpty()) Text(movie.directors.joinToString(), fontSize = 11.sp, color = Muted, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (progress?.watchlist == true) Text(t(Message.mobile_inWatchlist), fontSize = 10.sp, color = Accent)
        }
        TextButton(onClick = onWatched, enabled = !saving, modifier = Modifier.widthIn(min = 48.dp).testTag("watched-${movie.imdbId}").semantics {
            contentDescription = if (progress?.watched == true) t(Message.movie_unwatchedLabel, "title" to movie.title) else t(Message.movie_watchedLabel, "title" to movie.title)
        }) {
            Text(if (progress?.watched == true) "✓" else "+", fontSize = 26.sp, color = if (progress?.watched == true) Accent else Muted)
        }
    }
    HorizontalDivider(color = Line)
}

@Composable
private fun AlbumTile(movie: Movie, watched: Boolean, repository: CatalogRepository, version: Int, onClick: () -> Unit) {
    val t = LocalStrings.current
    Column(Modifier.clickable(onClick = onClick).testTag("album-${movie.imdbId}")) {
        Poster(movie, repository, Modifier.fillMaxWidth().aspectRatio(2f / 3f), muted = !watched, version = version)
        Text(movie.title, fontFamily = Serif, fontSize = 14.sp, maxLines = 2, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 6.dp))
        Text("${movie.releaseYear ?: "—"}  ${if (watched) t(Message.mobile_watched) else ""}", fontSize = 10.sp, color = if (watched) Accent else Muted)
    }
}

@Composable
private fun MovieDetail(movie: Movie, catalogs: List<CanonCatalog>, progress: MovieProgress?, repository: CatalogRepository, saving: Boolean, onWatched: () -> Unit, onWatchlist: () -> Unit) {
    val t = LocalStrings.current
    val uriHandler = LocalUriHandler.current
    Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp), verticalArrangement = Arrangement.spacedBy(16.dp)) {
        Eyebrow("#${movie.position.toString().padStart(3, '0')} · ${movie.releaseYear ?: ""}")
        Text(movie.title, style = MaterialTheme.typography.headlineLarge, modifier = Modifier.testTag("detail-title"))
        if (movie.originalTitle != null && movie.originalTitle != movie.title) Text(movie.originalTitle, fontFamily = Serif, color = Muted)
        Poster(movie, repository, Modifier.width(180.dp).height(270.dp).align(Alignment.CenterHorizontally))
        Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            Button(onClick = onWatched, enabled = !saving, modifier = Modifier.weight(1f).testTag("detail-watched")) { Text(if (progress?.watched == true) t(Message.mobile_watched) else t(Message.movie_markWatched)) }
            OutlinedButton(onClick = onWatchlist, enabled = !saving, modifier = Modifier.weight(1f).testTag("detail-watchlist")) { Text(if (progress?.watchlist == true) t(Message.mobile_watchlistSelected) else t(Message.mobile_watchlistAdd)) }
        }
        Text(listOfNotNull(movie.releaseYear?.toString(), movie.runtimeMinutes?.let { t(Message.movie_runtime, "minutes" to it) }, movie.imdbRating?.let { "IMDb ★ $it" }).joinToString(" · "), fontSize = 13.sp, color = Muted)
        if (movie.directors.isNotEmpty()) Text(t(Message.mobile_directors, "names" to movie.directors.joinToString()))
        if (movie.genres.isNotEmpty()) Text(movie.genres.joinToString(" · "), color = Muted, fontSize = 13.sp)
        if (movie.countries.isNotEmpty()) Text(movie.countries.joinToString(" · "), color = Muted, fontSize = 13.sp)
        HorizontalDivider(color = Line)
        Text(movie.overview.ifBlank { t(Message.movie_noOverview) }, lineHeight = 25.sp)
        Text(t(Message.mobile_inCollection), style = MaterialTheme.typography.titleLarge)
        catalogs.filter { it.movies.any { film -> film.imdbId == movie.imdbId } }.forEach { Text(it.canon.name, color = Accent) }
        TextButton(onClick = { uriHandler.openUri("https://www.imdb.com/title/${movie.imdbId}/") }) { Text(t(Message.movie_imdb)) }

    }
}

@Composable
private fun <T> Choice(label: String, choices: List<T>, selected: T, labelFor: (T) -> String, onSelect: (T) -> Unit) {
    var expanded by remember { mutableStateOf(false) }
    Box {
        OutlinedButton(onClick = { expanded = true }, modifier = Modifier.fillMaxWidth()) { Text("$label: ${labelFor(selected)}", maxLines = 1, overflow = TextOverflow.Ellipsis) }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }, modifier = Modifier.heightIn(max = 300.dp)) {
            choices.forEach { item -> DropdownMenuItem(text = { Text(labelFor(item)) }, onClick = { onSelect(item); expanded = false }) }
        }
    }
}
