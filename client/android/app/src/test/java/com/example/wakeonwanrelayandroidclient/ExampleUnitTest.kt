package com.example.wakeonwanrelayandroidclient

import org.junit.Test

import org.junit.Assert.*

/**
 * Example local unit test, which will execute on the development machine (host).
 *
 * See [testing documentation](http://d.android.com/tools/testing).
 */
class ExampleUnitTest {
    @Test
    fun testStandardEd25519SignAndVerify() {
        // Standard Ed25519 PKCS#8 seed prefix (16 bytes)
        val pkcs8Prefix = byteArrayOf(
            0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06,
            0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04, 0x20
        )
        // Standard Ed25519 X.509 public key prefix (12 bytes)
        val x509Prefix = byteArrayOf(
            0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65,
            0x70, 0x03, 0x21, 0x00
        )

        // Generate a standard KeyPair to test
        val kpg = java.security.KeyPairGenerator.getInstance("Ed25519")
        val kp = kpg.generateKeyPair()

        val message = "v2:GET:/wake:1710000000:aabbccddeeff00112233445566778899".toByteArray(Charsets.UTF_8)

        val signer = java.security.Signature.getInstance("Ed25519")
        signer.initSign(kp.private)
        signer.update(message)
        val signature = signer.sign()

        assertEquals(64, signature.size)

        val verifier = java.security.Signature.getInstance("Ed25519")
        verifier.initVerify(kp.public)
        verifier.update(message)
        assertTrue(verifier.verify(signature))

        println("Standard Ed25519 signing and verification passed! Signature length: " + signature.size)
    }

    @Test
    fun testRawSeedSigning() {
        val pkcs8Prefix = byteArrayOf(
            0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06,
            0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04, 0x20
        )
        // 32-byte test seed
        val seed = ByteArray(32) { it.toByte() }
        val pkcs8Bytes = pkcs8Prefix + seed
        val keySpec = java.security.spec.PKCS8EncodedKeySpec(pkcs8Bytes)
        val kf = java.security.KeyFactory.getInstance("Ed25519")
        val privKey = kf.generatePrivate(keySpec)

        val signer = java.security.Signature.getInstance("Ed25519")
        signer.initSign(privKey)
        signer.update("test payload".toByteArray(Charsets.UTF_8))
        val sig = signer.sign()

        assertEquals(64, sig.size)
        println("Raw seed PKCS8 conversion and signing successful!")
    }

    @Test
    fun testGoKeyCompatibility() {
        val pubHex = "9ebbf4d24af0c60f661a657d527f6debd668eb2bc83a4cace0cf4d8f3cb21fb3"
        val privHex = "8b0739886dcf8c1b92a4a15d3f6488a087cee9b884cdc8a7c208712f260544ea9ebbf4d24af0c60f661a657d527f6debd668eb2bc83a4cace0cf4d8f3cb21fb3"

        val privBytes = WakeRelayClient.hexToBytes(privHex)
        assertEquals(64, privBytes.size)

        val seed32 = privBytes.copyOfRange(0, 32)
        val pkcs8Prefix = byteArrayOf(
            0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06,
            0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04, 0x20
        )
        val privKeySpec = java.security.spec.PKCS8EncodedKeySpec(pkcs8Prefix + seed32)
        val kf = java.security.KeyFactory.getInstance("Ed25519")
        val privateKey = kf.generatePrivate(privKeySpec)

        val x509Prefix = byteArrayOf(
            0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65,
            0x70, 0x03, 0x21, 0x00
        )
        val pubBytes = WakeRelayClient.hexToBytes(pubHex)
        val pubKeySpec = java.security.spec.X509EncodedKeySpec(x509Prefix + pubBytes)
        val publicKey = kf.generatePublic(pubKeySpec)

        val payload = "v2:GET:/wake:1710000000:aabbccddeeff00112233445566778899".toByteArray(Charsets.UTF_8)

        val signer = java.security.Signature.getInstance("Ed25519")
        signer.initSign(privateKey)
        signer.update(payload)
        val sig = signer.sign()

        val verifier = java.security.Signature.getInstance("Ed25519")
        verifier.initVerify(publicKey)
        verifier.update(payload)
        assertTrue(verifier.verify(sig))
    }

    @Test
    fun testHexConversion() {
        val testBytes = byteArrayOf(0x00, 0x01, 0x0a, 0x0f, 0x10, 0xff.toByte())
        val hex = WakeRelayClient.bytesToHex(testBytes)
        assertEquals("00010a0f10ff", hex)
        val back = WakeRelayClient.hexToBytes(hex)
        assertArrayEquals(testBytes, back)
    }
}