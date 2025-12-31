# AirGit Agent Workflow

このディレクトリには、AirGit の GitHub Actions ワークフローが格納されています。

## agent.yml

GitHub Issues に `/airgit run` というコメントを投稿することで、自動的にコード生成エージェントが起動します。

### ワークフロー動作

1. **トリガー**: Issue のコメント欄に `/airgit run` を投稿
2. **確認**: Agent が確認メッセージを投稿
3. **解析**: Issue の内容を解析
4. **生成**: 変更を生成（現在はプレースホルダー）
5. **ブランチ作成**: `airgit/issue-<NUMBER>` という名前でブランチを作成
6. **コミット**: 生成された変更をコミット
7. **PR 作成**: 自動的に Pull Request を作成
8. **通知**: 完了コメントを投稿

### 使用方法

```bash
# Issue の詳細ページで、以下のコメントを投稿してください:
/airgit run
```

または AirGit UI の「Issues」タブから「Trigger Agent」ボタンをクリックします。

### 今後の拡張

- `gh copilot` または LLM API を使用した実際のコード生成
- より複雑なコード生成ロジックの実装
- 生成されたコードの検証とテスト
- より詳細な PR コメント
