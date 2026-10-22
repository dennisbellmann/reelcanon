package com.reelcanon.app

import android.os.Bundle
import android.net.ConnectivityManager
import android.net.Network
import androidx.lifecycle.lifecycleScope
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.*
import android.widget.Toast
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlin.time.Clock
import com.reelcanon.mobile.*

class MainActivity : ComponentActivity() {
    private lateinit var repository: CatalogRepository
    private val connectivity by lazy { getSystemService(ConnectivityManager::class.java) }
    private val networkCallback = object : ConnectivityManager.NetworkCallback() {
        override fun onAvailable(network: Network) { lifecycleScope.launch { repository.account.sync() } }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        repository = createAndroidRepository(applicationContext, BuildConfig.API_BASE_URL, AndroidGoogleAuthenticator(this))
        val collectionPreferences = getSharedPreferences("collection-preferences", MODE_PRIVATE)
        connectivity.registerDefaultNetworkCallback(networkCallback)
        setContent {
            val t = useStrings()
            val scope = rememberCoroutineScope()
            var pendingExport by remember { mutableStateOf("") }
            var pendingImport by remember { mutableStateOf<((String) -> Unit)?>(null) }
            val exportLauncher = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/json")) { uri ->
                if (uri != null) scope.launch {
                    try {
                        withContext(Dispatchers.IO) {
                            requireNotNull(contentResolver.openOutputStream(uri)).use { it.write(pendingExport.toByteArray()) }
                        }
                        Toast.makeText(this@MainActivity, t(Message.mobile_backupSaved), Toast.LENGTH_SHORT).show()
                    } catch (_: Exception) {
                        Toast.makeText(this@MainActivity, t(Message.mobile_backupSaveError), Toast.LENGTH_LONG).show()
                    } finally { pendingExport = "" }
                }
            }
            val importLauncher = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
                if (uri != null) scope.launch {
                    try {
                        val text = withContext(Dispatchers.IO) {
                            requireNotNull(contentResolver.openInputStream(uri)).use { input ->
                                val bytes = ByteArray(5_000_001)
                                var count = 0
                                while (count < bytes.size) {
                                    val read = input.read(bytes, count, bytes.size - count)
                                    if (read < 0) break
                                    count += read
                                }
                                require(count <= 5_000_000) { "Die Sicherung ist zu groß." }
                                String(bytes, 0, count, Charsets.UTF_8)
                            }
                        }
                        pendingImport?.invoke(text)
                    } catch (_: Exception) {
                        Toast.makeText(this@MainActivity, t(Message.mobile_backupReadError), Toast.LENGTH_LONG).show()
                    } finally { pendingImport = null }
                }
            }
            ReelCanonApp(repository, backHandler = { enabled, action ->
                BackHandler(enabled = enabled, onBack = action)
            }, backupExport = { text ->
                pendingExport = text
                exportLauncher.launch("reel-canon-${Clock.System.now().toString().take(10)}.json")
            }, backupImport = { callback ->
                pendingImport = callback
                importLauncher.launch(arrayOf("application/json", "text/plain", "application/octet-stream"))
            }, hideWatchedPreference = { slug ->
                collectionPreferences.getBoolean("hide-watched:$slug", false)
            }, saveHideWatchedPreference = { slug, hidden ->
                collectionPreferences.edit().putBoolean("hide-watched:$slug", hidden).apply()
            })
        }
    }

    override fun onStart() {
        super.onStart()
        lifecycleScope.launch { repository.account.sync() }
    }

    override fun onDestroy() {
        connectivity.unregisterNetworkCallback(networkCallback)
        super.onDestroy()
        repository.close()
    }
}
