plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

// Release signing: `angrboda build --release` / `angrboda release`
// create a release keystore once (~/.angrboda/keystore/) and write
// its path + passwords into local.properties (gitignored). Without
// those four keys the release APK is built unsigned.
val releaseKeystoreProps = java.util.Properties()
val localPropsFile = rootProject.file("local.properties")
if (localPropsFile.exists()) {
    localPropsFile.inputStream().use { releaseKeystoreProps.load(it) }
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

    kotlinOptions {
        jvmTarget = "11"
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.16.0")
}
