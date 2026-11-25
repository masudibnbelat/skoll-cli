plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
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

    buildFeatures {
        compose = true
    }
}

dependencies {
    val composeBom = platform("androidx.compose:compose-bom:{{ComposeBomVersion}}")
    implementation(composeBom)
    androidTestImplementation(composeBom)

    implementation("androidx.core:core-ktx:1.16.0")
    implementation("androidx.activity:activity-compose:1.10.1")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-graphics")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("androidx.compose.material3:material3")

    debugImplementation("androidx.compose.ui:ui-tooling")
    debugImplementation("androidx.compose.ui:ui-test-manifest")
}
