package com.openquizgo.guard.ui

import android.graphics.BitmapFactory
import androidx.compose.foundation.Image
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Text
import com.openquizgo.guard.BuildConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.URL

/** 把服务端媒体路径解析为 TV 可访问的绝对地址。 */
internal fun resolveMediaUrl(path: String): String {
    return if (path.startsWith("http")) path else BuildConfig.API_BASE_URL.trimEnd('/') + path
}

/** 轻量远程图片组件：避免为 TV 端引入额外图片库。 */
@Composable
internal fun RemoteImage(
    url: String,
    maxHeight: Dp = 240.dp,
    modifier: Modifier = Modifier.fillMaxWidth(),
    contentDescription: String = "图片",
) {
    var bitmap by remember(url) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    var failed by remember(url) { mutableStateOf(false) }
    LaunchedEffect(url) {
        val loaded = withContext(Dispatchers.IO) {
            runCatching {
                URL(url).openStream().use { BitmapFactory.decodeStream(it)?.asImageBitmap() }
            }.getOrNull()
        }
        if (loaded == null) failed = true else bitmap = loaded
    }
    val image = bitmap
    if (image != null) {
        Image(
            bitmap = image,
            contentDescription = contentDescription,
            modifier = modifier.heightIn(max = maxHeight),
            contentScale = ContentScale.Fit,
        )
    } else if (failed) {
        Text("图片加载失败", fontSize = 20.sp, color = Color(0xFFFFC107))
    } else {
        Text("图片加载中…", fontSize = 20.sp, color = Color(0xFFB0B0B0))
    }
}
