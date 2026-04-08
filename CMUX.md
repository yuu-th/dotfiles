# はじめに — cmux docs

# はじめに

cmuxはGhosttyベースの軽量なネイティブmacOSターミナルで、複数のAIコーディングエージェントを管理するために設計されています。縦タブ、通知パネル、socketベースの制御APIを搭載しています。

## インストール

### DMG（推奨）

[Mac版をダウンロード](https://github.com/manaflow-ai/cmux/releases/latest/download/cmux-macos.dmg)

.dmgを開き、cmuxをアプリケーションフォルダにドラッグしてください。cmuxはSparkle経由で自動更新されるため、ダウンロードは一度だけで済みます。

### Homebrew

```
brew tap manaflow-ai/cmux
brew install --cask cmux
```

後で更新する場合：

```
brew upgrade --cask cmux
```

初回起動時、macOSが確認済みの開発者からのアプリを開くことの確認を求める場合があります。**開く**をクリックして続行してください。

## インストールの確認

cmuxを開くと、以下が表示されるはずです：

-   左側に縦タブサイドバーがあるターミナルウィンドウ
-   既に開かれた1つのワークスペース
-   入力可能なGhosttyベースのターミナル

## CLIセットアップ

cmuxには自動化用のコマンドラインツールが含まれています。cmuxターミナル内では自動的に動作します。cmuxの外部からCLIを使用するには、シンボリックリンクを作成してください：

```
sudo ln -sf "/Applications/cmux.app/Contents/Resources/bin/cmux" /usr/local/bin/cmux
```

これで以下のようなコマンドを実行できます：

```
cmux list-workspaces
cmux notify --title "Build Complete" --body "Your build finished"
```

## 自動更新

cmuxはSparkle経由で自動的に更新を確認します。更新が利用可能な場合、タイトルバーに更新ピルが表示されます。メニューバーのcmux > アップデートを確認から手動で確認することもできます。

## セッション復元（現在の動作）

再起動後、cmuxはレイアウトとメタデータのみを復元します：

-   ウィンドウ、ワークスペース、ペインのレイアウト
-   作業ディレクトリ
-   ターミナルのスクロールバック（ベストエフォート）
-   ブラウザのURLとナビゲーション履歴

cmuxはライブプロセスの状態はまだ復元しません。Claude Code、tmux、vimなどのアクティブなターミナルアプリセッションは、アプリの再起動後に再開されません。

## 動作要件

-   macOS 14.0以降
-   Apple SiliconまたはIntel Mac



# コンセプト — cmux docs

# コンセプト

cmuxはターミナルを4層の階層構造で管理します。これらのレベルを理解することで、socket API、CLI、キーボードショートカットの使用が容易になります。

## 階層構造

```
Window
  └── Workspace (sidebar entry)
        └── Pane (split region)
              └── Surface (tab within pane)
                    └── Panel (terminal or browser content)
```

### ウィンドウ

macOSウィンドウです。⌘⇧Nで複数のウィンドウを開けます。各ウィンドウには独立したワークスペースを持つ独自のサイドバーがあります。

### ワークスペース

サイドバーの項目です。各ワークスペースには1つ以上の分割ペインが含まれます。ワークスペースは左側のサイドバーに一覧表示されます。

UIやキーボードショートカットでは、ワークスペースはサイドバーのタブのように動作するため「タブ」と呼ばれることがあります。socket APIや環境変数では「ワークスペース」という用語が使われます。

コンテキスト

使用される用語

サイドバーUI

タブ

キーボードショートカット

ワークスペースまたはタブ

Socket API

`workspace`

環境変数

`CMUX_WORKSPACE_ID`

**ショートカット：⌘N（新規）、⌘1–⌘9（ジャンプ）、⌘⇧W（閉じる）、⌃⌘\[ / ⌃⌘\]（前後）**

### ペイン

ワークスペース内の分割領域です。⌘D（右）または⌘⇧D（下）で分割して作成します。⌥⌘ + 矢印キーでペイン間を移動します。

各ペインは複数のサーフェス（ペイン内のタブ）を持つことができます。

### サーフェス

ペイン内のタブです。各ペインには独自のタブバーがあり、複数のサーフェスを持てます。⌘Tで作成、⌘\[ / ⌘\]または⌃1–⌃9で切り替えます。

サーフェスは操作対象となる個々のターミナルまたはブラウザセッションです。各サーフェスには独自のCMUX\_SURFACE\_ID環境変数があります。

### パネル

サーフェス内のコンテンツです。現在2種類あります：

-   **ターミナル：Ghosttyターミナルセッション**
-   **ブラウザ：組み込みWebビュー**

パネルは主に内部的な概念です。socket APIやCLIでは、パネルではなくサーフェスを直接操作します。

## 視覚的な例

```
┌──────────────────────────────────────────────────────┐
│ ┌──────────┐ ┌─────────────────────────────────────┐ │
│ │ Sidebar  │ │ Workspace "dev"                     │ │
│ │          │ │                                     │ │
│ │          │ │ ┌───────────────┬─────────────────┐ │ │
│ │ > dev    │ │ │ Pane 1        │ Pane 2          │ │ │
│ │   server │ │ │ [S1] [S2]     │ [S1]            │ │ │
│ │   logs   │ │ │               │                 │ │ │
│ │          │ │ │  Terminal     │  Terminal       │ │ │
│ │          │ │ │               │                 │ │ │
│ │          │ │ └───────────────┴─────────────────┘ │ │
│ └──────────┘ └─────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

この例では：

-   ウィンドウに3つのワークスペース（dev、server、logs）があるサイドバーがあります
-   ワークスペース「dev」が選択されており、2つのペインが横に並んでいます
-   ペイン1には2つのサーフェス（タブバーの\[S1\]と\[S2\]）があり、S1がアクティブです
-   ペイン2には1つのサーフェスがあります
-   各サーフェスにはパネル（この場合はターミナル）が含まれています

## まとめ

レベル

内容

作成方法

識別方法

ウィンドウ

macOSウィンドウ

`⌘⇧N`

—

ワークスペース

サイドバーの項目

`⌘N`

`CMUX_WORKSPACE_ID`

ペイン

分割領域

`⌘D` / `⌘⇧D`

ペインID（socket API）

サーフェス

ペイン内のタブ

`⌘T`

`CMUX_SURFACE_ID`

パネル

ターミナルまたはブラウザ

自動

パネルID（内部）

# 設定 — cmux docs

# 設定

cmuxはターミナル設定をGhosttyの設定ファイルから読み込みます。cmux管理のアプリ設定も ~/.config/cmux/settings.json で管理でき、ショートカット、自動化、サイドバー、通知、ブラウザ設定を含みます。

## 設定ファイルの場所

cmuxは以下の場所から設定を検索します（順番に）：

1.  `~/.config/ghostty/config`
2.  `~/Library/Application Support/com.mitchellh.ghostty/config`

設定ファイルが存在しない場合は作成してください：

```
mkdir -p ~/.config/ghostty
touch ~/.config/ghostty/config
```

## 設定例

~/.config/ghostty/config

```
font-family = SF Mono
font-size = 13
theme = One Dark
scrollback-limit = 50000
split-divider-color = #3e4451
working-directory = ~/code
```

## cmux settings.json

cmux keeps app-owned settings in a separate user file instead of mixing them into Ghostty config. On launch, if neither settings location exists, cmux writes a commented template to `~/.config/cmux/settings.json`.

1.  `~/.config/cmux/settings.json`
2.  `~/Library/Application Support/com.cmuxterm.app/settings.json`

**Precedence:** `~/.config/cmux/settings.json` wins over the Application Support fallback. File-managed values override the value saved in the Settings window. Remove a key to fall back to the Settings value again.

**Reload:** edit the file, then use `Cmd+Shift+,` or `cmux reload-config` to re-read it without restarting the app.

**Migrations:** keep `schemaVersion` at `1` for now. Future cmux versions will use that field for upgrades. If cmux sees a newer schema version, it logs a warning and parses known keys only.

The file accepts JSON with comments and trailing commas. The canonical schema is published at [https://raw.githubusercontent.com/manaflow-ai/cmux/main/web/data/cmux-settings.schema.json](https://raw.githubusercontent.com/manaflow-ai/cmux/main/web/data/cmux-settings.schema.json) and the source lives at [https://github.com/manaflow-ai/cmux/blob/main/web/data/cmux-settings.schema.json](https://github.com/manaflow-ai/cmux/blob/main/web/data/cmux-settings.schema.json).

~/.config/cmux/settings.json

```
{
  "$schema": "https://raw.githubusercontent.com/manaflow-ai/cmux/main/web/data/cmux-settings.schema.json",
  "schemaVersion": 1,

  // "app": {
  //   "appearance": "dark",
  //   "newWorkspacePlacement": "afterCurrent"
  // },

  // "browser": {
  //   "openTerminalLinksInCmuxBrowser": true,
  //   "hostsToOpenInEmbeddedBrowser": ["localhost", "*.internal.example"]
  // },

  // "workspaceColors": {
  //   "colors": {
  //     "Red": "#C0392B",
  //     "Blue": "#1565C0",
  //     "Neon Mint": "#00F5D4"
  //   }
  // },

  // "shortcuts": {
  //   "bindings": {
  //     "toggleSidebar": "cmd+b",
  //     "newTab": ["ctrl+b", "c"]
  //   }
  // },
}
```

## Schema reference

This reference covers every supported key in `settings.json`. The embedded browser, sidebar, notifications, automation, and cmux-owned keyboard shortcuts all live here.

### Metadata

`$schema`

Optional schema URL for editor completion and validation.

Type

`string`

Default

`"https://raw.githubusercontent.com/manaflow-ai/cmux/main/web/data/cmux-settings.schema.json"`

`schemaVersion`

Schema version for forward-compatible migrations. Newer versions are parsed on a best-effort basis.

Type

`integer`

Default

`1`

### `app`

General app preferences from Settings > App.

`app.language`

Preferred app language.

Type

`string`

Default

`"system"`

Allowed values

`system, en, ar, bs, zh-Hans, zh-Hant, da, de, es, fr, it, ja, ko, nb, pl, pt-BR, ru, th, tr`

`app.appearance`

App appearance mode.

Type

`string`

Default

`"system"`

Allowed values

`system, light, dark`

`app.appIcon`

Dock and app switcher icon style.

Type

`string`

Default

`"automatic"`

Allowed values

`automatic, light, dark`

`app.newWorkspacePlacement`

Where new workspaces are inserted in the sidebar.

Type

`string`

Default

`"afterCurrent"`

Allowed values

`top, afterCurrent, end`

`app.minimalMode`

Hide the workspace title bar and move controls into the sidebar.

Type

`boolean`

Default

`false`

`app.keepWorkspaceOpenWhenClosingLastSurface`

When true, closing the last surface keeps the workspace open.

Type

`boolean`

Default

`false`

`app.focusPaneOnFirstClick`

When cmux is inactive, the first click can activate and focus the clicked pane.

Type

`boolean`

Default

`true`

`app.preferredEditor`

Custom editor command used by cmux where applicable. Leave empty to use the default.

Type

`string`

Default

`""`

`app.reorderOnNotification`

Move workspaces with new notifications toward the top.

Type

`boolean`

Default

`true`

`app.sendAnonymousTelemetry`

Allow anonymous telemetry.

Type

`boolean`

Default

`true`

`app.warnBeforeQuit`

Show a confirmation before quitting cmux.

Type

`boolean`

Default

`true`

`app.renameSelectsExistingName`

Select the current name when opening rename flows.

Type

`boolean`

Default

`true`

`app.commandPaletteSearchesAllSurfaces`

Search every surface in the command palette switcher instead of only the active workspace.

Type

`boolean`

Default

`false`

### `notifications`

Notification behavior from Settings > Notifications.

`notifications.dockBadge`

Show the unread count in the Dock tile.

Type

`boolean`

Default

`true`

`notifications.showInMenuBar`

Show the menu bar extra.

Type

`boolean`

Default

`true`

`notifications.unreadPaneRing`

Highlight panes with unread notifications.

Type

`boolean`

Default

`true`

`notifications.paneFlash`

Flash the focused pane when requested.

Type

`boolean`

Default

`true`

`notifications.sound`

Notification sound preset.

Type

`string`

Default

`"default"`

Allowed values

`default, Basso, Blow, Bottle, Frog, Funk, Glass, Hero, Morse, Ping, Pop, Purr, Sosumi, Submarine, Tink, custom_file, none`

`notifications.customSoundFilePath`

Local path to the custom notification sound file.

Type

`string`

Default

`""`

`notifications.command`

Optional shell command to run alongside notification delivery.

Type

`string`

Default

`""`

### `sidebar`

Sidebar content and metadata visibility from Settings > Sidebar.

`sidebar.hideAllDetails`

Hide all per-workspace detail rows.

Type

`boolean`

Default

`false`

`sidebar.branchLayout`

Show git branch details stacked vertically or inline.

Type

`string`

Default

`"vertical"`

Allowed values

`vertical, inline`

`sidebar.showNotificationMessage`

Show the latest notification text in the sidebar.

Type

`boolean`

Default

`true`

`sidebar.showBranchDirectory`

Show the workspace working directory.

Type

`boolean`

Default

`true`

`sidebar.showPullRequests`

Show pull request metadata in the sidebar.

Type

`boolean`

Default

`true`

`sidebar.openPullRequestLinksInCmuxBrowser`

Open sidebar pull request links in the embedded cmux browser.

Type

`boolean`

Default

`true`

`sidebar.openPortLinksInCmuxBrowser`

Open sidebar port links in the embedded cmux browser.

Type

`boolean`

Default

`true`

`sidebar.showSSH`

Show SSH connection details.

Type

`boolean`

Default

`true`

`sidebar.showPorts`

Show listening ports.

Type

`boolean`

Default

`true`

`sidebar.showLog`

Show recent log snippets.

Type

`boolean`

Default

`true`

`sidebar.showProgress`

Show progress indicators.

Type

`boolean`

Default

`true`

`sidebar.showCustomMetadata`

Show custom metadata pills.

Type

`boolean`

Default

`true`

### `workspaceColors`

Workspace tab and badge colors from Settings > Workspace Colors.

`workspaceColors.indicatorStyle`

Active workspace indicator style. Legacy aliases are accepted and normalized.

Type

`string`

Default

`"leftRail"`

Allowed values

`leftRail, solidFill, rail, border, wash, lift, typography, washRail, blueWashColorRail`

`workspaceColors.selectionColor`

Override the selected workspace background color.

Type

`unknown`

Default

`null`

`workspaceColors.notificationBadgeColor`

Override the unread notification badge color.

Type

`unknown`

Default

`null`

`workspaceColors.colors`

Full named workspace color palette. Include built-in entries you want to keep, remove keys to remove colors, and add more named entries to extend the picker.

Type

`object`

Default

```
{
  "Red": "#C0392B",
  "Crimson": "#922B21",
  "Orange": "#A04000",
  "Amber": "#7D6608",
  "Olive": "#4A5C18",
  "Green": "#196F3D",
  "Teal": "#006B6B",
  "Aqua": "#0E6B8C",
  "Blue": "#1565C0",
  "Navy": "#1A5276",
  "Indigo": "#283593",
  "Purple": "#6A1B9A",
  "Magenta": "#AD1457",
  "Rose": "#880E4F",
  "Brown": "#7B3F00",
  "Charcoal": "#3E4B5E"
}
```

`workspaceColors.colors` is the full palette. Keep the built-in keys you want, delete keys to remove colors from the picker, and add more named color entries to extend it. Older `paletteOverrides` and `customColors` files still parse during upgrades, but new files should use `colors`.

```
{
  "workspaceColors": {
    "colors": {
      "Red": "#C0392B",
      "Blue": "#1565C0",
      "Neon Mint": "#00F5D4"
    }
  }
}
```

### `sidebarAppearance`

Sidebar tint settings from Settings > Sidebar Appearance.

`sidebarAppearance.matchTerminalBackground`

Use the terminal background instead of the sidebar tint.

Type

`boolean`

Default

`false`

`sidebarAppearance.tintColor`

Base sidebar tint color used when light/dark overrides are not set.

Type

`unknown`

Default

`"#000000"`

`sidebarAppearance.lightModeTintColor`

Sidebar tint override for light appearance.

Type

`unknown`

Default

`null`

`sidebarAppearance.darkModeTintColor`

Sidebar tint override for dark appearance.

Type

`unknown`

Default

`null`

`sidebarAppearance.tintOpacity`

Sidebar tint opacity from 0 to 1.

Type

`number`

Default

`0.03`

### `automation`

Socket control and automation settings from Settings > Automation.

`automation.socketControlMode`

Socket control mode. Legacy aliases are accepted and normalized.

Type

`string`

Default

`"cmuxOnly"`

Allowed values

`off, cmuxOnly, automation, password, allowAll, openAccess, fullOpenAccess, notifications, full`

`automation.socketPassword`

Password for password-mode socket access. Use null or an empty string to clear it.

Type

`string | null`

Default

`""`

`automation.claudeCodeIntegration`

Enable cmux integration hooks for Claude Code.

Type

`boolean`

Default

`true`

`automation.claudeBinaryPath`

Custom path to the claude binary.

Type

`string`

Default

`""`

`automation.portBase`

Starting value for workspace CMUX\_PORT assignments.

Type

`integer`

Default

`9100`

`automation.portRange`

Number of ports reserved per workspace.

Type

`integer`

Default

`10`

### `customCommands`

Custom command trust settings from Settings > Custom Commands.

`customCommands.trustedDirectories`

Directories whose cmux.json commands can run without confirmation.

Type

`array<string>`

Default

```
[]
```

### `browser`

Embedded browser settings from Settings > Browser.

`browser.defaultSearchEngine`

Default search engine for non-URL queries.

Type

`string`

Default

`"google"`

Allowed values

`google, duckduckgo, bing, kagi, startpage`

`browser.showSearchSuggestions`

Show omnibar search suggestions.

Type

`boolean`

Default

`true`

`browser.theme`

Embedded browser theme.

Type

`string`

Default

`"system"`

Allowed values

`system, light, dark`

`browser.openTerminalLinksInCmuxBrowser`

Open clicked terminal links in the embedded browser.

Type

`boolean`

Default

`true`

`browser.interceptTerminalOpenCommandInCmuxBrowser`

Intercept terminal open http(s) commands and route them through the embedded browser.

Type

`boolean`

Default

`true`

`browser.hostsToOpenInEmbeddedBrowser`

Allowlist of hosts that should stay inside the embedded browser.

Type

`array<string>`

Default

```
[]
```

`browser.urlsToAlwaysOpenExternally`

Rules that always open matching URLs in the system browser.

Type

`array<string>`

Default

```
[]
```

`browser.insecureHttpHostsAllowedInEmbeddedBrowser`

HTTP hosts allowed in the embedded browser without a warning prompt.

Type

`array<string>`

Default

```
[
  "localhost",
  "127.0.0.1",
  "::1",
  "0.0.0.0",
  "*.localtest.me"
]
```

`browser.showImportHintOnBlankTabs`

Show the browser import hint on blank tabs.

Type

`boolean`

Default

`true`

`browser.reactGrabVersion`

Pinned react-grab version for the browser toolbar helper.

Type

`string`

Default

`"0.1.29"`

### `shortcuts`

Keyboard shortcut settings from Settings > Keyboard Shortcuts.

`shortcuts.showModifierHoldHints`

Show shortcut hint pills while holding Cmd or Ctrl.

Type

`boolean`

Default

`true`

### `shortcuts.bindings`

Use a string for a single shortcut, or a two-item array for a chord. Example: `["ctrl+b", "c"]`. Numbered actions use `1` as the stored default and still match digits `1` through `9`.

The defaults below are the same cmux-owned actions listed on the [keyboard shortcuts page](/ja/docs/keyboard-shortcuts).

#### アプリ

`openSettings`

設定

Default file value

`cmd+,`

`reloadConfiguration`

構成を再読み込み

Default file value

`cmd+shift+,`

`showHideAllWindows`

すべてのcmuxウインドウを表示/非表示システム全体のホットキー

Default file value

`ctrl+opt+cmd+.`

`commandPalette`

コマンドパレット

Default file value

`cmd+shift+p`

`newWindow`

新規ウインドウ

Default file value

`cmd+shift+n`

`closeWindow`

ウインドウを閉じる

Default file value

`ctrl+cmd+w`

`toggleFullScreen`

フルスクリーンを切り替え

Default file value

`ctrl+cmd+f`

`sendFeedback`

フィードバックを送信

Default file value

`opt+cmd+f`

`quit`

cmuxを終了

Default file value

`cmd+q`

#### ワークスペース

`toggleSidebar`

サイドバーを切り替え

Default file value

`cmd+b`

`newTab`

新規ワークスペース

Default file value

`cmd+n`

`openFolder`

フォルダを開く

Default file value

`cmd+o`

`goToWorkspace`

ワークスペースへ移動ワークスペーススイッチャー

Default file value

`cmd+p`

`nextSidebarTab`

次のワークスペース

Default file value

`ctrl+cmd+]`

`prevSidebarTab`

前のワークスペース

Default file value

`ctrl+cmd+[`

`selectWorkspaceByNumber`

ワークスペース1…9を選択

Default file value

`cmd+1`

`renameWorkspace`

ワークスペース名を変更

Default file value

`cmd+shift+r`

`closeWorkspace`

ワークスペースを閉じる

Default file value

`cmd+shift+w`

#### サーフェス

`newSurface`

新規サーフェス

Default file value

`cmd+t`

`nextSurface`

次のサーフェス

Default file value

`cmd+shift+]`

`prevSurface`

前のサーフェス

Default file value

`cmd+shift+[`

`selectSurfaceByNumber`

サーフェス1…9を選択

Default file value

`ctrl+1`

`renameTab`

タブ名を変更

Default file value

`cmd+r`

`closeTab`

タブを閉じる

Default file value

`cmd+w`

`closeOtherTabsInPane`

ペイン内の他のタブを閉じる

Default file value

`opt+cmd+t`

`reopenClosedBrowserPanel`

閉じたブラウザパネルを再度開く

Default file value

`cmd+shift+t`

`toggleTerminalCopyMode`

ターミナルコピーモードを切り替え

Default file value

`cmd+shift+m`

#### 分割ペイン

`focusLeft`

左のペインにフォーカス

Default file value

`opt+cmd+left`

`focusRight`

右のペインにフォーカス

Default file value

`opt+cmd+right`

`focusUp`

上のペインにフォーカス

Default file value

`opt+cmd+up`

`focusDown`

下のペインにフォーカス

Default file value

`opt+cmd+down`

`splitRight`

右に分割

Default file value

`cmd+d`

`splitDown`

下に分割

Default file value

`cmd+shift+d`

`splitBrowserRight`

右にブラウザ分割

Default file value

`opt+cmd+d`

`splitBrowserDown`

下にブラウザ分割

Default file value

`opt+cmd+shift+d`

`toggleSplitZoom`

ペインズームを切り替え

Default file value

`cmd+shift+enter`

#### ブラウザ

`openBrowser`

ブラウザを開く

Default file value

`cmd+shift+l`

`focusBrowserAddressBar`

アドレスバーにフォーカス

Default file value

`cmd+l`

`browserBack`

戻る

Default file value

`cmd+[`

`browserForward`

進む

Default file value

`cmd+]`

`browserReload`

ページを再読み込みフォーカス中のブラウザ

Default file value

`cmd+r`

`browserZoomIn`

拡大

Default file value

`cmd+=`

`browserZoomOut`

縮小

Default file value

`cmd+-`

`browserZoomReset`

実寸表示

Default file value

`cmd+0`

`toggleBrowserDeveloperTools`

ブラウザ開発者ツールを切り替え

Default file value

`opt+cmd+i`

`showBrowserJavaScriptConsole`

ブラウザJavaScriptコンソールを表示

Default file value

`opt+cmd+c`

`toggleReactGrab`

React Grabを切り替えフォーカス中のブラウザ、またはターミナルにフォーカスがあるときは唯一のブラウザペイン

Default file value

`cmd+shift+g`

#### 検索

`find`

検索

Default file value

`cmd+f`

`findNext`

次を検索

Default file value

`cmd+g`

`findPrevious`

前を検索

Default file value

`opt+cmd+g`

`hideFind`

検索バーを隠す

Default file value

`cmd+shift+f`

`useSelectionForFind`

選択範囲で検索

Default file value

`cmd+e`

#### 通知

`showNotifications`

通知を表示

Default file value

`cmd+i`

`jumpToUnread`

最新の未読へ移動

Default file value

`cmd+shift+u`

`triggerFlash`

フォーカス中のパネルをフラッシュ

Default file value

`cmd+shift+h`


# カスタムコマンド — cmux docs

# カスタムコマンド

プロジェクトルートまたは ~/.config/cmux/ に cmux.json ファイルを追加してカスタムコマンドとワークスペースレイアウトを定義します。コマンドはコマンドパレットに表示されます。

## ファイルの場所

cmux は2か所で設定を検索します：

-   **プロジェクトごと：** `./cmux.json` — プロジェクトディレクトリに置かれ、優先されます
-   **グローバル：** `~/.config/cmux/cmux.json` — すべてのプロジェクトに適用され、ローカルで未定義のコマンドを補完します

ローカルコマンドは同名のグローバルコマンドを上書きします。

変更は自動的に反映されます — 再起動は不要です。

## スキーマ

cmux.json ファイルには commands 配列が含まれます。各コマンドはシンプルなシェルコマンドまたは完全なワークスペース定義です：

cmux.json

```
{
  "commands": [
    {
      "name": "Start Dev",
      "keywords": ["dev", "start"],
      "workspace": { ... }
    },
    {
      "name": "Run Tests",
      "command": "npm test",
      "confirm": true
    }
  ]
}
```

## シンプルコマンド

シンプルコマンドは現在フォーカスされているターミナルでシェルコマンドを実行します：

cmux.json

```
{
  "commands": [
    {
      "name": "Run Tests",
      "keywords": ["test", "check"],
      "command": "npm test",
      "confirm": true
    }
  ]
}
```

### フィールド

-   `name` — コマンドパレットに表示されます（必須）
-   `description` — 任意の説明
-   `keywords` — コマンドパレット用の追加検索キーワード
-   `command` — フォーカスされたターミナルで実行するシェルコマンド
-   `confirm` — 実行前に確認ダイアログを表示する

シンプルコマンドはフォーカスされたターミナルの現在の作業ディレクトリで実行されます。プロジェクト相対パスに依存するコマンドの場合は、先頭に `cd "$(git rev-parse --show-toplevel)" &&` を付けてリポジトリのルートから実行するか、 `cd /your/path &&` で任意のディレクトリを指定できます。

## ワークスペースコマンド

ワークスペースコマンドは、分割、ターミナル、ブラウザペインのカスタムレイアウトで新しいワークスペースを作成します：

cmux.json

```
{
  "commands": [
    {
      "name": "Dev Environment",
      "keywords": ["dev", "fullstack"],
      "restart": "confirm",
      "workspace": {
        "name": "Dev",
        "cwd": ".",
        "layout": {
          "direction": "horizontal",
          "split": 0.5,
          "children": [
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "Frontend",
                    "command": "npm run dev",
                    "focus": true
                  }
                ]
              }
            },
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "Backend",
                    "command": "cargo watch -x run",
                    "cwd": "./server",
                    "env": { "RUST_LOG": "debug" }
                  }
                ]
              }
            }
          ]
        }
      }
    }
  ]
}
```

### ワークスペースフィールド

-   `name` — ワークスペースのタブ名（デフォルトはコマンド名）
-   `cwd` — ワークスペースの作業ディレクトリ
-   `color` — ワークスペースのタブカラー
-   `layout` — 分割とペインを定義するレイアウトツリー

### 再起動の動作

同名のワークスペースが既に存在する場合の動作を制御します：

-   `"ignore"` — 既存のワークスペースに切り替える（デフォルト）
-   `"recreate"` — 確認なしに閉じて再作成する
-   `"confirm"` — 再作成前にユーザーに確認する

## レイアウトツリー

レイアウトツリーは、再帰的な分割ノードを使用してペインの配置を定義します：

### 分割ノード

スペースを2つの子に分割します：

-   `direction` — `"horizontal"` または `"vertical"`
-   `split` — 分割位置（0.1〜0.9、デフォルト0.5）
-   `children` — 正確に2つの子ノード（分割またはペイン）

### ペインノード

1つ以上のサーフェス（ペイン内のタブ）を含むリーフノード。

## サーフェス定義

ペイン内の各サーフェスはターミナルまたはブラウザです：

-   `type` — `"terminal"` または `"browser"`
-   `name` — カスタムタブタイトル
-   `command` — 作成時に自動実行するシェルコマンド（ターミナルのみ）
-   `cwd` — このサーフェスの作業ディレクトリ
-   `env` — キーと値のペアとしての環境変数
-   `url` — 開くURL（ブラウザのみ）
-   `focus` — 作成後にこのサーフェスにフォーカスする

### 作業ディレクトリの解決

-   `.` または 省略 — ワークスペースの作業ディレクトリ
-   `./subdir` — ワークスペースの作業ディレクトリからの相対パス
-   `~/path` — ホームディレクトリに展開
-   絶対パス — そのまま使用

## 完全な例

cmux.json

```
{
  "commands": [
    {
      "name": "Web Dev",
      "description": "Docs site with live preview",
      "keywords": ["web", "docs", "next", "frontend"],
      "restart": "confirm",
      "workspace": {
        "name": "Web Dev",
        "cwd": "./web",
        "color": "#3b82f6",
        "layout": {
          "direction": "horizontal",
          "split": 0.5,
          "children": [
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "Next.js",
                    "command": "npm run dev",
                    "focus": true
                  }
                ]
              }
            },
            {
              "direction": "vertical",
              "split": 0.6,
              "children": [
                {
                  "pane": {
                    "surfaces": [
                      {
                        "type": "browser",
                        "name": "Preview",
                        "url": "http://localhost:3777"
                      }
                    ]
                  }
                },
                {
                  "pane": {
                    "surfaces": [
                      {
                        "type": "terminal",
                        "name": "Shell",
                        "env": { "NODE_ENV": "development" }
                      }
                    ]
                  }
                }
              ]
            }
          ]
        }
      }
    },
    {
      "name": "Debug Log",
      "description": "Tail the debug event log from the running dev app",
      "keywords": ["log", "debug", "tail", "events"],
      "restart": "ignore",
      "workspace": {
        "name": "Debug Log",
        "layout": {
          "direction": "horizontal",
          "split": 0.5,
          "children": [
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "Events",
                    "command": "tail -f /tmp/cmux-debug.log",
                    "focus": true
                  }
                ]
              }
            },
            {
              "pane": {
                "surfaces": [
                  {
                    "type": "terminal",
                    "name": "Shell"
                  }
                ]
              }
            }
          ]
        }
      }
    },
    {
      "name": "Setup",
      "description": "Initialize submodules and build dependencies",
      "keywords": ["setup", "init", "install"],
      "command": "./scripts/setup.sh",
      "confirm": true
    },
    {
      "name": "Reload",
      "description": "Build and launch the debug app tagged to the current branch",
      "keywords": ["reload", "build", "run", "launch"],
      "command": "./scripts/reload.sh --tag $(git branch --show-current)"
    },
    {
      "name": "Run Unit Tests",
      "keywords": ["test", "unit"],
      "command": "./scripts/test-unit.sh",
      "confirm": true
    }
  ]
}
```


# キーボードショートカット — cmux docs

# キーボードショートカット

cmuxのデフォルトショートカット一覧です。cmux管理のショートカットはすべて設定画面または ~/.config/cmux/settings.json で変更でき、2段階のコードにも対応しています。

## ショートカットコード

cmux は `~/.config/cmux/settings.json` で2段階のショートカットコードを定義できます。設定ファイル全体の仕様は [設定ドキュメント](/ja/docs/configuration) を参照してください。

ショートカットは設定画面でも編集できますが、tmux 風のプレフィックスを正確に書きたい場合や dotfiles で管理したい場合は settings.json が分かりやすい方法です。

settings.json

```
{
  "shortcuts": {
    "bindings": {
      "newSurface": ["ctrl+b", "c"],
      "showNotifications": ["ctrl+b", "i"],
      "toggleSidebar": "cmd+b"
    }
  }
}
```

-   1回のショートカットは文字列で指定します。
-   コードは2要素の配列で指定します。1つ目がプレフィックス、2つ目が続けて押すキーです。
-   各要素は通常のショートカットと同じ構文で、例: cmd+b、ctrl+b、shift+/、ctrl+1。

アプリ

アプリ全体、ウインドウ、上位コマンドのショートカットです。

設定

⌘+,

構成を再読み込み

⌘+⇧+,

すべてのcmuxウインドウを表示/非表示システム全体のホットキー

⌃+⌥+⌘+.

コマンドパレット

⌘+⇧+P

新規ウインドウ

⌘+⇧+N

ウインドウを閉じる

⌃+⌘+W

フルスクリーンを切り替え

⌃+⌘+F

フィードバックを送信

⌥+⌘+F

cmuxを終了

⌘+Q

ワークスペース

ワークスペースはサイドバーに表示されます。各ワークスペースには独自のペインとサーフェスがあります。

サイドバーを切り替え

⌘+B

新規ワークスペース

⌘+N

フォルダを開く

⌘+O

ワークスペースへ移動ワークスペーススイッチャー

⌘+P

次のワークスペース

⌃+⌘+\]

前のワークスペース

⌃+⌘+\[

ワークスペース1…9を選択

⌘+1…9

ワークスペース名を変更

⌘+⇧+R

ワークスペースを閉じる

⌘+⇧+W

サーフェス

サーフェスはペイン内のタブです。

新規サーフェス

⌘+T

次のサーフェス

⌘+⇧+\]

前のサーフェス

⌘+⇧+\[

サーフェス1…9を選択

⌃+1…9

タブ名を変更

⌘+R

タブを閉じる

⌘+W

ペイン内の他のタブを閉じる

⌥+⌘+T

閉じたブラウザパネルを再度開く

⌘+⇧+T

ターミナルコピーモードを切り替え

⌘+⇧+M

分割ペイン

左のペインにフォーカス

⌥+⌘+←

右のペインにフォーカス

⌥+⌘+→

上のペインにフォーカス

⌥+⌘+↑

下のペインにフォーカス

⌥+⌘+↓

右に分割

⌘+D

下に分割

⌘+⇧+D

右にブラウザ分割

⌥+⌘+D

下にブラウザ分割

⌥+⌘+⇧+D

ペインズームを切り替え

⌘+⇧+↩

ブラウザ

ブラウザを開く

⌘+⇧+L

アドレスバーにフォーカス

⌘+L

戻る

⌘+\[

進む

⌘+\]

ページを再読み込みフォーカス中のブラウザ

⌘+R

拡大

⌘+\=

縮小

⌘+\-

実寸表示

⌘+0

ブラウザ開発者ツールを切り替え

⌥+⌘+I

ブラウザJavaScriptコンソールを表示

⌥+⌘+C

React Grabを切り替えフォーカス中のブラウザ、またはターミナルにフォーカスがあるときは唯一のブラウザペイン

⌘+⇧+G

検索

検索

⌘+F

次を検索

⌘+G

前を検索

⌥+⌘+G

検索バーを隠す

⌘+⇧+F

選択範囲で検索

⌘+E

通知

通知を表示

⌘+I

最新の未読へ移動

⌘+⇧+U

フォーカス中のパネルをフラッシュ

⌘+⇧+H



# APIリファレンス — cmux docs

# APIリファレンス

cmuxはCLIツールとUnix socketの両方をプログラム制御に提供します。すべてのコマンドは両方のインターフェースから利用可能です。

## Socket

ビルド

パス

リリース

`/tmp/cmux.sock`

デバッグ

`/tmp/cmux-debug.sock`

タグ付きデバッグビルド

`/tmp/cmux-debug-<tag>.sock`

CMUX\_SOCKET\_PATH環境変数で上書きできます。呼び出しごとに1つの改行区切りJSONリクエストを送信します：

```
{"id":"req-1","method":"workspace.list","params":{}}
// Response:
{"id":"req-1","ok":true,"result":{"workspaces":[...]}}
```

JSON socketリクエストにはmethodとparamsを使用する必要があります。`{"command":"..."}`などのレガシーv1 JSONペイロードはサポートされていません。

## アクセスモード

モード

説明

有効化方法

**Off**

Socketを無効化

設定UIまたはCMUX\_SOCKET\_MODE=off

**cmux processes only**

cmuxターミナル内で起動されたプロセスのみ接続可能。

設定UIのデフォルトモード

**allowAll**

ローカルプロセスすべての接続を許可（祖先チェックなし）。

環境変数のみで設定：CMUX\_SOCKET\_MODE=allowAll

共有マシンではオフまたはcmuxプロセスのみを使用してください。

## CLIオプション

フラグ

説明

`--socket PATH`

カスタムsocketパス

`--json`

JSON形式で出力

`--window ID`

特定のウィンドウを対象にする

`--workspace ID`

特定のワークスペースを対象にする

`--surface ID`

特定のサーフェスを対象にする

`--id-format refs|uuids|both`

JSON出力での識別子フォーマットを制御

## ワークスペースコマンド

#### list-workspaces

すべてのワークスペースを一覧表示。

CLI

```
cmux list-workspaces
cmux list-workspaces --json
```

Socket

```
{"id":"ws-list","method":"workspace.list","params":{}}
```

#### new-workspace

新しいワークスペースを作成。

CLI

```
cmux new-workspace
```

Socket

```
{"id":"ws-new","method":"workspace.create","params":{}}
```

#### select-workspace

特定のワークスペースに切り替え。

CLI

```
cmux select-workspace --workspace <id>
```

Socket

```
{"id":"ws-select","method":"workspace.select","params":{"workspace_id":"<id>"}}
```

#### current-workspace

現在アクティブなワークスペースを取得。

CLI

```
cmux current-workspace
cmux current-workspace --json
```

Socket

```
{"id":"ws-current","method":"workspace.current","params":{}}
```

#### close-workspace

ワークスペースを閉じる。

CLI

```
cmux close-workspace --workspace <id>
```

Socket

```
{"id":"ws-close","method":"workspace.close","params":{"workspace_id":"<id>"}}
```

## 分割コマンド

#### new-split

新しい分割ペインを作成。方向：left、right、up、down。

CLI

```
cmux new-split right
cmux new-split down
```

Socket

```
{"id":"split-new","method":"surface.split","params":{"direction":"right"}}
```

#### list-surfaces

現在のワークスペースのすべてのサーフェスを一覧表示。

CLI

```
cmux list-surfaces
cmux list-surfaces --json
```

Socket

```
{"id":"surface-list","method":"surface.list","params":{}}
```

#### focus-surface

特定のサーフェスにフォーカス。

CLI

```
cmux focus-surface --surface <id>
```

Socket

```
{"id":"surface-focus","method":"surface.focus","params":{"surface_id":"<id>"}}
```

## 入力コマンド

#### send

フォーカス中のターミナルにテキスト入力を送信。

CLI

```
cmux send "echo hello"
cmux send "ls -la\n"
```

Socket

```
{"id":"send-text","method":"surface.send_text","params":{"text":"echo hello\n"}}
```

#### send-key

キー入力を送信。キー：enter、tab、escape、backspace、delete、up、down、left、right。

CLI

```
cmux send-key enter
```

Socket

```
{"id":"send-key","method":"surface.send_key","params":{"key":"enter"}}
```

#### send-surface

特定のサーフェスにテキストを送信。

CLI

```
cmux send-surface --surface <id> "command"
```

Socket

```
{"id":"send-surface","method":"surface.send_text","params":{"surface_id":"<id>","text":"command"}}
```

#### send-key-surface

特定のサーフェスにキー入力を送信。

CLI

```
cmux send-key-surface --surface <id> enter
```

Socket

```
{"id":"send-key-surface","method":"surface.send_key","params":{"surface_id":"<id>","key":"enter"}}
```

## 通知コマンド

#### notify

通知を送信。

CLI

```
cmux notify --title "Title" --body "Body"
cmux notify --title "T" --subtitle "S" --body "B"
```

Socket

```
{"id":"notify","method":"notification.create","params":{"title":"Title","subtitle":"S","body":"Body"}}
```

#### list-notifications

すべての通知を一覧表示。

CLI

```
cmux list-notifications
cmux list-notifications --json
```

Socket

```
{"id":"notif-list","method":"notification.list","params":{}}
```

#### clear-notifications

すべての通知をクリア。

CLI

```
cmux clear-notifications
```

Socket

```
{"id":"notif-clear","method":"notification.clear","params":{}}
```

## サイドバーメタデータコマンド

任意のワークスペースのサイドバーにステータスピル、プログレスバー、ログエントリを設定します。ビルドスクリプト、CI連携、状態を一目で確認したいAIコーディングエージェントに便利です。

#### set-status

サイドバーのステータスピルを設定。ツールごとに独自のエントリを管理できるよう、一意のキーを使用してください。

CLI

```
cmux set-status build "compiling" --icon hammer --color "#ff9500"
cmux set-status deploy "v1.2.3" --workspace workspace:2
```

Socket

```
set_status build compiling --icon=hammer --color=#ff9500 --tab=<workspace-uuid>
```

#### clear-status

キーを指定してサイドバーのステータスエントリを削除。

CLI

```
cmux clear-status build
```

Socket

```
clear_status build --tab=<workspace-uuid>
```

#### list-status

ワークスペースのすべてのサイドバーステータスエントリを一覧表示。

CLI

```
cmux list-status
```

Socket

```
list_status --tab=<workspace-uuid>
```

#### set-progress

サイドバーにプログレスバーを設定（0.0〜1.0）。

CLI

```
cmux set-progress 0.5 --label "Building..."
cmux set-progress 1.0 --label "Done"
```

Socket

```
set_progress 0.5 --label=Building... --tab=<workspace-uuid>
```

#### clear-progress

サイドバーのプログレスバーをクリア。

CLI

```
cmux clear-progress
```

Socket

```
clear_progress --tab=<workspace-uuid>
```

#### log

サイドバーにログエントリを追加。レベル：info、progress、success、warning、error。

CLI

```
cmux log "Build started"
cmux log --level error --source build "Compilation failed"
cmux log --level success -- "All 42 tests passed"
```

Socket

```
log --level=error --source=build --tab=<workspace-uuid> -- Compilation failed
```

#### clear-log

すべてのサイドバーログエントリをクリア。

CLI

```
cmux clear-log
```

Socket

```
clear_log --tab=<workspace-uuid>
```

#### list-log

サイドバーのログエントリを一覧表示。

CLI

```
cmux list-log
cmux list-log --limit 5
```

Socket

```
list_log --limit=5 --tab=<workspace-uuid>
```

#### sidebar-state

すべてのサイドバーメタデータをダンプ（cwd、gitブランチ、ポート、ステータス、プログレス、ログ）。

CLI

```
cmux sidebar-state
cmux sidebar-state --workspace workspace:2
```

Socket

```
sidebar_state --tab=<workspace-uuid>
```

## ユーティリティコマンド

#### ping

cmuxが実行中で応答可能か確認。

CLI

```
cmux ping
```

Socket

```
{"id":"ping","method":"system.ping","params":{}}
// Response: {"id":"ping","ok":true,"result":{"pong":true}}
```

#### capabilities

利用可能なsocketメソッドと現在のアクセスモードを一覧表示。

CLI

```
cmux capabilities
cmux capabilities --json
```

Socket

```
{"id":"caps","method":"system.capabilities","params":{}}
```

#### identify

フォーカス中のウィンドウ/ワークスペース/ペイン/サーフェスのコンテキストを表示。

CLI

```
cmux identify
cmux identify --json
```

Socket

```
{"id":"identify","method":"system.identify","params":{}}
```

## 環境変数

変数

説明

`CMUX_SOCKET_PATH`

CLIや連携ツールが使用するsocketパスを上書き

`CMUX_SOCKET_ENABLE`

socketを強制的に有効/無効化（1/0、true/false、on/off）

`CMUX_SOCKET_MODE`

アクセスモードを上書き（cmuxOnly、allowAll、off）。cmux-only/cmux\_onlyやallow-all/allow\_allも使用可能

`CMUX_WORKSPACE_ID`

自動設定：現在のワークスペースID

`CMUX_SURFACE_ID`

自動設定：現在のサーフェスID

`TERM_PROGRAM`

ghosttyに設定

`TERM`

xterm-ghosttyに設定

レガシーのCMUX\_SOCKET\_MODE値fullとnotificationsは互換性のため引き続き使用可能です。

## cmuxの検出

bash

```
# Prefer explicit socket path if set
SOCK="${CMUX_SOCKET_PATH:-/tmp/cmux.sock}"
[ -S "$SOCK" ] && echo "Socket available"

# Check for the CLI
command -v cmux &>/dev/null && echo "cmux available"

# In cmux-managed terminals these are auto-set
[ -n "${CMUX_WORKSPACE_ID:-}" ] && [ -n "${CMUX_SURFACE_ID:-}" ] && echo "Inside cmux surface"

# Distinguish from regular Ghostty
[ "$TERM_PROGRAM" = "ghostty" ] && [ -n "${CMUX_WORKSPACE_ID:-}" ] && echo "In cmux"
```

## 使用例

### Pythonクライアント

python

```
import json
import os
import socket

SOCKET_PATH = os.environ.get("CMUX_SOCKET_PATH", "/tmp/cmux.sock")

def rpc(method, params=None, req_id=1):
    payload = {"id": req_id, "method": method, "params": params or {}}
    with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
        sock.connect(SOCKET_PATH)
        sock.sendall(json.dumps(payload).encode("utf-8") + b"\n")
        return json.loads(sock.recv(65536).decode("utf-8"))

# List workspaces
print(rpc("workspace.list", req_id="ws"))

# Send notification
print(rpc(
    "notification.create",
    {"title": "Hello", "body": "From Python!"},
    req_id="notify"
))
```

### シェルスクリプト

bash

```
#!/bin/bash
SOCK="${CMUX_SOCKET_PATH:-/tmp/cmux.sock}"

cmux_cmd() {
    printf "%s\n" "$1" | nc -U "$SOCK"
}

cmux_cmd '{"id":"ws","method":"workspace.list","params":{}}'
cmux_cmd '{"id":"notify","method":"notification.create","params":{"title":"Done","body":"Task complete"}}'
```

### 通知付きビルドスクリプト

bash

```
#!/bin/bash
npm run build
if [ $? -eq 0 ]; then
    cmux notify --title "✓ Build Success" --body "Ready to deploy"
else
    cmux notify --title "✗ Build Failed" --body "Check the logs"
fi
```


# ブラウザ自動化 — cmux docs

# ブラウザ自動化

cmux browserコマンドグループは、cmuxブラウザサーフェスに対するブラウザ自動化を提供します。ナビゲーション、DOM要素の操作、ページ状態のインスペクション、JavaScriptの実行、ブラウザセッションデータの管理に使用できます。

## コマンド一覧

カテゴリ

サブコマンド

ナビゲーションとターゲティング

`identify`, `open`, `open-split`, `navigate`, `back`, `forward`, `reload`, `url`, `focus-webview`, `is-webview-focused`

待機

`wait`

DOM操作

`click`, `dblclick`, `hover`, `focus`, `check`, `uncheck`, `scroll-into-view`, `type`, `fill`, `press`, `keydown`, `keyup`, `select`, `scroll`

インスペクション

`snapshot`, `screenshot`, `get`, `is`, `find`, `highlight`

JavaScriptと注入

`eval`, `addinitscript`, `addscript`, `addstyle`

フレーム、ダイアログ、ダウンロード

`frame`, `dialog`, `download`

状態とセッションデータ

`cookies`, `storage`, `state`

タブとログ

`tab`, `console`, `errors`

## ブラウザサーフェスの指定

ほとんどのサブコマンドにはターゲットサーフェスが必要です。位置引数または--surfaceで指定できます。

```
# Open a new browser split
cmux browser open https://example.com

# Discover focused IDs and browser metadata
cmux browser identify
cmux browser identify --surface surface:2

# Positional vs flag targeting are equivalent
cmux browser surface:2 url
cmux browser --surface surface:2 url
```

## ナビゲーション

```
cmux browser open https://example.com
cmux browser open-split https://news.ycombinator.com

cmux browser surface:2 navigate https://example.org/docs --snapshot-after
cmux browser surface:2 back
cmux browser surface:2 forward
cmux browser surface:2 reload --snapshot-after
cmux browser surface:2 url

cmux browser surface:2 focus-webview
cmux browser surface:2 is-webview-focused
```

## 待機

waitを使用して、セレクタ、テキスト、URLフラグメント、ロード状態、またはJavaScript条件が満たされるまでブロックします。

```
cmux browser surface:2 wait --load-state complete --timeout-ms 15000
cmux browser surface:2 wait --selector "#checkout" --timeout-ms 10000
cmux browser surface:2 wait --text "Order confirmed"
cmux browser surface:2 wait --url-contains "/dashboard"
cmux browser surface:2 wait --function "window.__appReady === true"
```

## DOM操作

変更を伴うアクションは、スクリプトでの高速検証のために--snapshot-afterをサポートしています。

```
cmux browser surface:2 click "button[type='submit']" --snapshot-after
cmux browser surface:2 dblclick ".item-row"
cmux browser surface:2 hover "#menu"
cmux browser surface:2 focus "#email"
cmux browser surface:2 check "#terms"
cmux browser surface:2 uncheck "#newsletter"
cmux browser surface:2 scroll-into-view "#pricing"

cmux browser surface:2 type "#search" "cmux"
cmux browser surface:2 fill "#email" --text "ops@example.com"
cmux browser surface:2 fill "#email" --text ""
cmux browser surface:2 press Enter
cmux browser surface:2 keydown Shift
cmux browser surface:2 keyup Shift
cmux browser surface:2 select "#region" "us-east"
cmux browser surface:2 scroll --dy 800 --snapshot-after
cmux browser surface:2 scroll --selector "#log-view" --dx 0 --dy 400
```

## インスペクション

スクリプト用には構造化されたゲッターを、人間によるレビュー用にはスナップショット/スクリーンショットを使用します。

```
cmux browser surface:2 snapshot --interactive --compact
cmux browser surface:2 snapshot --selector "main" --max-depth 5
cmux browser surface:2 screenshot --out /tmp/cmux-page.png

cmux browser surface:2 get title
cmux browser surface:2 get url
cmux browser surface:2 get text "h1"
cmux browser surface:2 get html "main"
cmux browser surface:2 get value "#email"
cmux browser surface:2 get attr "a.primary" --attr href
cmux browser surface:2 get count ".row"
cmux browser surface:2 get box "#checkout"
cmux browser surface:2 get styles "#total" --property color

cmux browser surface:2 is visible "#checkout"
cmux browser surface:2 is enabled "button[type='submit']"
cmux browser surface:2 is checked "#terms"

cmux browser surface:2 find role button --name "Continue"
cmux browser surface:2 find text "Order confirmed"
cmux browser surface:2 find label "Email"
cmux browser surface:2 find placeholder "Search"
cmux browser surface:2 find alt "Product image"
cmux browser surface:2 find title "Open settings"
cmux browser surface:2 find testid "save-btn"
cmux browser surface:2 find first ".row"
cmux browser surface:2 find last ".row"
cmux browser surface:2 find nth 2 ".row"

cmux browser surface:2 highlight "#checkout"
```

## JavaScript実行と注入

```
cmux browser surface:2 eval "document.title"
cmux browser surface:2 eval --script "window.location.href"

cmux browser surface:2 addinitscript "window.__cmuxReady = true;"
cmux browser surface:2 addscript "document.querySelector('#name')?.focus()"
cmux browser surface:2 addstyle "#debug-banner { display: none !important; }"
```

## 状態

セッションデータコマンドはcookie、ローカル/セッションストレージ、完全なブラウザ状態スナップショットをカバーします。

```
cmux browser surface:2 cookies get
cmux browser surface:2 cookies get --name session_id
cmux browser surface:2 cookies set session_id abc123 --domain example.com --path /
cmux browser surface:2 cookies clear --name session_id
cmux browser surface:2 cookies clear --all

cmux browser surface:2 storage local set theme dark
cmux browser surface:2 storage local get theme
cmux browser surface:2 storage local clear
cmux browser surface:2 storage session set flow onboarding
cmux browser surface:2 storage session get flow

cmux browser surface:2 state save /tmp/cmux-browser-state.json
cmux browser surface:2 state load /tmp/cmux-browser-state.json
```

## タブ

ブラウザタブ操作は、アクティブなブラウザタブグループのブラウザサーフェスにマッピングされます。

```
cmux browser surface:2 tab list
cmux browser surface:2 tab new https://example.com/pricing

# Switch by index or by target surface
cmux browser surface:2 tab switch 1
cmux browser surface:2 tab switch surface:7

# Close current tab or a specific target
cmux browser surface:2 tab close
cmux browser surface:2 tab close surface:7
```

## コンソールとエラー

```
cmux browser surface:2 console list
cmux browser surface:2 console clear

cmux browser surface:2 errors list
cmux browser surface:2 errors clear
```

## ダイアログ

```
cmux browser surface:2 dialog accept
cmux browser surface:2 dialog accept "Confirmed by automation"
cmux browser surface:2 dialog dismiss
```

## フレーム

```
# Enter an iframe context
cmux browser surface:2 frame "iframe[name='checkout']"
cmux browser surface:2 click "#pay-now"

# Return to the top-level document
cmux browser surface:2 frame main
```

## ダウンロード

```
cmux browser surface:2 click "a#download-report"
cmux browser surface:2 download --path /tmp/report.csv --timeout-ms 30000
```

## よくあるパターン

### ナビゲート、待機、インスペクト

```
cmux browser open https://example.com/login
cmux browser surface:2 wait --load-state complete --timeout-ms 15000
cmux browser surface:2 snapshot --interactive --compact
cmux browser surface:2 get title
```

### フォーム入力と成功テキストの確認

```
cmux browser surface:2 fill "#email" --text "ops@example.com"
cmux browser surface:2 fill "#password" --text "$PASSWORD"
cmux browser surface:2 click "button[type='submit']" --snapshot-after
cmux browser surface:2 wait --text "Welcome"
cmux browser surface:2 is visible "#dashboard"
```

### 失敗時のデバッグアーティファクトの取得

```
cmux browser surface:2 console list
cmux browser surface:2 errors list
cmux browser surface:2 screenshot --out /tmp/cmux-failure.png
cmux browser surface:2 snapshot --interactive --compact
```

### ブラウザセッションの保存と復元

```
cmux browser surface:2 state save /tmp/session.json
# ...later...
cmux browser surface:2 state load /tmp/session.json
cmux browser surface:2 reload
```


# 通知 — cmux docs

# 通知

cmuxはデスクトップ通知をサポートしており、AIエージェントやスクリプトが注意を必要とするときに通知できます。

## ライフサイクル

1.  受信：通知がパネルに表示され、デスクトップアラートが発火（抑制されていない場合）
2.  未読：ワークスペースタブにバッジを表示
3.  既読：そのワークスペースを表示するとクリア
4.  クリア済み：パネルから削除

### 抑制

デスクトップアラートは以下の場合に抑制されます：

-   cmuxウィンドウにフォーカスがある
-   通知を送信した特定のワークスペースがアクティブ
-   通知パネルが開いている

### 通知パネル

`⌘⇧I`を押して通知パネルを開きます。通知をクリックするとそのワークスペースにジャンプします。`⌘⇧U`を押すと、最新の未読通知があるワークスペースに直接ジャンプします。

## カスタムコマンド

通知がスケジュールされるたびにシェルコマンドを実行できます。設定 > アプリ > 通知コマンドで設定してください。コマンドは/bin/sh -cで実行され、以下の環境変数が使用可能です：

変数

説明

`CMUX_NOTIFICATION_TITLE`

通知タイトル（ワークスペース名またはアプリ名）

`CMUX_NOTIFICATION_SUBTITLE`

通知サブタイトル

`CMUX_NOTIFICATION_BODY`

通知本文

Examples

```
# Text-to-speech
say "$CMUX_NOTIFICATION_TITLE"

# Custom sound file
afplay /path/to/sound.aiff

# Log to file
echo "$CMUX_NOTIFICATION_TITLE: $CMUX_NOTIFICATION_BODY" >> ~/notifications.log
```

コマンドはシステムサウンドピッカーとは独立して実行されます。カスタムコマンドのみを使用する場合はピッカーを「なし」に設定し、システムサウンドとカスタムアクションの両方を使用する場合は両方を有効にしてください。

## 通知の送信

### CLI

```
cmux notify --title "Task Complete" --body "Your build finished"
cmux notify --title "Claude Code" --subtitle "Waiting" --body "Agent needs input"
```

### OSC 777（シンプル）

RXVTプロトコルはタイトルと本文の固定フォーマットを使用します：

```
printf '\e]777;notify;My Title;Message body here\a'
```

Shell function

```
notify_osc777() {
    local title="$1"
    local body="$2"
    printf '\e]777;notify;%s;%s\a' "$title" "$body"
}

notify_osc777 "Build Complete" "All tests passed"
```

### OSC 99（リッチ）

Kittyプロトコルはサブタイトルと通知IDをサポートします：

```
# Format: ESC ] 99 ; <params> ; <payload> ESC \

# Simple notification
printf '\e]99;i=1;e=1;d=0:Hello World\e\\'

# With title, subtitle, and body
printf '\e]99;i=1;e=1;d=0;p=title:Build Complete\e\\'
printf '\e]99;i=1;e=1;d=0;p=subtitle:Project X\e\\'
printf '\e]99;i=1;e=1;d=1;p=body:All tests passed\e\\'
```

機能

OSC 99

OSC 777

タイトル + 本文

あり

あり

サブタイトル

あり

なし

通知ID

あり

なし

複雑さ

高い

低い

シンプルな通知にはOSC 777を使用してください。サブタイトルや通知IDが必要な場合はOSC 99を使用してください。最も簡単な連携にはCLI（cmux notify）を使用してください。

## Claude Codeフック

cmuxは[Claude Code](https://docs.anthropic.com/en/docs/claude-code)とフックで連携し、タスク完了時に通知します。

### 1\. フックスクリプトの作成

~/.claude/hooks/cmux-notify.sh

```
#!/bin/bash
# Skip if not in cmux
[ -S /tmp/cmux.sock ] || exit 0

EVENT=$(cat)
EVENT_TYPE=$(echo "$EVENT" | jq -r '.hook_event_name // "unknown"')
TOOL=$(echo "$EVENT" | jq -r '.tool_name // ""')

case "$EVENT_TYPE" in
    "Stop")
        cmux notify --title "Claude Code" --body "Session complete"
        ;;
    "PostToolUse")
        [ "$TOOL" = "Task" ] && cmux notify --title "Claude Code" --body "Agent finished"
        ;;
esac
```

```
chmod +x ~/.claude/hooks/cmux-notify.sh
```

### 2\. Claude Codeの設定

~/.claude/settings.json

```
{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/cmux-notify.sh"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Task",
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/cmux-notify.sh"
          }
        ]
      }
    ]
  }
}
```

フックを適用するにはClaude Codeを再起動してください。

## GitHub Copilot CLI

Copilot CLIは、プロンプト送信、エージェント停止、エラーなどのライフサイクルイベントでシェルコマンドを実行する[フック](https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/use-hooks)をサポートしています。

~/.copilot/config.json

```
{
  "hooks": {
    "userPromptSubmitted": [
      {
        "type": "command",
        "bash": "if command -v cmux &>/dev/null; then cmux set-status copilot_cli Running; fi",
        "timeoutSec": 3
      }
    ],
    "agentStop": [
      {
        "type": "command",
        "bash": "if command -v cmux &>/dev/null; then cmux notify --title 'Copilot CLI' --body 'Done'; cmux set-status copilot_cli Idle; fi",
        "timeoutSec": 5
      }
    ],
    "errorOccurred": [
      {
        "type": "command",
        "bash": "if command -v cmux &>/dev/null; then cmux notify --title 'Copilot CLI' --subtitle 'Error' --body 'An error occurred'; cmux set-status copilot_cli Error; fi",
        "timeoutSec": 5
      }
    ],
    "sessionEnd": [
      {
        "type": "command",
        "bash": "if command -v cmux &>/dev/null; then cmux clear-status copilot_cli; fi",
        "timeoutSec": 3
      }
    ]
  }
}
```

リポジトリレベルのフックには、同じ構造で.github/hooks/notify.jsonファイルを作成してください：

.github/hooks/notify.json

```
{
  "version": 1,
  "hooks": {
    "userPromptSubmitted": [ ... ],
    "agentStop": [ ... ]
  }
}
```

## 連携の例

### 長時間コマンド後の通知

~/.zshrc

```
# Add to your shell config
notify-after() {
  "$@"
  local exit_code=$?
  if [ $exit_code -eq 0 ]; then
    cmux notify --title "✓ Command Complete" --body "$1"
  else
    cmux notify --title "✗ Command Failed" --body "$1 (exit $exit_code)"
  fi
  return $exit_code
}

# Usage: notify-after npm run build
```

### Python

python

```
import sys

def notify(title: str, body: str):
    """Send OSC 777 notification."""
    sys.stdout.write(f'\x1b]777;notify;{title};{body}\x07')
    sys.stdout.flush()

notify("Script Complete", "Processing finished")
```

### Node.js

node

```
function notify(title, body) {
  process.stdout.write(`\x1b]777;notify;${title};${body}\x07`);
}

notify('Build Done', 'webpack finished');
```

### tmuxパススルー

cmux内でtmuxを使用する場合、パススルーを有効にしてください：

.tmux.conf

```
set -g allow-passthrough on
```

```
printf '\ePtmux;\e\e]777;notify;Title;Body\a\e\\'
```


# SSH — cmux docs

# SSH

cmux ssh creates a workspace for a remote machine. Browser panes route through the remote network, files drag-and-drop via scp, coding agents send notifications to your local sidebar, and sessions reconnect on drops.

<iframe class="my-6 rounded-lg w-full aspect-video" src="https://www.youtube.com/embed/RoR9pMOZWkk" title="cmux SSH demo" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen=""></iframe>

## Usage

```
cmux ssh user@remote
cmux ssh user@remote --name "dev server"
cmux ssh user@remote -p 2222
cmux ssh user@remote -i ~/.ssh/id_ed25519
```

cmux ssh reads your ~/.ssh/config for host aliases, identity files, and proxy settings. All flags mirror their ssh equivalents.

## Flags

Flag

Description

`--name`

Set the workspace title

`-p, --port`

SSH port (default 22)

`-i, --identity`

Path to identity file

`-o, --ssh-option`

Pass arbitrary SSH options (e.g. -o StrictHostKeyChecking=no)

`--no-focus`

Create the workspace without switching to it

## Browser panes

Browser panes in a remote workspace route all HTTP and WebSocket traffic through the remote machine's network. Type localhost:3000 and you're looking at the dev server running on the remote box. No -L flags, no manual port forwarding. Each remote workspace gets an isolated cookie store so sessions are scoped per-connection.

## Drag and drop

Drag an image or file into a remote terminal and cmux uploads it via scp through the existing SSH connection. cmux detects the foreground SSH process by TTY and routes the upload through ControlMaster multiplexing.

## Notifications

Processes on the remote machine can run cmux commands that execute on your local instance. When a coding agent calls cmux notify on the remote box, the notification appears in your local sidebar. The blue ring lights up on the workspace tab. Cmd+Shift+U jumps to it. Notification spam from flaky connections is suppressed with a per-host cooldown.

## Coding agents over SSH

cmux claude-teams and cmux omo both work inside SSH sessions. The Go relay daemon on the remote host handles the same tmux-compat translation that the local Swift CLI does. Teammate agents spawn as native cmux splits on your local machine while computation runs on the remote box.

```
# Inside an SSH session:
cmux claude-teams
cmux omo
```

## Reconnect

When the connection drops, cmux reconnects with exponential backoff (3s, 6s, 12s, up to 60s). The remote session persists and cmux reattaches on reconnect, resizing with smallest-screen-wins semantics. Default keepalive options (ServerAliveInterval=20, ServerAliveCountMax=2) are injected unless your config already sets them.

## Relay daemon

On first connect, cmux probes the remote host (uname -s, uname -m) and uploads a versioned cmuxd-remote binary. The binary speaks JSON-RPC over stdio and handles three things:

Feature

How

Browser traffic proxying

SOCKS5 and HTTP CONNECT over the daemon's stdio channel

CLI relay

Reverse TCP tunnel with HMAC-SHA256 auth so remote processes can call cmux commands locally

Session management

Persists sessions across reconnects, coordinates PTY resize across multiple attachments

The daemon binary is stored at ~/.cmux/bin/cmuxd-remote/<version>/<os>-<arch>/cmuxd-remote on the remote host and verified against a SHA-256 manifest embedded in the app.