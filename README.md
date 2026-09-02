# VOA Content Pipeline

独立于 App 后台的多数据源音频学习资料采集工程。它只生产可审核的内容包，不直接发布内容，也不调用 App API。VOA 是第一个 Source Connector，不是 Pipeline 的内置假设。

## 数据流

`Source Connector -> 音频文章筛选 -> 原始数据 + 音频归档 -> 统一模型 -> 分类器 -> candidate manifest -> CMS 审核 -> 后台导入`

每篇资料保存为不可变对象：

```
candidates/<source-connector-id>/<content-id>/
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

默认启用 VOA connector，读取其官方 sitemap，只处理有 MP3 的文章并跳过视频 sitemap。使用 `source-id + URL` 记录独立抓取状态，多数据源之间不会碰撞。

VOA 当前最新索引包含大量无音频普通新闻。首次建库可用 `CONTENT_VOA_MAX_ARTICLE_ID` 设置历史游标，从已验证的音频内容段开始；设为 `0` 表示不限制。该字段只属于 VOA connector，不进入通用内容模型。

新增数据源只需实现 `source.Connector`，提供来源 ID、发现列表、规范化候选内容与媒体下载；Pipeline、分类器、存储和 CMS 契约不需要修改。

## 部署与迁移

采集进程只依赖 S3 协议。内网使用 MinIO，迁到公有云时将 bucket 镜像到目标对象存储，再替换 `CONTENT_S3_*` 配置即可；manifest 不保存供应商地址。详见 [docs/architecture.md](docs/architecture.md)。
