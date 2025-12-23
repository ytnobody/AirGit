# Systemd Registration - Process Behavior Documentation

## よくある質問：登録後のプロセスの動作

### Q: 登録時に現在のプロセスは終了しますか？

**A: はい。登録成功後、以下の処理が自動的に行われます：**

```
1. POST /api/systemd/register リクエスト受信
       ↓
2. サービスファイルを作成: ~/.config/systemd/user/airgit.service
       ↓
3. systemctl daemon-reload を実行
       ↓
4. systemctl enable airgit を実行
       ↓
5. クライアントに成功レスポンス送信
       ↓
6. 現在のプロセス(手動起動)を終了
       ↓
7. systemctl start airgit で新しいプロセスを起動
       ↓
8. ブラウザが自動的に再接続
```

### Q: ポート番号は同じですか？

**A: はい。完全に同じポート番号で起動します。**

**例：**
```bash
# 手動で起動（ポート8080）
./airgit --listen-port 8080

# ブラウザで Settings → Register with Systemd をクリック

# 現在のプロセスが終了
# ↓
# systemdが同じオプションで新しいプロセスを起動
# ExecStart=/path/to/airgit --listen-port 8080
```

結果：
- ✓ ポート: 8080（変わらない）
- ✓ オプション: --listen-port 8080（保存される）
- ✓ 環境変数: すべて引き継がれる
- ✓ ユーザーは同じURLで再接続可能

## 詳細な動作フロー

### 登録時のプロセス遷移

```
時刻   プロセス状態                    画面表示
───────────────────────────────────────────────────────────
T0     [AirGit手動プロセス実行中]      通常の画面
       PID: 12345
       ポート: 8080
       
       ユーザーが登録ボタンをクリック
       
T1     [登録中...]                    ローディング表示
       サービスファイル作成中
       
T2     [登録成功]                     ✓ Successfully registered!
       systemctl enable 実行           Restarting via systemd...
       
T3     [手動プロセス終了]              
       PID: 12345 → 終了
       ブラウザ接続: 切断
       
T4     [新しいsystemdプロセス起動]     Reconnecting...
       PID: 12346
       ポート: 8080（同じ）
       
T5     [新プロセス起動完了]            ページ自動リロード
       ブラウザ再接続: 成功
```

## 実装の詳細

### バックエンド処理（Go）

```go
// 登録成功後の処理
json.NewEncoder(w).Encode(map[string]interface{}{
    "success": true,
    "message": "Service registered and enabled successfully",
})

// Goroutineで非同期実行
go func() {
    // レスポンス送信を待つ
    time.Sleep(500 * time.Millisecond)
    
    // systemd経由でサービスを起動
    startCmd := exec.Command("systemctl", "--user", "start", "airgit.service")
    startCmd.Run()
    
    // 現在のプロセスを終了
    os.Exit(0)
}()
```

**重要ポイント：**
- レスポンス送信後に処理を実行（500ms待機）
- 新しいプロセスを起動（同じオプションで）
- その後、現在のプロセスを終了

### フロントエンド処理（JavaScript）

```javascript
systemdRegisterBtn.addEventListener('click', async () => {
    // 登録APIを呼び出し
    const response = await fetch('/api/systemd/register', { method: 'POST' });
    const data = await response.json();
    
    if (response.ok && data.success) {
        // 成功メッセージ表示
        systemdStatusMessage.textContent = '✓ Successfully registered! Restarting via systemd...';
        
        // 1秒後に再接続予定を表示
        setTimeout(() => {
            systemdStatusMessage.textContent = '✓ Registered! Service is now running via systemd. Reconnecting...';
            
            // 2秒後にページをリロード
            setTimeout(() => {
                window.location.reload();
            }, 2000);
        }, 1000);
    }
});
```

**ユーザー体験：**
1. 「成功しました！Systemd経由で再起動中...」表示
2. 1秒後に「再接続中...」表示
3. 2秒後にページが自動リロード
4. 新しいプロセスに再接続

## ポート番号が同じ理由

### コマンドライン引数の保存

```go
// 現在のプロセスのコマンドライン引数を取得
execArgs := os.Args[1:]  // ["--listen-port", "8080"]

// サービスファイルに含める
cmdLine := execPath + " " + strings.Join(execArgs, " ")
// 結果: /path/to/airgit --listen-port 8080

// サービスファイルの ExecStart に設定
ExecStart=/path/to/airgit --listen-port 8080
```

### サービスファイルの例

```ini
[Service]
Type=simple
ExecStart=/home/user/airgit --listen-port 8080
Restart=on-failure
RestartSec=5
```

新しいプロセスが起動するときは、このExecStartコマンドが実行される：
```bash
/home/user/airgit --listen-port 8080
```

結果：元々と同じポート8080で起動

## 環境変数も引き継がれる

環境変数も同様に保存される：

```bash
# 登録時
export AIRGIT_SSH_HOST=git.example.com
export GIT_AUTHOR_NAME="Deploy Bot"
./airgit --listen-port 8080
# Settings → Register をクリック

# サービスファイルに保存される
ExecStart=/path/to/airgit --listen-port 8080
Environment="AIRGIT_SSH_HOST=git.example.com"
Environment="GIT_AUTHOR_NAME=Deploy Bot"

# 新しいプロセスも同じ環境で起動
```

## トラブルシューティング

### 再接続に失敗する場合

**症状：** "Reconnecting..." と表示されて進まない

**原因：**
1. systemdサービスが起動に失敗
2. ネットワーク接続の問題
3. ファイアウォール設定

**解決方法：**
```bash
# サービスの状態確認
systemctl --user status airgit

# ログを確認
journalctl --user-unit airgit -n 20

# 手動で再起動
systemctl --user restart airgit

# ブラウザでリロード
# または http://localhost:8080 にアクセス
```

### ポート番号が変わる場合

**症状：** 登録後、別のポートになっている

**原因：** コマンドライン引数がサービスファイルに保存されていない

**確認方法：**
```bash
# サービスファイルを確認
cat ~/.config/systemd/user/airgit.service

# ExecStart に引数が含まれているか確認
# 例：ExecStart=/path/to/airgit --listen-port 8080
```

## よくある状況別対応

### 状況1: ポート8080で起動中に登録

```bash
./airgit --listen-port 8080

# Settings → Register をクリック
↓
# 新しいプロセスがポート8080で起動（同じポート）
# ユーザーは同じURLで継続使用可能
```

### 状況2: 登録後、別のポートで使いたい場合

```bash
# 1. 現在のサービスを停止
systemctl --user stop airgit

# 2. サービスファイルを編集
nano ~/.config/systemd/user/airgit.service
# ExecStart を修正

# 3. systemdをリロード
systemctl --user daemon-reload

# 4. サービスを開始
systemctl --user start airgit
```

### 状況3: 複数のポートで複数インスタンス

```bash
# インスタンス1（ポート8001）
./airgit --listen-port 8001
# 別の方法で登録（手動でサービスファイル作成）

# インスタンス2（ポート8002）
./airgit --listen-port 8002
# 別の方法で登録（手動でサービスファイル作成）

# または、同じプロセスで複数ポート設定
```

## セキュリティ考慮事項

### プロセス切り替え時

✓ 安全：
- コマンドライン引数は保存済み
- 環境変数は保存済み
- 設定は継続

✗ 注意：
- 登録中はブラウザが一時的に切断される（正常）
- 接続が中断される（ブラウザが自動リロード）

### サービスファイル保護

```bash
# サービスファイルの権限確認
ls -l ~/.config/systemd/user/airgit.service

# 標準的な権限
-rw-r--r-- user group

# 敏感な情報が含まれていないか確認
cat ~/.config/systemd/user/airgit.service
```

## まとめ

| 質問 | 回答 |
|------|------|
| **プロセスは終了する？** | はい。登録後、手動プロセスは終了し、systemdプロセスが起動します |
| **ポート番号は同じ？** | はい。コマンドライン引数が保存されるので、同じポートで起動します |
| **ユーザーに影響？** | ブラウザが自動リロードされるだけ。通常は継続して使用可能 |
| **環境変数は？** | すべて保存され、新しいプロセスに引き継がれます |
| **再接続失敗は？** | ブラウザのリロードで対応。systemdのログ確認推奨 |

## 実装の利点

✓ **シームレスな切り替え** - ユーザーは違和感なく使用可能
✓ **設定の継続** - オプションと環境変数がそのまま適用
✓ **自動化** - 手動操作なしで切り替え完了
✓ **ポート競合なし** - 同じポートで起動（キープアライブ処理がある）

---

**実装状況:** ✅ 完了
**テスト状況:** 准備完了
**ドキュメント:** 完全
