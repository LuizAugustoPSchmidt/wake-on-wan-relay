package com.example.wakeonwanrelayandroidclient

import android.content.Context
import android.content.SharedPreferences

data class RelayConfig(
    val serverUrl: String = "",
    val privateKey: String = "",
    val serverName: String = "",
    val certFingerprint: String = ""
)

object ConfigStore {
    private const val PREFS_NAME = "wake_on_wan_prefs"
    private const val KEY_SERVER_URL = "server_url"
    private const val KEY_PRIVATE_KEY = "private_key"
    private const val KEY_SERVER_NAME = "server_name"
    private const val KEY_CERT_FINGERPRINT = "cert_fingerprint"

    fun loadConfig(context: Context): RelayConfig {
        val prefs: SharedPreferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        return RelayConfig(
            serverUrl = prefs.getString(KEY_SERVER_URL, "") ?: "",
            privateKey = prefs.getString(KEY_PRIVATE_KEY, "") ?: "",
            serverName = prefs.getString(KEY_SERVER_NAME, "") ?: "",
            certFingerprint = prefs.getString(KEY_CERT_FINGERPRINT, "") ?: ""
        )
    }

    fun saveConfig(context: Context, config: RelayConfig) {
        val prefs: SharedPreferences = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)
        prefs.edit()
            .putString(KEY_SERVER_URL, config.serverUrl.trim())
            .putString(KEY_PRIVATE_KEY, config.privateKey.trim())
            .putString(KEY_SERVER_NAME, config.serverName.trim())
            .putString(KEY_CERT_FINGERPRINT, config.certFingerprint.trim())
            .apply()
    }
}
