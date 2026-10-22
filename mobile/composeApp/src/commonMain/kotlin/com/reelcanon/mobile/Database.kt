package com.reelcanon.mobile

import androidx.room.*
import kotlinx.coroutines.flow.Flow
import androidx.room.migration.Migration
import androidx.sqlite.SQLiteConnection
import androidx.sqlite.execSQL

@Entity(tableName = "cached_canons")
data class CachedCanon(@PrimaryKey val slug: String, val payload: String, val savedAt: String)

@Entity(tableName = "progress", primaryKeys = ["owner", "imdbId"])
data class ProgressEntity(
    val imdbId: String,
    val watched: Boolean = false,
    val watchlist: Boolean = false,
    val watchedAt: String? = null,
    val updatedAt: String,
    val owner: String = "guest",
    val revision: Long = 0,
    val serverWatched: Boolean = false,
    val serverWatchlist: Boolean = false,
) {
    fun model() = MovieProgress(imdbId, watched, watchlist, watchedAt, updatedAt)
}

@Entity(tableName = "sync_outbox", indices = [Index(value = ["operationId"], unique = true)])
data class OutboxEntity(
    @PrimaryKey(autoGenerate = true) val sequence: Long = 0,
    val operationId: String,
    val owner: String,
    val imdbId: String,
    val field: String,
    val value: Boolean,
    val baseValue: Boolean,
    val revision: Long,
    val conflict: Boolean = false,
)

@Dao
interface CatalogDao {
    @Query("SELECT * FROM cached_canons ORDER BY slug")
    fun observeCatalog(): Flow<List<CachedCanon>>
    @Query("SELECT * FROM cached_canons WHERE slug = :slug")
    suspend fun catalog(slug: String): CachedCanon?
    @Upsert
    suspend fun saveCatalog(value: CachedCanon)
    @Query("SELECT * FROM progress WHERE owner = :owner")
    fun observeProgress(owner: String): Flow<List<ProgressEntity>>
    @Query("SELECT * FROM progress WHERE owner = :owner")
    suspend fun allProgress(owner: String): List<ProgressEntity>
    @Query("SELECT * FROM progress WHERE owner = :owner AND imdbId = :imdbId")
    suspend fun progress(owner: String, imdbId: String): ProgressEntity?
    @Upsert
    suspend fun saveProgress(value: ProgressEntity)
    @Query("SELECT * FROM sync_outbox WHERE owner = :owner ORDER BY sequence")
    suspend fun outbox(owner: String): List<OutboxEntity>
    @Query("SELECT * FROM sync_outbox WHERE owner = :owner AND conflict = 1 ORDER BY sequence")
    fun observeConflicts(owner: String): Flow<List<OutboxEntity>>
    @Upsert
    suspend fun saveOperation(value: OutboxEntity)
    @Query("DELETE FROM sync_outbox WHERE sequence = :sequence")
    suspend fun deleteOperation(sequence: Long)
    @Query("DELETE FROM progress WHERE owner = :owner")
    suspend fun clearProgress(owner: String)
}

val PROGRESS_MIGRATION = object : Migration(1, 2) {
    override fun migrate(connection: SQLiteConnection) {
        connection.execSQL("CREATE TABLE progress_v2 (imdbId TEXT NOT NULL, watched INTEGER NOT NULL, watchlist INTEGER NOT NULL, watchedAt TEXT, updatedAt TEXT NOT NULL, owner TEXT NOT NULL, revision INTEGER NOT NULL, serverWatched INTEGER NOT NULL, serverWatchlist INTEGER NOT NULL, PRIMARY KEY(owner, imdbId))")
        connection.execSQL("INSERT INTO progress_v2 SELECT imdbId,watched,watchlist,watchedAt,updatedAt,'guest',0,0,0 FROM progress")
        connection.execSQL("DROP TABLE progress")
        connection.execSQL("ALTER TABLE progress_v2 RENAME TO progress")
        connection.execSQL("CREATE TABLE sync_outbox (sequence INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL, operationId TEXT NOT NULL, owner TEXT NOT NULL, imdbId TEXT NOT NULL, field TEXT NOT NULL, value INTEGER NOT NULL, baseValue INTEGER NOT NULL, revision INTEGER NOT NULL, conflict INTEGER NOT NULL)")
        connection.execSQL("CREATE UNIQUE INDEX index_sync_outbox_operationId ON sync_outbox(operationId)")
    }
}

@Database(entities = [CachedCanon::class, ProgressEntity::class, OutboxEntity::class], version = 2, exportSchema = true)
@ConstructedBy(CatalogDatabaseConstructor::class)
abstract class CatalogDatabase : RoomDatabase() { abstract fun catalogDao(): CatalogDao }

@Suppress("KotlinNoActualForExpect")
expect object CatalogDatabaseConstructor : RoomDatabaseConstructor<CatalogDatabase> {
    override fun initialize(): CatalogDatabase
}
