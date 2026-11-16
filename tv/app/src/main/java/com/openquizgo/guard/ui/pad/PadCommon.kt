package com.openquizgo.guard.ui.pad

import android.graphics.BitmapFactory
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.openquizgo.guard.BuildConfig
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.URL

/**
 * 084：安卓平板（触摸）界面基础组件。
 *
 * 与电视界面刻意分开：字距、触摸目标、滚动方式都按手持设备来；
 * 电视界面保持 tv-material 不动，避免影响已上线的遥控器体验。
 */

internal val PadBackground = Color(0xFF0B0B0B)
internal val PadTextPrimary = Color(0xFFF2F2F2)
internal val PadTextSecondary = Color(0xFFB0B0B0)
internal val PadGood = Color(0xFF7CFF9B)
internal val PadWarn = Color(0xFFFFC107)
internal val PadBad = Color(0xFFFF7A7A)
internal val PadAccent = Color(0xFF7CC7FF)

@Composable
internal fun PadTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = darkColorScheme(
            primary = PadAccent,
            background = PadBackground,
            surface = Color(0xFF161616),
            onBackground = PadTextPrimary,
            onSurface = PadTextPrimary,
        ),
    ) {
        Surface(modifier = Modifier.fillMaxSize(), color = PadBackground) {
            content()
        }
    }
}

/** 单列、限宽、可滚动的平板页面容器。 */
@Composable
internal fun PadPage(
    horizontalAlignment: Alignment.Horizontal = Alignment.CenterHorizontally,
    content: @Composable ColumnScope.() -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 24.dp),
        contentAlignment = Alignment.TopCenter,
    ) {
        Column(
            modifier = Modifier
                .widthIn(max = 560.dp)
                .fillMaxWidth(),
            horizontalAlignment = horizontalAlignment,
            content = content,
        )
    }
}

@Composable
internal fun PadTitle(text: String, modifier: Modifier = Modifier) {
    Text(
        text = text,
        modifier = modifier,
        fontSize = 24.sp,
        fontWeight = FontWeight.Bold,
        color = PadTextPrimary,
        textAlign = TextAlign.Center,
    )
}

@Composable
internal fun PadSubtitle(text: String, color: Color = PadTextSecondary) {
    Text(
        text = text,
        fontSize = 14.sp,
        color = color,
        textAlign = TextAlign.Center,
    )
}

@Composable
internal fun PadPrimaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    Button(onClick = onClick, modifier = modifier.heightIn(min = 48.dp), enabled = enabled) {
        Text(text = text, fontSize = 16.sp)
    }
}

@Composable
internal fun PadSecondaryButton(
    text: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    enabled: Boolean = true,
) {
    OutlinedButton(
        onClick = onClick,
        modifier = modifier.heightIn(min = 48.dp),
        enabled = enabled,
    ) {
        Text(text = text, fontSize = 16.sp)
    }
}

@Composable
internal fun PadCard(content: @Composable ColumnScope.() -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = Color(0xFF161616)),
    ) {
        Column(modifier = Modifier.padding(16.dp), content = content)
    }
}

@Composable
internal fun PadMetric(label: String, value: String, color: Color, hint: String? = null) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Text(text = label, fontSize = 13.sp, color = PadTextSecondary)
        Spacer(Modifier.height(4.dp))
        Text(text = value, fontSize = 22.sp, fontWeight = FontWeight.Bold, color = color)
        hint?.let {
            Spacer(Modifier.height(2.dp))
            Text(text = it, fontSize = 11.sp, color = Color(0xFF707070))
        }
    }
}

@Composable
internal fun PadRowMetrics(vararg metrics: Triple<String, String, Color>) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceEvenly,
    ) {
        metrics.forEach { (label, value, color) ->
            PadMetric(label = label, value = value, color = color)
        }
    }
}

@Composable
internal fun PadBanner(text: String, color: Color = PadWarn) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .background(color.copy(alpha = 0.12f), MaterialTheme.shapes.small)
            .padding(12.dp),
    ) {
        Text(text = text, fontSize = 13.sp, color = color)
    }
}

internal fun padResolveMediaUrl(path: String): String =
    if (path.startsWith("http")) path else BuildConfig.API_BASE_URL.trimEnd('/') + path

/** 轻量远程图片（与电视端同一思路，改用 Material3 文本）。 */
@Composable
internal fun PadRemoteImage(
    url: String,
    maxHeight: Dp = 200.dp,
    contentDescription: String = "图片",
) {
    var bitmap by remember(url) { mutableStateOf<androidx.compose.ui.graphics.ImageBitmap?>(null) }
    var failed by remember(url) { mutableStateOf(false) }
    LaunchedEffect(url) {
        val loaded = withContext(Dispatchers.IO) {
            runCatching { URL(url).openStream().use { BitmapFactory.decodeStream(it)?.asImageBitmap() } }
                .getOrNull()
        }
        if (loaded == null) failed = true else bitmap = loaded
    }
    val image = bitmap
    if (image != null) {
        Image(
            bitmap = image,
            contentDescription = contentDescription,
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(max = maxHeight),
            contentScale = ContentScale.Fit,
        )
    } else if (failed) {
        Text("图片加载失败", fontSize = 13.sp, color = PadWarn)
    } else {
        Text("图片加载中…", fontSize = 13.sp, color = PadTextSecondary)
    }
}

/** 秒数转「N 分 NN 秒」。 */
internal fun padFormatDuration(totalSeconds: Long): String {
    val seconds = totalSeconds.coerceAtLeast(0)
    val minutes = seconds / 60
    val rest = seconds % 60
    return if (minutes > 0) "%d 分 %02d 秒".format(minutes, rest) else "${rest} 秒"
}
