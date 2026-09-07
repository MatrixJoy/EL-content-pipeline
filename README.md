# English Learning Content Pipeline

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

## 音频与句子时间轴

抓取完成后，独立 aligner 使用 faster-whisper 生成词级时间，再与资料库正文做单调对齐并聚合为每句的 `start_ms/end_ms`：

```sh
docker compose run --rm aligner --limit 1
```

结果写入 `enrichments/<source>/<content-id>/alignment/<model>-<alignment-version>/timeline.json`，包含正文和音频哈希、模型版本、覆盖率及逐句置信度。只有至少 50% 词被音频证实的句子才标记为 `spoken=true` 并提供时间；网页附录等未朗读内容保持空时间，App 不应高亮。对齐失败不影响原始 candidate，可独立重试。

## 离线重分类

分类规则拥有独立版本。规则升级后直接读取资料库中已有的 `article.json`，不重新抓网页、也不重新下载音频。先执行只读预检，再显式应用：

```sh
docker compose run --rm crawler -reclassify
docker compose run --rm crawler -reclassify -apply
```

应用时只改写分类发生变化的 manifest，并递增其 revision；正文、来源快照、音频和时间轴对象保持不变。重复执行是幂等的。CMS 通过 manifest ETag 自动发现新分类，并按现有安全规则将变化内容恢复为待审核。

## 语法内容 enrichment

对分类为语法学习目标的文章，可从正文中提取高置信度的情态动词、完成时、进行时和条件句，生成例句、简明解释以及基于原文的填空选项：

```sh
docker compose run --rm crawler -enrich-grammar
docker compose run --rm crawler -enrich-grammar -apply
```

默认只读预检。应用后写入新的 `article.grammar-v<version>.json`，再原子切换 manifest 引用；原始 `article.json`、音频和时间轴不被覆盖。非语法类内容不会生成低相关度练习。

## 正式建库

```sh
docker compose up -d crawler-worker aligner-worker
```

`crawler-worker` 遍历配置范围内的来源并断点续跑；`aligner-worker` 每分钟发现新增 candidate，持续补齐句级时间轴。抓取完成后 crawler 正常退出，对齐进程继续等待新数据源内容。单批默认检查 5,000 个新候选，可用 `CONTENT_BUILD_BATCH_SIZE` 按对象存储容量调整，下一批会从断点继续。

## 本地试运行

```sh
cp .env.example .env
docker compose up -d minio
go run ./cmd/library-sync -limit 10
```

默认启用 VOA connector，读取其官方 sitemap，只处理有 MP3 的文章并跳过视频 sitemap。使用 `source-id + extraction-version + URL` 记录独立抓取状态；解析规则升级时会重新清洗旧资料并复用对象存储中的原音频，多数据源之间不会碰撞。VOA 的 “Words in This Story” 会保存为独立的 `featured_words`，不会混入正文或评论说明。

VOA 当前最新索引包含大量无音频普通新闻。首次建库可用 `CONTENT_VOA_MAX_ARTICLE_ID` 设置历史游标，从已验证的音频内容段开始；设为 `0` 表示不限制。该字段只属于 VOA connector，不进入通用内容模型。

新增数据源只需实现 `source.Connector`，提供来源 ID、发现列表、规范化候选内容与媒体下载；Pipeline、分类器、存储和 CMS 契约不需要修改。

## 部署与迁移

采集进程只依赖 S3 协议。内网使用 MinIO，迁到公有云时将 bucket 镜像到目标对象存储，再替换 `CONTENT_S3_*` 配置即可；manifest 不保存供应商地址。详见 [docs/architecture.md](docs/architecture.md)。

MinIO 数据目录由 `CONTENT_OBJECT_DATA_PATH` 配置。生产环境应指向独立数据盘；切换路径前必须停止 MinIO、完整复制并核对普通文件数量和字节总和，原数据卷保留至新路径完成读写验证。
