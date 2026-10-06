package com.example.wakeonwanrelayandroidclient

import java.io.ByteArrayInputStream
import java.io.File
import java.security.MessageDigest
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

object KeyFileUtils {

    private fun decodeBase64(str: String): ByteArray {
        val clean = str.replace("\r", "").replace("\n", "").replace(" ", "").trim()
        return try {
            java.util.Base64.getDecoder().decode(clean)
        } catch (_: Exception) {
            android.util.Base64.decode(clean, android.util.Base64.DEFAULT)
        }
    }

    /**
     * Resolves and extracts an Ed25519 private key from a file path or raw string.
     */
    fun loadPrivateKey(inputOrPath: String): String {
        val trimmed = inputOrPath.trim()
        if (trimmed.isEmpty()) return ""

        val isPath = trimmed.startsWith("/") || trimmed.startsWith("file:") || trimmed.contains(File.separator)
        val file = File(trimmed)

        if (isPath) {
            if (!file.exists()) {
                throw java.io.FileNotFoundException("Private key file does not exist: $trimmed")
            }
            if (!file.canRead()) {
                throw java.io.IOException("Permission denied reading private key file at: $trimmed. Please grant file access.")
            }
            return extractPrivateKey(file.readBytes())
        }

        val bytes = if (file.exists() && file.isFile && file.canRead()) {
            file.readBytes()
        } else {
            trimmed.toByteArray(Charsets.UTF_8)
        }
        return extractPrivateKey(bytes)
    }

    /**
     * Resolves and extracts the SHA-256 certificate fingerprint from a file path or raw string.
     */
    fun loadCertificateFingerprint(inputOrPath: String): String {
        val trimmed = inputOrPath.trim()
        if (trimmed.isEmpty()) return ""

        val isPath = trimmed.startsWith("/") || trimmed.startsWith("file:") || trimmed.contains(File.separator)
        val file = File(trimmed)

        if (isPath) {
            if (!file.exists()) {
                throw java.io.FileNotFoundException("Fingerprint file does not exist: $trimmed")
            }
            if (!file.canRead()) {
                throw java.io.IOException("Permission denied reading fingerprint file at: $trimmed. Please grant file access.")
            }
            return extractCertificateFingerprint(file.readBytes())
        }

        val bytes = if (file.exists() && file.isFile && file.canRead()) {
            file.readBytes()
        } else {
            trimmed.toByteArray(Charsets.UTF_8)
        }
        return extractCertificateFingerprint(bytes)
    }

    /**
     * Extracts an Ed25519 private key hex string from file bytes.
     * Supports:
     * - OpenSSH private key files (~/.ssh/id_ed25519)
     * - PKCS#8 PEM private key files (-----BEGIN PRIVATE KEY-----)
     * - Plaintext hex files containing 128 or 64 hex characters
     * - Raw binary 32-byte or 64-byte key files
     * - Console output ("Private: <hex>")
     */
    fun extractPrivateKey(bytes: ByteArray): String {
        // 1. Raw binary keys (32 bytes seed or 64 bytes ed25519 privkey)
        if (bytes.size == 32 || bytes.size == 64) {
            val isHexAscii = bytes.all {
                (it in '0'.code.toByte()..'9'.code.toByte()) ||
                (it in 'a'.code.toByte()..'f'.code.toByte()) ||
                (it in 'A'.code.toByte()..'F'.code.toByte())
            }
            if (!isHexAscii) {
                return WakeRelayClient.bytesToHex(bytes)
            }
        }

        val text = String(bytes, Charsets.UTF_8).trim()

        // 2. OpenSSH private key format (common in ~/.ssh/id_ed25519)
        if (text.contains("BEGIN OPENSSH PRIVATE KEY")) {
            val openSshKey = parseOpenSshEd25519(text)
            if (openSshKey != null) {
                return WakeRelayClient.bytesToHex(openSshKey)
            }
        }

        // 3. PKCS#8 PEM private key format (-----BEGIN PRIVATE KEY-----)
        if (text.contains("BEGIN PRIVATE KEY")) {
            val pkcs8Key = parsePkcs8Pem(text)
            if (pkcs8Key != null) {
                return WakeRelayClient.bytesToHex(pkcs8Key)
            }
        }

        // 4. keypair-generator console output ("Private: <hex>")
        val privPrefixRegex = Regex("""Private:\s*([0-9a-fA-F]+)""", RegexOption.IGNORE_CASE)
        privPrefixRegex.find(text)?.groupValues?.get(1)?.let {
            val clean = it.trim()
            if (clean.length == 64 || clean.length == 128) return clean
        }

        // 5. JSON format (if present)
        if (text.contains("privateKey")) {
            val regex = Regex(""""privateKey"\s*:\s*"([^"]+)"""")
            regex.find(text)?.groupValues?.get(1)?.let {
                val clean = it.trim()
                if (clean.isNotEmpty()) return clean
            }
        }

        // 6. Raw hex string in text (128 chars for full Go Ed25519 key, 64 chars for seed)
        val stripped = text.replace("\r", "").replace("\n", "").replace(" ", "").trim()
        val match128 = Regex("[0-9a-fA-F]{128}").find(stripped)
        if (match128 != null) {
            return match128.value
        }
        val match64 = Regex("[0-9a-fA-F]{64}").find(stripped)
        if (match64 != null) {
            return match64.value
        }

        throw IllegalArgumentException("Could not find a valid Ed25519 private key in the file (supports OpenSSH, PEM, and hex files).")
    }

    private fun parseOpenSshEd25519(text: String): ByteArray? {
        val clean = text
            .replace("-----BEGIN OPENSSH PRIVATE KEY-----", "")
            .replace("-----END OPENSSH PRIVATE KEY-----", "")
            .trim()
        val decoded = try {
            decodeBase64(clean)
        } catch (_: Exception) {
            return null
        }

        val magic = "openssh-key-v1\u0000".toByteArray(Charsets.US_ASCII)
        if (decoded.size < magic.size || !decoded.take(magic.size).toByteArray().contentEquals(magic)) {
            return null
        }

        // Locate "ssh-ed25519" algorithm marker
        val marker = "ssh-ed25519".toByteArray(Charsets.US_ASCII)
        var lastIdx = -1
        for (i in 0 until decoded.size - marker.size) {
            if (decoded.copyOfRange(i, i + marker.size).contentEquals(marker)) {
                lastIdx = i
            }
        }
        if (lastIdx == -1) return null

        var offset = lastIdx + marker.size
        // In the private key record:
        // [4 bytes pubKey len] + [pubKey] + [4 bytes privKey len] + [64 bytes privKey (32 seed + 32 pub)]
        if (offset + 4 <= decoded.size) {
            val pubLen = ((decoded[offset].toInt() and 0xFF) shl 24) or
                         ((decoded[offset + 1].toInt() and 0xFF) shl 16) or
                         ((decoded[offset + 2].toInt() and 0xFF) shl 8) or
                         (decoded[offset + 3].toInt() and 0xFF)
            offset += 4 + pubLen
            if (offset + 4 + 64 <= decoded.size) {
                val privLen = ((decoded[offset].toInt() and 0xFF) shl 24) or
                              ((decoded[offset + 1].toInt() and 0xFF) shl 16) or
                              ((decoded[offset + 2].toInt() and 0xFF) shl 8) or
                              (decoded[offset + 3].toInt() and 0xFF)
                if (privLen == 64) {
                    return decoded.copyOfRange(offset + 4, offset + 4 + 64)
                }
            }
        }
        return null
    }

    private fun parsePkcs8Pem(text: String): ByteArray? {
        val clean = text
            .replace("-----BEGIN PRIVATE KEY-----", "")
            .replace("-----END PRIVATE KEY-----", "")
            .trim()
        val decoded = try {
            decodeBase64(clean)
        } catch (_: Exception) {
            return null
        }

        // Standard Ed25519 PKCS#8 prefix is 16 bytes: 30 2e 02 01 00 30 05 06 03 2b 65 70 04 22 04 20
        // Followed by 32 bytes seed
        if (decoded.size == 48 && decoded[decoded.size - 34] == 0x04.toByte() && decoded[decoded.size - 33] == 0x20.toByte()) {
            return decoded.copyOfRange(decoded.size - 32, decoded.size)
        }
        return null
    }

    /**
     * Extracts or calculates the SHA-256 certificate fingerprint from file bytes.
     * Supports:
     * - Dedicated fingerprint files (containing just the 64 hex characters or colons)
     * - PEM certificate file (cert.pem)
     * - DER certificate file (.crt, .cer, .der)
     * - JSON files containing "certFingerprint": "..."
     */
    fun extractCertificateFingerprint(bytes: ByteArray): String {
        // 1. Try parsing as dedicated single-line / raw fingerprint file
        val text = String(bytes, Charsets.UTF_8).trim()
        val cleanCandidate = text.replace(":", "").replace(" ", "").replace("-", "").trim().lowercase()
        if (cleanCandidate.length == 64 && cleanCandidate.all { it in '0'..'9' || it in 'a'..'f' }) {
            return cleanCandidate
        }

        // 2. Try parsing as X.509 certificate (PEM or DER format)
        try {
            val cf = CertificateFactory.getInstance("X.509")
            val cert = cf.generateCertificate(ByteArrayInputStream(bytes)) as? X509Certificate
            if (cert != null) {
                val md = MessageDigest.getInstance("SHA-256")
                val digest = md.digest(cert.encoded)
                return WakeRelayClient.bytesToHex(digest).lowercase()
            }
        } catch (_: Exception) {
            // Not a direct X.509 certificate stream
        }

        // 3. JSON format (e.g. config.json)
        if (text.contains("certFingerprint")) {
            val regex = Regex(""""certFingerprint"\s*:\s*"([^"]+)"""")
            regex.find(text)?.groupValues?.get(1)?.let {
                val clean = it.replace(":", "").replace(" ", "").trim().lowercase()
                if (clean.length == 64) return clean
            }
        }

        // 4. Hex substring search
        val hexMatch = Regex("[0-9a-f]{64}").find(cleanCandidate)
        if (hexMatch != null) {
            return hexMatch.value
        }

        throw IllegalArgumentException("Could not extract a valid SHA-256 fingerprint from the file.")
    }
}
