# Kubernetes Deployment-HPA Validator デプロイガイド

このガイドでは、Kubernetes Deployment-HPA Validatorを本番環境にデプロイする手順を説明します。

## 目次

1. [前提条件](#1-前提条件)
   - [必要なツール](#必要なツール)
   - [アクセス権](#アクセス権)
2. [kubectl を使用した直接デプロイ](#2-kubectl-を使用した直接デプロイ)
   - [リポジトリのクローン](#1-リポジトリのクローン)
   - [TLS証明書の生成](#2-tls証明書の生成)
   - [コンテナイメージのビルドとプッシュ](#3-コンテナイメージのビルドとプッシュ)
   - [マニフェストの編集](#4-マニフェストの編集)
   - [Kubernetesリソースのデプロイ](#5-kubernetesリソースのデプロイ)
   - [デプロイの確認](#6-デプロイの確認)
   - [動作確認](#7-動作確認)
3. [ArgoCDを使用した自動デプロイ](#3-argocdを使用した自動デプロイ)
   - [リポジトリの準備](#1-リポジトリの準備)
   - [TLS証明書の準備](#2-tls証明書の準備)
   - [ArgoCD Application定義の作成](#3-argocd-application定義の作成)
   - [ArgoCDダッシュボードでの確認](#4-argocdダッシュボードでの確認)
   - [同期の手動トリガー](#5-同期の手動トリガー必要な場合)
   - [デプロイの確認](#6-デプロイの確認-1)
   - [更新の適用](#7-更新の適用)
4. [デプロイ後の確認](#4-デプロイ後の確認)
   - [Webhookの動作確認](#1-webhookの動作確認)
   - [メトリクスの確認](#2-メトリクスの確認)
   - [ヘルスチェックの確認](#3-ヘルスチェックの確認)
5. [トラブルシューティング](#5-トラブルシューティング)
   - [Webhookが応答しない](#1-webhookが応答しない)
   - [証明書エラー](#2-証明書エラー)
   - [権限エラー](#3-権限エラー)
   - [設定の問題](#4-設定の問題)

詳細な手順は以下のファイルに分かれています：

- [デプロイガイド - パート1](deployment-guide-part1.md) - 前提条件、kubectl直接デプロイ（準備）
- [デプロイガイド - パート2](deployment-guide-part2.md) - kubectl直接デプロイ（実行）
- [デプロイガイド - パート3](deployment-guide-part3.md) - ArgoCDを使用した自動デプロイ（準備）
- [デプロイガイド - パート4](deployment-guide-part4.md) - ArgoCDを使用した自動デプロイ（実行）
- [デプロイガイド - パート5](deployment-guide-part5.md) - デプロイ後の確認、トラブルシューティング