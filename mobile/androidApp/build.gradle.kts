plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
}

val configuredApi = providers.gradleProperty("reelcanon.apiBaseUrl")
    .orElse("http://10.0.2.2:8080/api/v1")

android {
    namespace = "com.reelcanon.app"
    compileSdk = 37
    defaultConfig {
        applicationId = "com.reelcanon.app"
        minSdk = 26
        targetSdk = 36
        versionCode = 1
        versionName = "0.1.0"
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }
    buildFeatures { compose = true; buildConfig = true }
    buildTypes {
        debug { buildConfigField("String", "API_BASE_URL", "\"${configuredApi.get()}\"") }
        release {
            // A release must explicitly supply a real HTTPS catalog endpoint.
            buildConfigField("String", "API_BASE_URL", "\"${configuredApi.get()}\"")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
}
tasks.matching { it.name == "preReleaseBuild" }.configureEach {
    doFirst {
        require(configuredApi.get().startsWith("https://")) {
            "Release requires -Preelcanon.apiBaseUrl=https://your-host/api/v1"
        }
    }
}
dependencies {
    implementation(project(":composeApp"))
    implementation(libs.activity.compose)
    implementation(libs.coroutines)
    androidTestImplementation(libs.android.test.runner)
    androidTestImplementation(libs.android.test.junit)
    androidTestImplementation(libs.compose.test)
    androidTestImplementation(libs.ktor.mock)
    androidTestImplementation(libs.ktor.core)
    androidTestImplementation(libs.ktor.content)
    androidTestImplementation(libs.ktor.json)
    androidTestImplementation(libs.serialization)
    androidTestImplementation(libs.room.runtime)
    androidTestImplementation(libs.sqlite)
    debugImplementation(libs.compose.test.manifest)
}
