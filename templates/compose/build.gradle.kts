// Location: templates/compose/build.gradle.kts
// Top-level build file
// Kotlin is built into AGP 9 — there is no org.jetbrains.kotlin.android
// plugin anymore (applying it fails the build). The Compose compiler
// plugin below carries the Kotlin version, so Gradle aligns the
// built-in Kotlin to it automatically.
plugins {
    id("com.android.application") version "{{AgpVersion}}" apply false
    id("org.jetbrains.kotlin.plugin.compose") version "{{KotlinVersion}}" apply false
}
