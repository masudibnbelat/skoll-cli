// Location: templates/empty/app/build.gradle.kts
import java.io.FileInputStream
import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// Release signing: `angrboda build --release` / `angrboda release`
// create a release keystore once (~/.angrboda/keystore/) and write
// its path + passwords into local.properties (gitignored). Without
// those four keys the release APK is built unsigned.
// NOTE: do NOT use java.util.Properties() here — in a Gradle Kotlin
// DSL script `java` resolves to the Java plugin extension, so
// `java.util` fails with "Unresolved reference 'util'". Import the
// class and use the simple name.
val releaseKeystoreProps = Properties()
val localPropsFile = rootProject.file("local.properties")
if (localPropsFile.exists()) {
    releaseKeystoreProps.load(FileInputStream(localPropsFile))
}
val hasReleaseKey = listOf(
    "releaseStoreFile", "releaseStorePassword",
    "releaseKeyAlias", "releaseKeyPassword"
).all { releaseKeystoreProps.containsKey(it) }

android {
    namespace = "{{Package}}"
    compileSdk = 36

    signingConfigs {
        if (hasReleaseKey) {
            create("release") {
                storeFile = file(releaseKeystoreProps.getProperty("releaseStoreFile"))
                storePassword = releaseKeystoreProps.getProperty("releaseStorePassword")
                keyAlias = releaseKeystoreProps.getProperty("releaseKeyAlias")
                keyPassword = releaseKeystoreProps.getProperty("releaseKeyPassword")
            }
        }
    }

    defaultConfig {
        applicationId = "{{Package}}"
        minSdk = {{MinSdk}}
        targetSdk = 36
        versionCode = 1
        versionName = "1.0"
    }

    buildTypes {
        release {
            if (hasReleaseKey) signingConfig = signingConfigs.getByName("release")
            isMinifyEnabled = false
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
}

kotlin {
    compilerOptions {
        jvmTarget = org.jetbrains.kotlin.gradle.dsl.JvmTarget.JVM_11
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.16.0")
}
