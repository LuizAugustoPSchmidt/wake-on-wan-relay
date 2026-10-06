package com.example.wakeonwanrelayandroidclient

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.example.wakeonwanrelayandroidclient.ui.theme.WakeOnWanRelayAndroidClientTheme
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            WakeOnWanRelayAndroidClientTheme {
                WakeScreen()
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun WakeScreen() {
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }

    var serverUrl by remember { mutableStateOf("") }
    var privateKey by remember { mutableStateOf("") }
    var serverName by remember { mutableStateOf("") }
    var certFingerprint by remember { mutableStateOf("") }
    var showPrivateKey by remember { mutableStateOf(false) }
    var showAdvanced by remember { mutableStateOf(false) }
    var activeFileBrowser by remember { mutableStateOf<String?>(null) }

    var isLoading by remember { mutableStateOf(false) }
    var lastResult by remember { mutableStateOf<WakeResult?>(null) }
    var lastResultTime by remember { mutableStateOf("") }

    val privateKeyPickerLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.OpenDocument()
    ) { uri ->
        uri?.let {
            try {
                val bytes = context.contentResolver.openInputStream(it)?.use { stream ->
                    stream.readBytes()
                } ?: throw java.io.IOException("Could not read file data")
                val extracted = KeyFileUtils.extractPrivateKey(bytes)
                privateKey = extracted
                scope.launch {
                    snackbarHostState.showSnackbar("Private key loaded from file!")
                }
            } catch (e: Exception) {
                scope.launch {
                    snackbarHostState.showSnackbar("Failed to load key: ${e.message}")
                }
            }
        }
    }

    val certPickerLauncher = rememberLauncherForActivityResult(
        contract = ActivityResultContracts.OpenDocument()
    ) { uri ->
        uri?.let {
            try {
                val bytes = context.contentResolver.openInputStream(it)?.use { stream ->
                    stream.readBytes()
                } ?: throw java.io.IOException("Could not read file data")
                val extracted = KeyFileUtils.extractCertificateFingerprint(bytes)
                certFingerprint = extracted
                scope.launch {
                    snackbarHostState.showSnackbar("Certificate fingerprint loaded from file!")
                }
            } catch (e: Exception) {
                scope.launch {
                    snackbarHostState.showSnackbar("Failed to load cert: ${e.message}")
                }
            }
        }
    }

    // Load persisted configuration on launch
    LaunchedEffect(Unit) {
        val config = ConfigStore.loadConfig(context)
        serverUrl = config.serverUrl
        privateKey = config.privateKey
        serverName = config.serverName
        certFingerprint = config.certFingerprint
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        text = "Wake On WAN",
                        fontWeight = FontWeight.Bold
                    )
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.primaryContainer,
                    titleContentColor = MaterialTheme.colorScheme.onPrimaryContainer
                )
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) },
        modifier = Modifier.fillMaxSize()
    ) { innerPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(innerPadding)
                .padding(horizontal = 16.dp)
                .verticalScroll(rememberScrollState()),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Spacer(modifier = Modifier.height(4.dp))

            // Action Card
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surfaceVariant
                ),
                elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(20.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(16.dp)
                ) {
                    Text(
                        text = "Remote Target Wake",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold
                    )

                    Button(
                        onClick = {
                            if (serverUrl.isBlank() || privateKey.isBlank()) {
                                scope.launch {
                                    snackbarHostState.showSnackbar("Please fill Server URL and Private Key first")
                                }
                                return@Button
                            }

                            isLoading = true
                            lastResult = null
                            scope.launch {
                                val result = withContext(Dispatchers.IO) {
                                    WakeRelayClient.sendWakeRequest(
                                        serverUrlString = serverUrl.trim(),
                                        privateKeyHex = privateKey.trim(),
                                        serverName = serverName.trim(),
                                        certFingerprint = certFingerprint.trim()
                                    )
                                }
                                isLoading = false
                                lastResult = result
                                val formatter = SimpleDateFormat("HH:mm:ss", Locale.getDefault())
                                lastResultTime = formatter.format(Date())
                            }
                        },
                        enabled = !isLoading,
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(56.dp),
                        shape = RoundedCornerShape(12.dp)
                    ) {
                        if (isLoading) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(24.dp),
                                color = MaterialTheme.colorScheme.onPrimary,
                                strokeWidth = 2.dp
                            )
                            Spacer(modifier = Modifier.width(12.dp))
                            Text(text = "Sending Signal...", fontSize = 16.sp)
                        } else {
                            Text(
                                text = "SEND WAKE SIGNAL",
                                fontSize = 16.sp,
                                fontWeight = FontWeight.Bold
                            )
                        }
                    }

                    // Result Status Display
                    lastResult?.let { result ->
                        val isSuccess = result is WakeResult.Success
                        val bgColor = if (isSuccess) {
                            MaterialTheme.colorScheme.primaryContainer
                        } else {
                            MaterialTheme.colorScheme.errorContainer
                        }
                        val textColor = if (isSuccess) {
                            MaterialTheme.colorScheme.onPrimaryContainer
                        } else {
                            MaterialTheme.colorScheme.onErrorContainer
                        }

                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(8.dp))
                                .background(bgColor)
                                .padding(12.dp)
                        ) {
                            Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
                                Row(
                                    modifier = Modifier.fillMaxWidth(),
                                    horizontalArrangement = Arrangement.SpaceBetween
                                ) {
                                    Text(
                                        text = if (isSuccess) "Success (HTTP ${(result as WakeResult.Success).statusCode})" else "Request Failed",
                                        fontWeight = FontWeight.Bold,
                                        color = textColor,
                                        style = MaterialTheme.typography.bodyMedium
                                    )
                                    if (lastResultTime.isNotEmpty()) {
                                        Text(
                                            text = lastResultTime,
                                            style = MaterialTheme.typography.labelSmall,
                                            color = textColor
                                        )
                                    }
                                }
                                val detailMessage = when (result) {
                                    is WakeResult.Success -> result.message
                                    is WakeResult.Failure -> {
                                        if (result.statusCode != null) {
                                            "HTTP ${result.statusCode}: ${result.errorMessage}"
                                        } else {
                                            result.errorMessage
                                        }
                                    }
                                }
                                Text(
                                    text = detailMessage,
                                    style = MaterialTheme.typography.bodySmall,
                                    color = textColor,
                                    fontFamily = FontFamily.Monospace
                                )
                            }
                        }
                    }
                }
            }

            // Configuration Card
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.surface
                ),
                elevation = CardDefaults.cardElevation(defaultElevation = 1.dp)
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Text(
                        text = "Relay Configuration",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold
                    )

                    OutlinedTextField(
                        value = serverUrl,
                        onValueChange = { serverUrl = it },
                        label = { Text("Server URL") },
                        placeholder = { Text("https://example.com:8443/wake") },
                        singleLine = true,
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Uri),
                        modifier = Modifier.fillMaxWidth()
                    )

                    OutlinedTextField(
                        value = privateKey,
                        onValueChange = { privateKey = it },
                        label = { Text("Ed25519 Private Key File / Key") },
                        placeholder = { Text("Path to key file (e.g. ~/.ssh/id_ed25519)") },
                        singleLine = true,
                        visualTransformation = if (showPrivateKey) VisualTransformation.None else PasswordVisualTransformation(),
                        trailingIcon = {
                            TextButton(onClick = { showPrivateKey = !showPrivateKey }) {
                                Text(text = if (showPrivateKey) "Hide" else "Show", fontSize = 12.sp)
                            }
                        },
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Ascii),
                        modifier = Modifier.fillMaxWidth()
                    )

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "Select key file on device",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                        Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                            OutlinedButton(
                                onClick = { activeFileBrowser = "key" }
                            ) {
                                Text("Browse", fontSize = 12.sp)
                            }
                            TextButton(
                                onClick = { privateKeyPickerLauncher.launch(arrayOf("*/*")) }
                            ) {
                                Text("System", fontSize = 12.sp)
                            }
                        }
                    }

                    // Advanced TLS Options Toggle
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "Advanced TLS Options",
                            style = MaterialTheme.typography.bodyMedium,
                            fontWeight = FontWeight.Medium
                        )
                        TextButton(onClick = { showAdvanced = !showAdvanced }) {
                            Text(if (showAdvanced) "Hide" else "Show")
                        }
                    }

                    AnimatedVisibility(visible = showAdvanced) {
                        Column(
                            verticalArrangement = Arrangement.spacedBy(12.dp),
                            modifier = Modifier.fillMaxWidth()
                        ) {
                            OutlinedTextField(
                                value = serverName,
                                onValueChange = { serverName = it },
                                label = { Text("Server Name (SNI Override)") },
                                placeholder = { Text("e.g. relay.home.arpa") },
                                singleLine = true,
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Ascii),
                                modifier = Modifier.fillMaxWidth()
                            )

                            OutlinedTextField(
                                value = certFingerprint,
                                onValueChange = { certFingerprint = it },
                                label = { Text("Certificate Fingerprint File / Value") },
                                placeholder = { Text("Path to fingerprint file (or SHA-256 hex)") },
                                singleLine = true,
                                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Ascii),
                                modifier = Modifier.fillMaxWidth()
                            )

                            Row(
                                modifier = Modifier.fillMaxWidth(),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = "Select fingerprint file",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                                Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                                    OutlinedButton(
                                        onClick = { activeFileBrowser = "cert" }
                                    ) {
                                        Text("Browse", fontSize = 12.sp)
                                    }
                                    TextButton(
                                        onClick = { certPickerLauncher.launch(arrayOf("*/*")) }
                                    ) {
                                        Text("System", fontSize = 12.sp)
                                    }
                                }
                            }
                        }
                    }

                    Spacer(modifier = Modifier.height(4.dp))

                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.End
                    ) {
                        OutlinedButton(
                            onClick = {
                                ConfigStore.saveConfig(
                                    context,
                                    RelayConfig(
                                        serverUrl = serverUrl,
                                        privateKey = privateKey,
                                        serverName = serverName,
                                        certFingerprint = certFingerprint
                                    )
                                )
                                scope.launch {
                                    snackbarHostState.showSnackbar("Configuration saved!")
                                }
                            }
                        ) {
                            Text("Save Configuration")
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(16.dp))
        }
    }

    // In-app File Browser Dialog
    when (activeFileBrowser) {
        "key" -> {
            FileBrowserDialog(
                title = "Select Private Key File",
                onDismissRequest = { activeFileBrowser = null },
                onFileSelected = { selectedPath ->
                    privateKey = selectedPath
                    activeFileBrowser = null
                    scope.launch {
                        snackbarHostState.showSnackbar("Selected key file: ${selectedPath.substringAfterLast('/')}")
                    }
                }
            )
        }
        "cert" -> {
            FileBrowserDialog(
                title = "Select Fingerprint File",
                onDismissRequest = { activeFileBrowser = null },
                onFileSelected = { selectedPath ->
                    certFingerprint = selectedPath
                    activeFileBrowser = null
                    scope.launch {
                        snackbarHostState.showSnackbar("Selected fingerprint file: ${selectedPath.substringAfterLast('/')}")
                    }
                }
            )
        }
    }
}