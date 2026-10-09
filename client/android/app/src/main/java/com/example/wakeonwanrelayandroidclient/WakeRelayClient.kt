package com.example.wakeonwanrelayandroidclient

import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.security.KeyFactory
import java.security.MessageDigest
import java.security.SecureRandom
import java.security.Signature
import java.security.cert.CertificateException
import java.security.cert.X509Certificate
import java.security.spec.PKCS8EncodedKeySpec
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager

sealed class WakeResult {
    data class Success(val statusCode: Int, val message: String) : WakeResult()
    data class Failure(val statusCode: Int?, val errorMessage: String) : WakeResult()
}

object WakeRelayClient {

    private val PKCS8_ED25519_PREFIX = byteArrayOf(
        0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06,
        0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04, 0x20
    )

    fun hexToBytes(hex: String): ByteArray {
        val clean = hex.trim().replace(":", "").replace(" ", "")
        require(clean.length % 2 == 0) { "Hex string must have an even length" }
        val result = ByteArray(clean.length / 2)
        for (i in result.indices) {
            result[i] = clean.substring(i * 2, i * 2 + 2).toInt(16).toByte()
        }
        return result
    }

    fun bytesToHex(bytes: ByteArray): String {
        val sb = StringBuilder(bytes.size * 2)
        for (b in bytes) {
            sb.append(String.format("%02x", b))
        }
        return sb.toString()
    }

    private fun getPrivateKey(privKeyHex: String): java.security.PrivateKey {
        val rawBytes = hexToBytes(privKeyHex)
        val seed = when (rawBytes.size) {
            32 -> rawBytes
            64 -> rawBytes.copyOfRange(0, 32) // First 32 bytes is seed in Go Ed25519 format
            else -> throw IllegalArgumentException("Expected 32-byte seed or 64-byte Ed25519 private key (got ${rawBytes.size} bytes)")
        }

        val pkcs8Bytes = PKCS8_ED25519_PREFIX + seed
        val spec = PKCS8EncodedKeySpec(pkcs8Bytes)
        val kf = KeyFactory.getInstance("Ed25519")
        return kf.generatePrivate(spec)
    }

    private fun signPayload(privateKey: java.security.PrivateKey, payload: ByteArray): String {
        val signer = Signature.getInstance("Ed25519")
        signer.initSign(privateKey)
        signer.update(payload)
        return bytesToHex(signer.sign())
    }

    fun resolveEndpointUrl(serverUrlString: String, endpoint: String): URL {
        var trimmed = serverUrlString.trim().trimEnd('/')
        if (!trimmed.startsWith("http://", ignoreCase = true) && !trimmed.startsWith("https://", ignoreCase = true)) {
            trimmed = "https://$trimmed"
        }
        val targetEndpoint = if (endpoint.startsWith("/")) endpoint else "/$endpoint"

        val base = when {
            trimmed.endsWith("/wake", ignoreCase = true) -> trimmed.substring(0, trimmed.length - 5)
            trimmed.endsWith("/status", ignoreCase = true) -> trimmed.substring(0, trimmed.length - 7)
            else -> trimmed
        }

        return URL(base + targetEndpoint)
    }

    fun sendWakeRequest(
        serverUrlString: String,
        privateKeyHex: String,
        serverName: String = "",
        certFingerprint: String = ""
    ): WakeResult = sendRequest(
        serverUrlString = serverUrlString,
        endpoint = "/wake",
        privateKeyHex = privateKeyHex,
        serverName = serverName,
        certFingerprint = certFingerprint
    )

    fun sendStatusRequest(
        serverUrlString: String,
        privateKeyHex: String,
        serverName: String = "",
        certFingerprint: String = ""
    ): WakeResult = sendRequest(
        serverUrlString = serverUrlString,
        endpoint = "/status",
        privateKeyHex = privateKeyHex,
        serverName = serverName,
        certFingerprint = certFingerprint
    )

    fun sendRequest(
        serverUrlString: String,
        endpoint: String,
        privateKeyHex: String,
        serverName: String = "",
        certFingerprint: String = ""
    ): WakeResult {
        return try {
            val resolvedKeyHex = KeyFileUtils.loadPrivateKey(privateKeyHex)
            val resolvedFp = KeyFileUtils.loadCertificateFingerprint(certFingerprint)
            val privateKey = getPrivateKey(resolvedKeyHex)
            val parsedUrl = resolveEndpointUrl(serverUrlString, endpoint)

            // 1. Attempt v2 request
            val v2Result = executeRequest(
                parsedUrl = parsedUrl,
                privateKey = privateKey,
                isV2 = true,
                serverName = serverName,
                certFingerprint = resolvedFp
            )

            // 2. If server returns 404 (possibly legacy v1 server), fallback to v1 auth
            if (v2Result is WakeResult.Failure && v2Result.statusCode == 404) {
                val v1Result = executeRequest(
                    parsedUrl = parsedUrl,
                    privateKey = privateKey,
                    isV2 = false,
                    serverName = serverName,
                    certFingerprint = resolvedFp
                )
                v1Result
            } else {
                v2Result
            }
        } catch (e: Exception) {
            WakeResult.Failure(statusCode = null, errorMessage = e.message ?: e.toString())
        }
    }

    private fun executeRequest(
        parsedUrl: URL,
        privateKey: java.security.PrivateKey,
        isV2: Boolean,
        serverName: String,
        certFingerprint: String
    ): WakeResult {
        val connection = (parsedUrl.openConnection() as? HttpURLConnection)
            ?: return WakeResult.Failure(null, "Not an HTTP/HTTPS URL")

        try {
            connection.requestMethod = "GET"
            connection.connectTimeout = 8000
            connection.readTimeout = 8000

            // Configure TLS if HTTPS
            if (connection is HttpsURLConnection) {
                configureTLS(connection, serverName, certFingerprint)
            }

            val timestamp = (System.currentTimeMillis() / 1000).toString()
            connection.setRequestProperty("X-Timestamp", timestamp)

            val path = if (parsedUrl.path.isNullOrEmpty()) "/wake" else parsedUrl.path

            if (isV2) {
                val nonceBytes = ByteArray(16)
                SecureRandom().nextBytes(nonceBytes)
                val nonce = bytesToHex(nonceBytes)
                connection.setRequestProperty("X-Nonce", nonce)

                val v2Payload = "v2:GET:$path:$timestamp:$nonce"
                val sig = signPayload(privateKey, v2Payload.toByteArray(Charsets.UTF_8))
                connection.setRequestProperty("X-Signature", sig)
            } else {
                val sig = signPayload(privateKey, timestamp.toByteArray(Charsets.UTF_8))
                connection.setRequestProperty("X-Signature", sig)
            }

            val statusCode = connection.responseCode
            val inputStream = if (statusCode in 200..299) {
                connection.inputStream
            } else {
                connection.errorStream
            }

            val responseBody = inputStream?.let { stream ->
                BufferedReader(InputStreamReader(stream)).use { it.readText() }
            }?.trim() ?: ""

            return if (statusCode in 200..299) {
                WakeResult.Success(statusCode, responseBody.ifBlank { "OK" })
            } else {
                WakeResult.Failure(statusCode, responseBody.ifBlank { "HTTP $statusCode" })
            }
        } catch (e: Exception) {
            return WakeResult.Failure(null, e.message ?: e.toString())
        } finally {
            connection.disconnect()
        }
    }

    private fun configureTLS(
        conn: HttpsURLConnection,
        serverName: String,
        certFingerprint: String
    ) {
        val cleanFp = certFingerprint.replace(":", "").replace(" ", "").lowercase().trim()

        if (cleanFp.isNotEmpty()) {
            if (cleanFp.length != 64 || !cleanFp.all { it in '0'..'9' || it in 'a'..'f' }) {
                throw IllegalArgumentException(
                    "Invalid certificate fingerprint: expected 64 hex characters (SHA-256), got: '$certFingerprint'"
                )
            }
            val trustManager = object : X509TrustManager {
                override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) {}

                override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) {
                    if (chain.isNullOrEmpty()) {
                        throw CertificateException("No certificates presented by server")
                    }
                    val leaf = chain[0]
                    val md = MessageDigest.getInstance("SHA-256")
                    val digest = md.digest(leaf.encoded)
                    val actualFp = bytesToHex(digest).lowercase()
                    if (actualFp != cleanFp) {
                        throw CertificateException(
                            "Certificate fingerprint mismatch:\nExpected: $cleanFp\nReceived: $actualFp"
                        )
                    }
                }

                override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
            }

            val sslContext = SSLContext.getInstance("TLS")
            sslContext.init(null, arrayOf(trustManager), SecureRandom())
            conn.sslSocketFactory = sslContext.socketFactory
        }

        if (serverName.isNotBlank() || cleanFp.isNotEmpty()) {
            conn.hostnameVerifier = HostnameVerifier { hostname, session ->
                if (serverName.isNotBlank()) {
                    hostname.equals(serverName, ignoreCase = true) ||
                            session.peerHost?.equals(serverName, ignoreCase = true) == true ||
                            HttpsURLConnection.getDefaultHostnameVerifier().verify(serverName, session)
                } else if (cleanFp.isNotEmpty()) {
                    // Fingerprint is pinned directly, accept host
                    true
                } else {
                    HttpsURLConnection.getDefaultHostnameVerifier().verify(hostname, session)
                }
            }
        }
    }
}
