# AirGit 要件定義書 (Requirements Definition)

## 1. プロジェクト概要

**AirGit** は、モバイル端末（Android等）からSSH経由でリモートサーバー上のGitリポジトリを操作するための、軽量なWebベースGUIツールである。Goのシングルバイナリとして提供され、サーバー側での環境構築やスマホ側へのクローンを必要とせず、「ブラウザからワンタップでPush」を実現する。

## 2. ターゲット・ユースケース

* VibeKanban等のツールで編集したファイルを、モバイルから即座にリモートリポジトリへPushする。
* スマホブラウザでのレイアウト崩れや、ターミナルでのコマンド入力を排除する。

## 3. システムアーキテクチャ

* **Backend**: Go (net/http, golang.org/x/crypto/ssh)
* **Frontend**: HTML5, Tailwind CSS (embedパッケージでGoバイナリに同梱)
* **Communication**: スマホ ↔ AirGit (HTTP) ↔ Remote Server (SSH)

## 4. 機能要件

### A. SSHクライアント機能

* 指定されたリモートサーバーへのSSH接続。
* 公開鍵認証（`.ssh/id_rsa` 等）をサポート。
* 接続情報は起動時の環境変数（`.env`）または引数で指定。

### B. Git操作API

1. **Status API (`GET /api/status`)**:
* 現在のブランチ名 (`git branch --show-current`) を取得。
* 未プッシュ・未コミットの変更の有無を取得。


2. **Push API (`POST /api/push`)**:
* 下記コマンドを順次実行：
`git add .` -> `git commit -m "Updated via AirGit"` -> `git push origin [current_branch]`
* 実行ログの標準出力をフロントエンドにストリーミング、または一括返却。



### C. フロントエンド (AirGit GUI)

* **ミニマルデザイン**: 画面中央に大きな「Push」ボタンを1つ配置。
* **ステータス表示**: 現在のブランチ名と、接続先サーバー情報を上部に表示。
* **フィードバック**: Push実行中はローディングアニメーションを表示し、成功時は「Success!」と大きく通知。
* **PWA対応**: モバイルのホーム画面にアイコンとして追加できるよう、`manifest.json` と `service-worker.js` (簡易版) を用意。

## 5. 画面設計（モバイル最適化）

* 縦画面（Portrait）専用設計。
* 指でタップしやすいボタンサイズ（44px以上）。
* 背景は目に優しいダークモード対応。

## 6. Systemd ユーザーモードサービス登録機能

### 概要
AirGit をユーザーモード（user-mode）の systemd サービスとして登録し、自動起動を実現する機能を提供する。

### API エンドポイント

#### 1. **Systemd Status API (`GET /api/systemd/status`)**
現在のサービス登録状態を確認する。

**レスポンス例:**
```json
{
  "registered": true
}
```

#### 2. **Systemd Register API (`POST /api/systemd/register`)**
AirGit をユーザーモード systemd サービスとして登録する。

**機能:**
- `~/.config/systemd/user/airgit.service` ファイルを作成
- 現在の実行可能ファイルのパスを自動取得
- systemd デーモンをリロード
- サービスを enable して自動起動を有効化

**レスポンス成功例:**
```json
{
  "success": true,
  "message": "Service registered and enabled successfully",
  "path": "/home/user/.config/systemd/user/airgit.service"
}
```

**レスポンスエラー例（既に登録済み）:**
```json
{
  "success": false,
  "error": "Service is already registered with systemd"
}
```

### サービスファイル内容
生成されるサービスファイル（`airgit.service`）の内容：
```ini
[Unit]
Description=AirGit - Lightweight web-based Git GUI
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/path/to/airgit
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=default.target
```

### 使用方法
1. AirGit サーバーが起動している状態で、`POST /api/systemd/register` にリクエストを送信
2. 既に登録されている場合は、409 Conflict エラーが返される
3. 登録成功後、ユーザーのログイン時に AirGit は自動起動

### サービス管理コマンド
```bash
# 登録状態確認
curl http://localhost:8080/api/systemd/status

# サービス登録
curl -X POST http://localhost:8080/api/systemd/register

# 登録後、systemctl で管理
systemctl --user status airgit
systemctl --user start airgit
systemctl --user stop airgit
systemctl --user restart airgit
```

---

## AI（Cursor/Claude等）への指示用プロンプト

プロジェクト名「AirGit」を作成してください。Go言語のシングルバイナリで動作し、SSH経由でリモートのGitリポジトリを操作するWebツールです。
1. `main.go` で `golang.org/x/crypto/ssh` を使い、SSH経由で `git status` や `git push` を実行するHTTPサーバーを構築してください。
2. `embed` パッケージを使用して、`static/index.html` をバイナリに埋め込んでください。
3. フロントエンドはTailwind CSSを使用し、モバイルで「Pushボタン」を1回タップするだけで現在のブランチをPushできる直感的なUIにしてください。
4. 設定（SSHホスト、ユーザー、秘密鍵パス、リポジトリの絶対パス）は環境変数または `config.yaml` から読み込むようにしてください。
5. サーバー上のワーキングディレクトリを直接操作するため、スマホ側にクローンは行いません。


まずはディレクトリ構造の提案と、最小限の `main.go` から作成してください。

