package com.reelcanon.mobile

import android.content.Context
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import kotlinx.coroutines.Dispatchers
import okio.Path.Companion.toPath

actual fun imageCacheDirectory(context: coil3.PlatformContext): okio.Path =
    "${context.cacheDir}/image_cache".toPath()

fun createAndroidRepository(context: Context, apiBase: String, google: GoogleAuthenticator? = null): CatalogRepository {
    val database = Room.databaseBuilder<CatalogDatabase>(context, "reel-canon.db")
        .addMigrations(PROGRESS_MIGRATION)
        .setDriver(BundledSQLiteDriver())
        .setQueryCoroutineContext(Dispatchers.IO)
        .build()
    return CatalogRepository(database, catalogClient(), apiBase, "${context.noBackupFilesDir}/posters", AndroidSessionStore(context, apiBase), google)
}
