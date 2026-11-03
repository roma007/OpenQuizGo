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
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.MaterialTheme
import androidx.tv.material3.darkColorScheme
import androidx.tv.material3.Surface
import androidx.tv.material3.Text
import com.openquizgo.guard.GuardApplication
import kotlinx.coroutines.launch

class MainActivity : ComponentActivity() {

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val container = (application as GuardApplication).container
        setContent {
            MaterialTheme(colorScheme = darkColorScheme()) {
                Surface(modifier = Modifier.fillMaxSize()) {
                    GuardHome(
                        tokenFlow = container.preferences.deviceToken,
                        checkServer = {
                            container.api.meta()
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun GuardHome(
    tokenFlow: kotlinx.coroutines.flow.Flow<String?>,
    checkServer: suspend () -> com.openquizgo.guard.data.remote.MetaResponse,
) {
    val token by tokenFlow.collectAsState(initial = null)
    val scope = rememberCoroutineScope()
    var status by remember { mutableStateOf("尚未检测服务端") }

    Column(
        modifier = Modifier.fillMaxSize().padding(64.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = "OpenQuizGo 守护端",
            fontSize = 52.sp,
            fontWeight = FontWeight.Bold,
            color = Color.White,
        )
        Spacer(Modifier.height(24.dp))
        Text(
            text = if (token.isNullOrBlank()) "设备未激活" else "已绑定设备",
            fontSize = 28.sp,
            color = if (token.isNullOrBlank()) Color(0xFFFFC107) else Color(0xFF7CFF9B),
        )
        Spacer(Modifier.height(40.dp))
        Button(onClick = {
            scope.launch {
                status = runCatching {
                    val meta = checkServer()
                    "服务端正常：${meta.service} ${meta.version}"
                }.getOrElse {
                    "连接失败：${it.message}"
                }
            }
        }) {
            Text("检测服务端", fontSize = 24.sp)
        }
        Spacer(Modifier.height(24.dp))
        Text(text = status, fontSize = 20.sp, color = Color(0xFFB0B0B0))
    }
}
