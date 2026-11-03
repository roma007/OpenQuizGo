package com.openquizgo.guard.ui

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.ViewModelProvider
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import androidx.tv.material3.darkColorScheme
import com.openquizgo.guard.GuardApplication

class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = (application as GuardApplication).container
        val viewModel = ViewModelProvider(
            this,
            GuardViewModel.Factory(container),
        )[GuardViewModel::class.java]

        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    GuardScreen(viewModel)
                }
            }
        }
    }
}

@Composable
private fun GuardScreen(viewModel: GuardViewModel) {
    val state by viewModel.state.collectAsState()
    when (val current = state) {
        is GuardUiState.Loading -> CenterMessage("正在连接服务端…")
        is GuardUiState.Activation -> ActivationScreen(
            state = current,
            onRegenerate = viewModel::regenerate,
        )
        is GuardUiState.Bound -> BoundScreen(
            childName = current.childName,
            onReset = viewModel::resetBinding,
        )
        is GuardUiState.Failure -> FailureScreen(
            message = current.message,
            onRetry = viewModel::regenerate,
        )
    }
}

@Composable
private fun CenterMessage(message: String) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = message, fontSize = 28.sp, color = Color(0xFFE0E0E0))
    }
}

@Composable
private fun ActivationScreen(
    state: GuardUiState.Activation,
    onRegenerate: () -> Unit,
) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "设备激活",
            fontSize = 32.sp,
            color = Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(16.dp))
        Text(
            text = state.code,
            fontSize = 96.sp,
            fontWeight = FontWeight.Bold,
            letterSpacing = 12.sp,
            color = Color.White,
        )
        Spacer(Modifier.height(24.dp))
        Text(
            text = if (state.expired) "激活码已过期" else "请在家长端输入此码，完成后电视会自动绑定",
            fontSize = 22.sp,
            color = if (state.expired) Color(0xFFFF7A7A) else Color(0xFFB0B0B0),
        )
        Spacer(Modifier.height(32.dp))
        if (state.expired) {
            Button(onClick = onRegenerate) {
                Text("重新生成", fontSize = 22.sp)
            }
        } else {
            Text(text = "等待家长确认…", fontSize = 20.sp, color = Color(0xFF7CFF9B))
        }
        Spacer(Modifier.height(40.dp))
        Text(
            text = "设备 ID：${state.deviceId.take(8)}…",
            fontSize = 14.sp,
            color = Color(0xFF606060),
        )
    }
}

@Composable
private fun BoundScreen(childName: String, onReset: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = "设备已绑定", fontSize = 32.sp, color = Color(0xFF7CFF9B))
        Spacer(Modifier.height(16.dp))
        Text(text = "孩子：$childName", fontSize = 40.sp, color = Color.White)
        Spacer(Modifier.height(40.dp))
        Button(onClick = onReset) {
            Text("重置绑定", fontSize = 22.sp)
        }
    }
}

@Composable
private fun FailureScreen(message: String, onRetry: () -> Unit) {
    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(text = "激活失败", fontSize = 32.sp, color = Color(0xFFFF7A7A))
        Spacer(Modifier.height(16.dp))
        Text(text = message, fontSize = 20.sp, color = Color(0xFFB0B0B0))
        Spacer(Modifier.height(32.dp))
        Button(onClick = onRetry) {
            Text("重试", fontSize = 22.sp)
        }
    }
}
