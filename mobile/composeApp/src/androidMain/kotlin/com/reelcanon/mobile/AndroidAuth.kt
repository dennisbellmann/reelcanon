package com.reelcanon.mobile

import android.app.Activity
import android.content.Context
import android.content.MutableContextWrapper
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.AtomicFile
import androidx.credentials.*
import androidx.credentials.exceptions.GetCredentialCancellationException
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.serialization.encodeToString
import java.io.File
import java.security.KeyStore
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import okio.ByteString.Companion.encodeUtf8

class AndroidGoogleAuthenticator(activity: Activity) : GoogleAuthenticator {
    private val context = MutableContextWrapper(activity)
    private val manager = CredentialManager.create(activity.applicationContext)
    override suspend fun idToken(serverClientId: String, nonce: String): String {
        val request = GetCredentialRequest.Builder().addCredentialOption(
            GetSignInWithGoogleOption.Builder(serverClientId).setNonce(nonce).build()
        ).build()
        try {
            val credential = manager.getCredential(context, request).credential
            require(credential is CustomCredential && credential.type == GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL)
            return GoogleIdTokenCredential.createFrom(credential.data).idToken
        } catch (_: GetCredentialCancellationException) { throw SignInCancelled() }
    }
    override suspend fun clear() { manager.clearCredentialState(ClearCredentialStateRequest()) }
}

/** One atomically replaced ciphertext file, excluded from backups. No plaintext token preferences. */
class AndroidSessionStore(context: Context, apiBase: String) : SecureSessionStore {
    private val namespace = apiBase.encodeUtf8().sha256().hex()
    private val file = AtomicFile(File(context.noBackupFilesDir, "account-$namespace.bin"))
    private val alias = "reel-canon-account-$namespace"
    private fun key(): SecretKey {
        val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
        (store.getKey(alias, null) as? SecretKey)?.let { return it }
        return KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore").apply {
            init(KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
                .setKeySize(256).setBlockModes(KeyProperties.BLOCK_MODE_GCM)
                .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE).build())
        }.generateKey()
    }
    override suspend fun read(): SessionEnvelope = withContext(Dispatchers.IO) {
        if (!file.baseFile.exists()) return@withContext SessionEnvelope()
        try {
            val bytes = file.readFully()
            require(bytes.size > 28)
            val cipher = Cipher.getInstance("AES/GCM/NoPadding")
            cipher.init(Cipher.DECRYPT_MODE, key(), GCMParameterSpec(128, bytes.copyOfRange(0, 12)))
            cipher.updateAAD(namespace.toByteArray())
            catalogJson.decodeFromString<SessionEnvelope>(cipher.doFinal(bytes.copyOfRange(12, bytes.size)).toString(Charsets.UTF_8))
        } catch (_: Exception) {
            // A missing/invalidated Keystore key requires reauthentication, never plaintext fallback.
            file.delete()
            SessionEnvelope()
        }
    }
    override suspend fun write(value: SessionEnvelope) = withContext(Dispatchers.IO) {
        val cipher = Cipher.getInstance("AES/GCM/NoPadding")
        cipher.init(Cipher.ENCRYPT_MODE, key())
        cipher.updateAAD(namespace.toByteArray())
        val bytes = cipher.iv + cipher.doFinal(catalogJson.encodeToString(value).toByteArray())
        val output = file.startWrite()
        try { output.write(bytes); file.finishWrite(output) }
        catch (error: Exception) { file.failWrite(output); throw error }
    }
}
