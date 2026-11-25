<div align="center">

![angrboda](assets/banner.svg)

[![Release](https://img.shields.io/github/v/release/masudibnbelat/angrboda-cli?style=for-the-badge)](https://github.com/masudibnbelat/angrboda-cli/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/masudibnbelat/angrboda-cli/release.yml?style=for-the-badge)](https://github.com/masudibnbelat/angrboda-cli/actions)
[![Go](https://img.shields.io/github/go-mod/go-version/masudibnbelat/angrboda-cli?style=for-the-badge)](https://go.dev/)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS%20%7C%20Windows-blue?style=for-the-badge)](https://github.com/masudibnbelat/angrboda-cli/releases)
[![Downloads](https://img.shields.io/github/downloads/masudibnbelat/angrboda-cli/total?style=for-the-badge)](https://github.com/masudibnbelat/angrboda-cli/releases)
[![Stars](https://img.shields.io/github/stars/masudibnbelat/angrboda-cli?style=for-the-badge)](https://github.com/masudibnbelat/angrboda-cli/stargazers)

[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat-square)](https://github.com/masudibnbelat/angrboda-cli/pulls)
[![Last Commit](https://img.shields.io/github/last-commit/masudibnbelat/angrboda-cli?style=flat-square)](https://github.com/masudibnbelat/angrboda-cli/commits/main)
[![Issues](https://img.shields.io/github/issues/masudibnbelat/angrboda-cli?style=flat-square)](https://github.com/masudibnbelat/angrboda-cli/issues)

**One small CLI for any Android project.**
Build it. Run it on your phone. Release it on GitHub. One command each.

[![Typing SVG](https://readme-typing-svg.demolab.com?font=Fira+Code&size=22&pause=1000&color=FFFFFF&background=0D0D0D&center=true&vCenter=true&width=600&height=50&lines=angrboda+build;angrboda+run;angrboda+release+v1.5.0)](https://github.com/masudibnbelat/angrboda-cli)

[![angrboda-cli](https://github-readme-stats.vercel.app/api/pin/?username=masudibnbelat&repo=angrboda-cli&theme=dark)](https://github.com/masudibnbelat/angrboda-cli)

</div>

---

## Contents

- [Why angrboda?](#why-angrboda)
- [See how it works](#see-how-it-works)
- [See it in action](#see-it-in-action)
- [angrboda vs doing it by hand](#angrboda-vs-doing-it-by-hand)
- [1. User Guide](#1-user-guide)
  - [1.1 Installation Process](#11-installation-process)
  - [1.2 Usage](#12-usage)
  - [1.3 Command List](#13-command-list)
  - [1.4 How It Works](#14-how-it-works)
- [2. Developer Guide](#2-developer-guide)
- [Contributing](#contributing)
- [Changelog](#changelog)
- [Troubleshooting](#troubleshooting)
- [What angrboda does NOT do](#what-angrboda-does-not-do)

---

## Why angrboda?

| | |
|---|---|
| **Any Android project** | Kotlin or Java — it makes no difference. `angrboda` never reads your source code. It only runs Gradle and talks to `adb`. |
| **No setup fights** | Java, Gradle, and `adb` install themselves when they are missing. No Android Studio needed for them. |
| **Right Gradle, every time** | The version comes from your own project. Gradle 8.9 in one project, 8.5 in another — `angrboda` uses the right one. It never uses the wrong Gradle from `PATH`. |
| **APK where you can find it** | Every build also copies the APK to `MyApp/apk/MyApp-debug.apk` (or `-release.apk`). No digging through `app/build/outputs/apk/`. |
| **Safe releases** | A release stops on a dirty tree, checks the APK version matches the tag, and never uploads the wrong version. |
| **Small and fast** | One binary, about 9 MB. Written in Go with Cobra — one small dependency. |

> [!NOTE]
> **Runs on your computer only — Windows, Linux, or macOS.**
> It does **not** run on your phone. Your phone is only used to install
> and open the app. `build` and `release` do not need a phone at all.

---

## See how it works

```mermaid
flowchart LR
    A["You run a command"] --> B{"Java? Gradle? adb?"}
    B -->|"Missing"| C["angrboda downloads it once\ninto ~/.angrboda/"]
    B -->|"Ready"| D["Gradle builds your app"]
    C --> D
    D --> E["APK copied to apk/"]
    E --> F["Phone or GitHub Release"]
```

---

## See it in action

A fresh computer — no Java, no `adb`, no Gradle installed. Just `angrboda`:

![angrboda terminal session](assets/terminal.svg)

The same session as text:

```
$ angrboda doctor
angrboda 0.2.0 — environment check
java: MISSING (auto-installs Temurin 17 on build)
adb: MISSING (auto-installs platform-tools on run)
gh: /usr/bin/gh
---
Project: /home/professor/MyApp
Wrapper Gradle version: 8.9
Gradle: /home/professor/MyApp/gradlew

$ angrboda build
== Java not found — downloading Eclipse Temurin 17 once...
== Java: /home/professor/.angrboda/jdk/temurin-17/bin/java
== Gradle: /home/professor/MyApp/gradlew
== APK: /home/professor/MyApp/apk/MyApp-debug.apk

$ angrboda devices
== adb not found — downloading Google platform-tools once...
== adb: /home/professor/.angrboda/platform-tools/adb
List of devices attached
bb0758c3	device
```

First time only — next builds reuse the saved copies, in under a second.

---

## angrboda vs doing it by hand

| Step | By hand | With angrboda |
|---|---|---|
| Install Java | Download a JDK, set `JAVA_HOME`, fix the version | Automatic on first build |
| Install adb | Download platform-tools, add it to `PATH` | Automatic on first run |
| Pick the Gradle version | Read the wrapper file, find a matching install | Automatic, from your project |
| Find the APK | Dig through `app/build/outputs/apk/` | Copied to `apk/` for you |
| Make a release | Tag, build, check the version, push, `gh release create` | One command: `angrboda release v1.5.0` |

---

## 1. User Guide

### 1.1 Installation Process

Follow these steps in order. Nothing else is needed.

**Step 1 — Run the install command for your computer.**

Linux / macOS — paste this in a terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/masudibnbelat/angrboda-cli/main/install.sh | bash
```

Windows — paste this in PowerShell:

```powershell
irm https://raw.githubusercontent.com/masudibnbelat/angrboda-cli/main/install.ps1 | iex
```

**Step 2 — Close the terminal and open it again.**

This loads the new `angrboda` command. Without this step, your
terminal still uses the old setup.

**Step 3 — Check that it works.**

```bash
angrboda --version
```

You should see `angrboda 0.2.0` (or newer).

**Step 4 — Check your setup.**

```bash
angrboda doctor
```

Example output:

```
angrboda 0.2.0 — environment check
ANDROID_HOME: /home/professor/Android/Sdk
java: /usr/bin/java (system)
adb: /home/professor/Android/Sdk/platform-tools/adb (system)
gh: /usr/bin/gh
gradle: /home/professor/.local/opt/gradle-9.3.1/bin/gradle
---
List of devices attached
bb0758c3	device
Project: /home/professor/MyApp
Wrapper Gradle version: 8.9
Gradle: /home/professor/MyApp/gradlew
```

**Step 5 — Know what you need first.**

| Need | For |
|---|---|
| Android SDK (auto-installed on first build) | Building |
| `git` | `release` |
| `gh` (GitHub CLI, `gh auth login` once) | `release` only |
| A phone with USB debugging on | `run`, `devices`, `logs`, `uninstall` |

> [!TIP]
> Java, `adb`, the Android SDK and the Flutter SDK are **not** in this list — `angrboda` installs them itself
> when they are missing. See [1.4 How It Works](#14-how-it-works).

---

### 1.2 Usage

<table align="center">
  <tr>
    <td align="center" width="33%">
      <img src="assets/icon-build.svg" width="72" alt="Build"><br>
      <b>Build</b><br>
      No phone needed
    </td>
    <td align="center" width="33%">
      <img src="assets/icon-run.svg" width="72" alt="Run"><br>
      <b>Run</b><br>
      Phone needed
    </td>
    <td align="center" width="33%">
      <img src="assets/icon-release.svg" width="72" alt="Release"><br>
      <b>Release</b><br>
      No phone needed
    </td>
  </tr>
</table>

Every use starts the same way: go into your Android project folder
first.

```bash
cd MyApp
```

#### Use 0 — Create a new project (no Android Studio needed)

```bash
angrboda new
```

It asks three things — the project name, the project type, then the package name (the example is shown; press Enter to accept it):

```
Project name: MyApp
Project type:
  1) Kotlin (Compose + Material 3)
  2) Flutter
  3) Empty (Kotlin, no Compose)
  4) Library (Android .aar)
Choose [1]:
Package name [com.example.myapp]:
```

That creates `MyApp/` — a Kotlin project with the latest stable versions (Kotlin, AGP, Gradle, Compose BOM are looked up when you run it; pinned versions are used offline), a Flutter project from Flutter's own current template (Flutter 3.50, Dart ^3.14.0, Gradle 9.5.0, AGP 9.3.1, Kotlin 2.4.20), an Empty Kotlin project (one Activity, no Compose), or a Library project (one module, builds an .aar). Then:

```bash
cd MyApp
angrboda build
```

No prompts? `angrboda new MyApp -t flutter -p com.masud.myapp`. Short form: `angrboda n MyApp`. To create the project in the current folder instead of a new subfolder: `angrboda init -t empty`. Pin a version instead of the latest: `angrboda new MyApp --gradle 9.3.1 --agp 9.1.1 --kotlin 2.2.10 --bom 2025.09.00` (pins are ignored for Flutter — it uses its own pinned versions).

Flutter projects: the Flutter SDK installs itself on first build (large — over 1 GB, once) and are Android-only for now — add other platforms later with `flutter create --platforms=ios .`. `build`, `run`, `clean`, `doctor` and `release` all work for Flutter projects.

#### Use 1 — Build only (no phone needed)

**Step 1 —** Go into your project folder (`cd MyApp`).

**Step 2 —** Run:

```bash
angrboda build
```

**Step 3 —** Find your APK here (the original stays where Gradle put it):

```
MyApp/apk/MyApp-debug.apk
```

For a release build, use `angrboda build --release` instead.
That makes `MyApp/apk/MyApp-release.apk` — signed with your release
keystore (created once on the first release build; see `angrboda
keystore` and back it up). For a library project, `angrboda build`
makes `MyLib/aar/MyLib-release.aar` instead — there is no app to run.

For Google Play, build the App Bundle instead of the APK:

```bash
angrboda build --release --bundle
```

That makes `MyApp/bundle/MyApp-release.aab` — new apps on Google
Play need an .aab, not an .apk. (`angrboda release --bundle`
uploads both the APK and the .aab.)

#### Use 2 — Build + install on your phone (phone needed)

**Step 1 —** Connect your phone by USB and turn on USB debugging.

**Step 2 —** Go into your project folder (`cd MyApp`).

**Step 3 —** Run:

```bash
angrboda run
```

**Step 4 —** `angrboda` builds the app, installs it on your phone,
and opens it. The APK is also copied to
`MyApp/apk/MyApp-debug.apk`
(or `MyApp-release.apk` with `angrboda run --release`).

If you have two phones connected, pick one:

```bash
angrboda run --device bb0758c3
```

#### Use 3 — Build + GitHub release (no phone needed)

**Step 1 —** Go into your project folder (`cd MyApp`).

**Step 2 —** Run:

```bash
angrboda release v1.5.0 "Fixed percent bug"
```

**Step 3 —** `angrboda` does everything for you, in order:

1. Bumps the version for you (Kotlin: versionName becomes the tag and versionCode goes up by one; Flutter: version in pubspec.yaml becomes the tag and the build number goes up by one; library: the tag is the version)
2. Makes the tag `v1.5.0`
3. Builds the release APK (signed — see below) or the .aar for a library
4. Checks the APK version matches the tag, and that it is really signed
5. Copies the APK to `MyApp/apk/MyApp-release.apk`
6. Pushes the code and the tag to GitHub
7. Creates the GitHub Release with the APK

No phone needed. You need internet and `gh auth login` done once.

**Release signing.** The first release build creates one release keystore, shared by all your apps, at `~/.angrboda/keystore/release.keystore` (alias `release`), and points the project at it — `local.properties` for Kotlin, `android/key.properties` for Flutter. Both files are gitignored. A keystore you already configured is never overwritten. See it any time with:

```bash
angrboda keystore
```

It prints the path, alias and SHA-1 / SHA-256 fingerprints. **Back up the keystore file and its passwords.** If it is lost, the app can never be updated on Google Play again. Move it to another machine with:

```bash
angrboda keystore export ~/keystore-backup   # on the old machine
angrboda keystore import ~/keystore-backup   # on the new machine
```

Import refuses when a keystore already exists on the new machine — a silent replacement would break Play Store updates for every app signed with the old key.

A download that is interrupted (net drops, laptop sleeps) resumes where it stopped the next time you run the command — the half-downloaded file is kept and only the rest is fetched. Before a big download angrboda also checks there is enough free disk space for it.

> [!TIP]
> To see what will happen, without doing it:
>
> ```bash
> angrboda release --dry-run
> ```
>
> To release the next patch version without typing a number:
>
> ```bash
> angrboda release
> ```
>
> `v1.4.2` becomes `v1.4.3`.

---

### 1.3 Command List

| Command | What it does | Phone needed? |
|---|---|---|
| `angrboda new [name] [-t kotlin|flutter|empty|library] [-p PKG] [--gradle V --agp V --kotlin V --bom V]` | Create a new Kotlin, Flutter, Empty or Library project | No |
| `angrboda init [name] [-t TYPE] [-p PKG]` | Create a project in this folder | No |
| `angrboda run [--clean] [--device SERIAL] [--release]` | Build, install on your phone, open it | **Yes** |
| `angrboda build [--release] [--clean] [--bundle]` | Build only, copy APK to `apk/` (.aar to `aar/` for a library, .aab to `bundle/` with --bundle) | No |
| `angrboda release [v1.5.0] ["notes"] [--major\|--minor\|--dry-run\|--yes\|--bundle]` | Bump version, tag, build (signed), check, push, GitHub Release | No |
| `angrboda keystore` | Show the release keystore (creates it if missing) | No |
| `angrboda keystore export <folder>` | Copy keystore + passwords to a folder (backup / new machine) | No |
| `angrboda keystore import <folder>` | Copy a keystore + passwords back in | No |
| `angrboda devices` | List connected phones | Yes |
| `angrboda logs` | Live logcat for your app | Yes |
| `angrboda uninstall` | Uninstall your app from the phone | Yes |
| `angrboda clean` | Gradle clean | No |
| `angrboda doctor` | Check SDK, Java, adb, Gradle, keystore, git, gh login, disk space, project | No |
| `angrboda update [--check]` | Update angrboda itself to the latest release | No |
| `angrboda --help` | Show help | No |
| `angrboda --version` | Show version | No |

---

### 1.4 How It Works

```mermaid
flowchart LR
    A["angrboda"] --> B["Gradle + JDK 17"]
    A --> C["adb"]
    B --> D["APK"]
    C --> E["Phone"]
    D --> E
    D --> F["GitHub Release"]
```

<details>
<summary><b>Gradle — picked from your project, never hardcoded</b></summary>

`angrboda` reads the version from your own project:

```
gradle/wrapper/gradle-wrapper.properties
```

Order:

1. `./gradlew` in your project (the wrapper downloads its own Gradle).
2. A local install of that exact version
   (`~/.local/opt/gradle-<version>`, the wrapper cache, or `~/.angrboda/gradle/`).
3. If it is nowhere, `angrboda` downloads that exact version once
   to `~/.angrboda/gradle/gradle-<version>/`.

The Gradle on `PATH` is never used as a fallback when the wrapper
wants a different version — that is the bug that broke AGP 8.x
projects with Gradle 9.x.

</details>

<details>
<summary><b>Java — Temurin 17, only when you need it</b></summary>

- You already have Java 17–21? `angrboda` uses yours. Nothing downloads.
- No Java, or the wrong version? On the first build, `angrboda` downloads
  Eclipse Temurin 17 once (about 190 MB) to `~/.angrboda/jdk/`
  (`%USERPROFILE%\.angrboda\jdk\` on Windows) and sets `JAVA_HOME`
  for Gradle. Next builds use the saved copy.

</details>

<details>
<summary><b>adb — Google platform-tools, only when you need it</b></summary>

- `adb` already on your computer? `angrboda` uses it. Nothing downloads.
- No `adb`? On the first `run`, `devices`, `logs`, or `uninstall`,
  `angrboda` downloads Google's platform-tools once (about 10–15 MB)
  to `~/.angrboda/platform-tools/`
  (`%USERPROFILE%\.angrboda\platform-tools\` on Windows).

</details>

<div align="right">

[Back to top ↑](#contents)

</div>

---

## 2. Developer Guide

You want to change `angrboda` itself? You only need Go 1.23 or newer.
Follow these steps in order. Everything is copy-paste.

**Step 1 — Get the code.**

```bash
git clone https://github.com/masudibnbelat/angrboda-cli.git
cd angrboda-cli
```

**Step 2 — Change the code.**

Edit any `.go` file. When you get changed files from someone,
put them in this folder and replace the old ones.

**Step 3 — Build and install, in one command.**

Linux / macOS:

```bash
go build -o angrboda . && cp angrboda ~/.local/bin/angrboda && angrboda --version
```

Windows (PowerShell):

```powershell
go build -o angrboda.exe .; Copy-Item angrboda.exe "$env:LOCALAPPDATA\angrboda\bin\angrboda.exe" -Force; angrboda --version
```

That one command builds it, installs it, and shows the new version.
If it shows the old version, close the terminal and open it again.

**Step 4 — Test it.**

```bash
angrboda doctor
```

Then test the command you changed, for example `angrboda build`
in a test Android project.

**Step 5 — Release a new version.**

First, change the version number in `main.go`
(`const version = "0.2.0"`). Then run this:

```bash
git add -A
git commit -m "v0.2.0: what changed"
git push
git tag v0.2.0
git push origin v0.2.0
```

> [!WARNING]
> Tag only **after** you pushed the latest code — the tag must be on
> the newest commit, or the Release will build old code.

After the push, GitHub Actions builds the Release by itself.
Before you tell anyone to install, check the Release page has
files named `angrboda-linux-amd64` and `angrboda-windows-amd64.exe`
(no `.tar.gz` / `.zip`).

<div align="right">

[Back to top ↑](#contents)

</div>

---

## Contributing

Found a bug, or want a new command? Contributions are welcome.
Follow these steps:

**Step 1 —** Fork the repo on GitHub, then clone your fork:

```bash
git clone https://github.com/YOUR-NAME/angrboda-cli.git
cd angrboda-cli
```

**Step 2 —** Make a new branch for your change:

```bash
git checkout -b my-change
```

**Step 3 —** Change the code, then build and test it
(see [2. Developer Guide](#2-developer-guide), Steps 3–4).

**Step 4 —** Commit with a clear message:

```bash
git add -A
git commit -m "Add angrboda logs --follow"
```

**Step 5 —** Push and open a pull request on GitHub.
Write what you changed and how you tested it.

> [!TIP]
> Keep it small: one change per pull request.

---

## Changelog

| Version | What changed |
|---|---|
| **v0.9.1** | A failed release really removes the local tag on every post-tag failure: copy failures (APK / .aar / .aab) and `git push` failures now go through the same cleanup, and the copy helpers return errors instead of dying; `--bundle` on a library project is rejected before the tag is created. `gh release create` failing still keeps the tag — the code and tag are already pushed by then. Version bumped so `angrboda version` tells the fixed build apart. |
| **v0.9.0** | Release APKs are signed: first release build creates one shared keystore (`~/.angrboda/keystore/`, `angrboda keystore` shows it) and `release` verifies the signature with apksigner (unsigned or debug-signed fails); `build --release` and `run --release` also verify the signature; a failed release really removes the local tag again (the old defer never ran, because `die()` calls `os.Exit`); `release` works for Flutter (pubspec version bump) and libraries (.aar upload) and bumps the version for you (Kotlin versionName + versionCode, Flutter build number); `new` adds Empty and Library types; `angrboda init` scaffolds in the current folder; `--gradle/--agp/--kotlin/--bom` pin versions on `new`/`init`; downloads resume after an interruption (Range) and check free disk space first; `build --bundle` / `release --bundle` build the App Bundle (.aab) for Google Play; `keystore export` / `import` move the keystore to another machine; `doctor` shows keystore, git and gh login status; doctor never downloads Gradle as a side effect (read-only lookup) and shows free disk space (warns below 2 GB); the disk-space check works on Windows too (GetDiskFreeSpaceExW); the update notice re-checks every 6 hours instead of once a day |
| **v0.8.1** | Automatic update notice: every command checks once a day (in the background) whether a newer release exists and shows a notice box with `angrboda update`; never slows a command, never fails one, silent when piped (ANGRBODA_NO_UPDATE_CHECK=1 turns it off) |
| **v0.8.0** | Terminal UI: colored steps (→ / ✓ / ✗), live download bar with percent, sizes, speed and ETA, spinner with elapsed time, APK summary with size and build time; plain output when piped (NO_COLOR=1 forces plain) |
| **v0.7.0** | Android SDK + Flutter SDK auto-install on first build (nothing left to install by hand); `angrboda update` self-updates with SHA256 check |
| **v0.6.0** | `new` makes Kotlin or Flutter projects (type question + `-t` flag); Flutter template from Flutter 3.50 stable; `build`/`run`/`clean`/`doctor` work for Flutter; `release` stops clearly for Flutter |
| **v0.5.0** | Renamed ktdev-cli to angrboda-cli; `new` is interactive (name + package prompts); loading spinner on downloads and version lookup; `new` always uses the latest stable versions (pinned fallback offline) |
| **v0.4.2** | `angrboda new` default package is neutral com.example.<name> — the com.masud default was wrong for a universal CLI |
| **v0.4.1** | `angrboda new` is shorter: `angrboda new Moan` makes com.masud.moan, `-p` overrides, `n` alias |
| **v0.4.0** | `angrboda new` creates a Compose + Material 3 project (Kotlin 2.2.10, AGP 9.1.1, Gradle 9.3.1) |
| **v0.3.0** | Cobra CLI: per-command `--help`, flag errors with suggestions, `version` subcommand, shell completion |
| **v0.2.0** | Java + adb auto-install (Temurin 17, Google platform-tools); `doctor` reports without downloading |
| **v0.1.0** | First release: `build`, `run`, `release`, `devices`, `logs`, `uninstall`, `clean`, `doctor`; dynamic Gradle; APK copy to `apk/` |

---

## Troubleshooting

| Problem | Fix |
|---|---|
| `no Android project found` | Go into your project folder first (`cd MyApp`) |
| `no phone found` | Connect the phone, turn on USB debugging, run `adb devices` |
| First build downloads Java | Normal, only once (about 190 MB). Next builds use the saved copy. |
| First `run` downloads adb | Normal, only once (about 10–15 MB). |
| First build downloads Gradle | Normal, only once. It downloads your project's exact Gradle version. |
| `tag v1.5.0 already exists` | Use a new version number |
| `Working tree is not clean` | Commit your changes first, or add `--yes` |
| `gh is not logged in` | Run `gh auth login` once |
| `APK stamp mismatch` | Do not upload it. Check the tag and version, then try again. |

---

## What angrboda does NOT do

- **It does not install Android Studio (the IDE).** The SDK, Java, `adb` and Flutter all install themselves; only the IDE itself is yours to install if you want it.
- **It does not support React Native.** Flutter projects work (Android-only for now).
- **It does not run on your phone.** It runs on Windows, Linux,
  or macOS only.

---

<div align="center">

Made with Go + Cobra.

If angrboda saves you time, give it a star ⭐ — it helps others find it.

</div>
