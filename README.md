# VOA Content Pipeline

独立于 App 后台的音频学习资料采集工程。它只生产可审核的内容包，不直接发布内容，也不调用 App API。

## 数据流

`VOA sitemap -> 音频文章筛选 -> 原始网页 + 音频归档 -> 规范化与分类 -> candidate manifest -> CMS 审核 -> 后台导入`

每篇资料保存为不可变对象：

```
candidates/voa/<source-id>/
  manifest.json
  article.json
  source.html
  audio.mp3
```

`manifest.json` 是 CMS 的稳定交换契约。状态只会是 `candidate`，是否上架由 CMS 决定。

## 本地试运行

```sh
cp .env.example .env
docker compose up -d minio
go run ./cmd/library-sync -limit 10
```

默认读取 VOA 官方 sitemap，只处理有 MP3 的文章，跳过视频 sitemap，使用 BoltDB 记录已成功和失败 URL，重复执行可断点续跑。

## 部署与迁移

采集进程只依赖 S3 协议。内网使用 MinIO，迁到公有云时将 bucket 镜像到目标对象存储，再替换 `CONTENT_S3_*` 配置即可；manifest 不保存供应商地址。详见 [docs/architecture.md](docs/architecture.md)。
