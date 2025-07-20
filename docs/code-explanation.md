# Webhookコードの詳細解説（初学者向け）

このドキュメントでは、Kubernetes Deployment-HPA Validatorのコードを初学者にもわかりやすく解説します。

## 目次

1. [全体構造](#1-全体構造)
2. [処理の流れ](#2-処理の流れ)
3. [メインエントリーポイント](#3-メインエントリーポイント-cmdwebhookmaingo)
4. [設定管理](#4-設定管理-internalconfigconfiggo)
5. [バリデーションロジック](#5-バリデーションロジック-internalvalidatorvalidatorgo)
   - [5.1 バリデーター構造体](#51-バリデーター構造体)
   - [5.2 HPA検証ロジック](#52-hpa検証ロジック)
   - [5.3 Deployment検証ロジック](#53-deployment検証ロジック)
6. [Webhookサーバー](#6-webhookサーバー-internalwebhookservergo)
   - [6.1 サーバー構造体](#61-サーバー構造体)
   - [6.2 サーバー初期化](#62-サーバー初期化)
   - [6.3 サーバー起動](#63-サーバー起動)
   - [6.4 リクエスト処理](#64-リクエスト処理)
   - [6.5 HPA検証処理](#65-hpa検証処理)
7. [証明書管理](#7-証明書管理-internalcertmanagergo)
   - [7.1 証明書マネージャー](#71-証明書マネージャー)
   - [7.2 証明書の読み込み](#72-証明書の読み込み)
   - [7.3 証明書の自動更新](#73-証明書の自動更新)
8. [メトリクス収集](#8-メトリクス収集-internalmetricsmetricsgo)
   - [8.1 メトリクス定義](#81-メトリクス定義)
   - [8.2 リクエストメトリクス](#82-リクエストメトリクス)
9. [ロギング](#9-ロギング-internalloggingloggergo)
   - [9.1 ロガー構造体](#91-ロガー構造体)
   - [9.2 ログ出力メソッド](#92-ログ出力メソッド)

詳細な解説は以下のファイルに分かれています：

- [コード解説 - パート1](code-explanation-part1.md) - 全体構造、処理の流れ、メインエントリーポイント
- [コード解説 - パート2](code-explanation-part2.md) - 設定管理
- [コード解説 - パート3](code-explanation-part3.md) - バリデーションロジック（HPA）
- [コード解説 - パート4](code-explanation-part4.md) - バリデーションロジック（Deployment）、Webhookサーバー構造
- [コード解説 - パート5](code-explanation-part5.md) - サーバー初期化
- [コード解説 - パート6](code-explanation-part6.md) - サーバー起動、リクエスト処理
- [コード解説 - パート7](code-explanation-part7.md) - HPA検証処理
- [コード解説 - パート8](code-explanation-part8.md) - 証明書管理
- [コード解説 - パート9](code-explanation-part9.md) - メトリクス収集
- [コード解説 - パート10](code-explanation-part10.md) - リクエストメトリクス、ロギング