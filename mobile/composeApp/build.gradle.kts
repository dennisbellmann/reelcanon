import org.jetbrains.kotlin.gradle.dsl.JvmTarget

plugins {
    alias(libs.plugins.kotlin.multiplatform)
    alias(libs.plugins.kotlin.serialization)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.compose)
    alias(libs.plugins.android.multiplatform)
    alias(libs.plugins.ksp)
}

kotlin {
    android {
        namespace = "com.reelcanon.shared"
        compileSdk = 37
        minSdk = 26
        androidResources { enable = true }
        compilerOptions { jvmTarget.set(JvmTarget.JVM_17) }
        withHostTestBuilder {}.configure {}
    }
    // Native toolchain and Xcode run only on macOS. Android builds work on Windows/Linux.
    if (System.getProperty("os.name").contains("Mac")) {
        listOf(iosArm64(), iosSimulatorArm64()).forEach {
            it.binaries.framework { baseName = "ReelCanon"; isStatic = true }
        }
    }
    sourceSets {
        commonMain.dependencies {
            implementation(compose.runtime)
            implementation(compose.foundation)
            implementation(compose.material3)
            implementation(compose.ui)
            implementation(compose.components.resources)
            implementation(libs.coroutines)
            implementation(libs.serialization)
            implementation(libs.room.runtime)
            implementation(libs.sqlite)
            implementation(libs.ktor.core)
            implementation(libs.ktor.content)
            implementation(libs.ktor.json)
            implementation(libs.coil.compose)
            implementation(libs.coil.network)
        }
        commonTest.dependencies {
            implementation(kotlin("test"))
            implementation(libs.coroutines.test)
            implementation(libs.ktor.mock)
        }
        androidMain.dependencies {
            implementation(libs.ktor.okhttp)
            implementation(libs.credentials)
            implementation(libs.credentials.play)
            implementation(libs.googleid)
        }
        if (System.getProperty("os.name").contains("Mac")) {
            getByName("iosMain").dependencies { implementation(libs.ktor.darwin) }
        }
    }
}
dependencies {
    add("kspAndroid", libs.room.compiler)
    if (System.getProperty("os.name").contains("Mac")) {
        add("kspIosArm64", libs.room.compiler)
        add("kspIosSimulatorArm64", libs.room.compiler)
    }
}
ksp { arg("room.schemaLocation", "$projectDir/schemas") }

compose.resources {
    publicResClass = true
    packageOfResClass = "com.reelcanon.shared.generated.resources"
}
