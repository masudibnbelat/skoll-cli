// Location: templates/empty/build.gradle.kts
// Top-level build file
// Kotlin is built into AGP 9 — there is no org.jetbrains.kotlin.android
// plugin anymore (applying it fails the build). The Kotlin version
// follows AGP's built-in default for this template.
plugins {
    id("com.android.application") version "{{AgpVersion}}" apply false
}
