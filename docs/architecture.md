# 内容平台边界与发布流程

## 职责

- **Source Connector**：每个数据源独立实现发现、读取、来源字段解析和媒体下载；不得包含 CMS 或发布逻辑。
- **Normalizer / Classifier**：将 connector 输出映射为统一英文学习模型，产生稳定主题 slug、CEFR 估计、学习目标和带规则版本的质量分。分类升级可基于已有 article 对象离线重算，无需重新下载媒体。
- **Grammar Enricher**：只处理具有 grammar 学习目标的资料，保存可审核的语法类型、解释、原文例句、填空题干、答案与选项；enrichment 使用独立版本化 article 对象，不覆盖原始抓取结果。
- **Content Pipeline（本仓库）**：编排 connector、分类、去重和持久化，输出 `candidate` 内容包。不能发布。
- **Content Library（S3 bucket）**：保存来源快照和不可变内容包，是可迁移的资料库；不是 App 在线数据库。
- **CMS（后续独立模块）**：扫描 candidate manifest，支持编辑分类、难度、词汇与语法标记；运营人员批准后写入发布 outbox。
- **App Backend（现有 Go 仓库）**：消费 CMS outbox，将已批准版本导入在线目录，向 iOS 提供稳定 API；不访问 VOA。
- **iOS**：只访问 App Backend。后期推荐系统使用学习行为和内容特征排序，不改变内容入库链路。

## 审核与自动入库

1. Pipeline 完成对象上传后，最后写 `manifest.json`，这一步代表内容包完整。
2. CMS 定时列举 `candidates/`，按 `content_id + revision` 幂等导入待审队列。
3. 运营批准后，CMS 产生带版本号的 `content.approved` outbox 事件。
4. Backend importer 拉取事件与内容包，校验哈希后事务性入库，成功后回写 receipt。
5. 修改产生新 revision，不覆盖已发布版本，可回滚和审计。

## 学习数据模型预留

正文保留段落边界，后续可生成句级时间轴；`learning` 字段预留 CEFR 难度、主题、重点词、语法点和复述提示。这些增强内容由 CMS 编辑或离线 NLP 作业产生，不污染来源快照。

## 内网到公有云

- 对象 key、manifest 和 checksum 均与云厂商无关。
- 当前 MinIO bucket 开启版本控制与定期备份；迁移时先全量 mirror，再增量同步，最后做 manifest/checksum 清点。
- 状态库只用于断点续抓，可备份但不是核心资产；核心资产是 bucket。
- 公有云使用私有 bucket、服务账号最小权限、服务端加密和生命周期规则。CMS/Backend 使用短期凭证读取，不暴露源音频地址给客户端。

## 内容与合规

只采集带音频且有可学习正文的页面，视频资源一律忽略。保存来源 URL、抓取时间和归属信息。是否允许对外分发必须在 CMS 审核时确认；个人学习用途不自动等同于获得再分发授权。
