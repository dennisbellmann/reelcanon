package com.reelcanon.mobile

import androidx.compose.ui.window.ComposeUIViewController
import androidx.room.Room
import androidx.sqlite.driver.bundled.BundledSQLiteDriver
import kotlinx.coroutines.Dispatchers
import platform.Foundation.*
import kotlinx.cinterop.ExperimentalForeignApi
import okio.Path.Companion.toPath

@OptIn(ExperimentalForeignApi::class)
actual fun imageCacheDirectory(context: coil3.PlatformContext): okio.Path =
    (NSSearchPathForDirectoriesInDomains(NSCachesDirectory, NSUserDomainMask, true).first() as String)
        .toPath() / "image_cache"

@OptIn(ExperimentalForeignApi::class)
fun MainViewController(apiBase: String): platform.UIKit.UIViewController {
    val directory = NSFileManager.defaultManager.URLForDirectory(
        NSApplicationSupportDirectory, NSUserDomainMask, null, true, null
    )!!
    val path = requireNotNull(directory.path)
    val posterDirectory = "$path/posters"
    NSFileManager.defaultManager.createDirectoryAtPath(posterDirectory, true, null, null)
    NSURL.fileURLWithPath(posterDirectory).setResourceValue(true, NSURLIsExcludedFromBackupKey, null)
    val database = Room.databaseBuilder<CatalogDatabase>(name = "$path/reel-canon.db")
        .addMigrations(PROGRESS_MIGRATION)
        .setDriver(BundledSQLiteDriver())
        .setQueryCoroutineContext(Dispatchers.IO)
        .build()
    val repository = CatalogRepository(database, catalogClient(), apiBase, posterDirectory)
    return ComposeUIViewController { ReelCanonApp(repository) }
}
