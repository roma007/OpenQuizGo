import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.plugin.compose")
}

android {
    namespace = "com.openquizgo.guard"
    compileSdk = 35

    defaultConfig {
        applicationId = "com.openquizgo.quiz"
        minSdk = 26
        targetSdk = 33
        versionCode = 129
        versionName = "1.0.29"
        // 080：TV 回归服务端模式。默认走 Tailscale 内网（备案前 8082 只放行 tailnet）。
        // 覆盖方式：./gradlew assembleRelease -PapiBaseUrl=https://quiz.openapp.asia/
        val apiBaseUrl = providers.gradleProperty("apiBaseUrl")
            .orElse("http://100.65.110.80:8082/")
            .get()
        buildConfigField("String", "API_BASE_URL", "\"$apiBaseUrl\"")
        val expectedSignature = providers.gradleProperty("expectedSignature")
            .orElse("")
            .get()
        buildConfigField("String", "EXPECTED_SIGNATURE", "\"$expectedSignature\"")
    }

    // 生产签名：在仓库根目录放置 keystore.properties（已 gitignore），否则回退 debug 签名仅供内部验收。
    val keystorePropsFile = rootProject.file("keystore.properties")
    val hasReleaseKey = keystorePropsFile.exists()
    val releaseProps = Properties().apply {
        if (hasReleaseKey) keystorePropsFile.inputStream().use { load(it) }
    }

    signingConfigs {
        if (hasReleaseKey) {
            create("release") {
                storeFile = rootProject.file(releaseProps.getProperty("storeFile"))
                storePassword = releaseProps.getProperty("storePassword")
                keyAlias = releaseProps.getProperty("keyAlias")
                keyPassword = releaseProps.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = if (hasReleaseKey) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("debug")
            }
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    packaging {
        resources.excludes += "/META-INF/{AL2.0,LGPL2.1}"
    }

    testOptions {
        // GuardLog 直接调用 android.util.Log；JVM 单测下返回默认值即可。
        unitTests.isReturnDefaultValues = true
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:2024.12.01")
    implementation(composeBom)

    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.activity:activity-compose:1.9.3")
    implementation("androidx.lifecycle:lifecycle-runtime-compose:2.8.7")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.7")
    implementation("androidx.tv:tv-material:1.0.0")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    debugImplementation("androidx.compose.ui:ui-tooling")

    implementation("androidx.datastore:datastore-preferences:1.1.1")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")

    implementation("com.squareup.retrofit2:retrofit:2.11.0")
    implementation("com.squareup.retrofit2:converter-gson:2.11.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.squareup.okhttp3:logging-interceptor:4.12.0")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.jetbrains.kotlinx:kotlinx-coroutines-test:1.9.0")
}
