---
title: Languages
description: Install language packs, change TTT's display language, and keep translations up to date.
---

TTT 1.12.0 and later supports interface language packs for menus, commands, settings, and dialogs. English is built in; other languages are installed through the Plugins panel.

## Change the display language

1. Open the **Plugins** sidebar tab, or run **Plugins: Show Panel** from the command palette (**Ctrl+P**).
2. Search for your language or `language-pack`. Select its pack and click **Install**, then approve the plugin when prompted.
3. Run **Change Display Language** and select the installed language. You can also use **Settings → General → Display language** and click **Apply**.
4. Restart TTT to use the selected language.

Installing a pack makes it available in the language picker without changing your current language. You can install several packs and switch between them. Each selection takes effect after a restart.

## Available languages

The community registry includes these 30 language packs. Search by the language name or the pack name shown below. English needs no plugin.

| Language | Pack name in the Plugins panel |
|----------|--------------------------------|
| Arabic | العربية Language Pack |
| Bengali | বাংলা Language Pack |
| Chinese (Simplified) | 简体中文 Language Pack |
| Chinese (Traditional) | 繁體中文 Language Pack |
| Czech | Čeština Language Pack |
| Danish | Dansk Language Pack |
| Dutch | Nederlands Language Pack |
| Finnish | Suomi Language Pack |
| French | Français Language Pack |
| German | Deutsch Language Pack |
| Greek | Ελληνικά Language Pack |
| Hebrew | עברית Language Pack |
| Hindi | हिन्दी Language Pack |
| Hungarian | Magyar Language Pack |
| Indonesian | Bahasa Indonesia Language Pack |
| Italian | Italiano Language Pack |
| Japanese | 日本語 Language Pack |
| Korean | 한국어 Language Pack |
| Norwegian Bokmål | Norsk bokmål Language Pack |
| Persian | فارسی Language Pack |
| Polish | Polski Language Pack |
| Portuguese (Brazil) | Português (Brasil) Language Pack |
| Romanian | Română Language Pack |
| Russian | Русский Language Pack |
| Spanish | Español Language Pack |
| Swedish | Svenska Language Pack |
| Thai | ไทย Language Pack |
| Turkish | Türkçe Language Pack |
| Ukrainian | Українська Language Pack |
| Vietnamese | Tiếng Việt Language Pack |

Your terminal's support for bidirectional text and character shaping affects how Arabic, Hebrew, and Persian appear.

## Updates and offline use

**Update languages on upgrade** is enabled by default. When a newer TTT release starts, it checks installed packs for compatible translations in the background. If the active language is updated, TTT asks you to restart to use the new translations.

Installed translations are cached locally and work offline. If an update cannot be downloaded, TTT keeps the cached translations. Missing or outdated messages fall back to English individually, so a recently added command or setting may appear in English until the pack is updated.

To update manually, use **Plugins: Update**, then restart TTT. To turn off automatic language updates, disable **Update languages on upgrade** in Settings. See the [display language settings](/reference/settings/#display-language) for JSON configuration.

Language packs translate TTT's interface. Copy supplied by other plugins is maintained by those plugins.

## Return to English

Choose **English** with **Change Display Language** or in Settings, then restart TTT. English is always available.

Disabling a language plugin keeps its cached language available. Uninstalling the pack removes it from the picker and selects English if it was your chosen language; restart to apply the change.

## Help improve translations

The maintained packs use machine-generated translations, and corrections from native speakers are welcome. Report unclear or incorrect wording in the [ttt-plugins issue tracker](https://github.com/eugenioenko/ttt-plugins/issues), including the language, the current text, and your suggested wording.

To contribute a correction or a new language, see the [language-pack maintenance guide](https://github.com/eugenioenko/ttt-plugins/tree/main/i18n). The [plugin authoring reference](/guides/plugin-authoring/#language-packs) documents the catalog and manifest formats.
