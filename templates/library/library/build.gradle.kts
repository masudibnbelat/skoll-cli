// Location: templates/library/library/build.gradle.kts
plugins {
    id("com.android.library")
}

android {
    namespace = "{{Package}}"
    {{CompileSdkBlock}}

    defaultConfig {
        minSdk = {{MinSdk}}
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            consumerProguardFiles("consumer-rules.pro")
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
