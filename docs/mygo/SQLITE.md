# MyGo SQLite 存储

2026-10-15，Sidelet 0.2.0 / build 27。

build 30 已增加旧 WebView 库的显式离线迁移，下文描述最初接入时的状态；新规则见 [迁移说明](LEGACY_MIGRATION.md)。

build 43 新增独立的新建任务草稿表，SQLite `user_version` 升为 2；主窗口新建草稿支持跨窗口和重启恢复，其他编辑会话仍不持久化。下文“两表 / 草稿不保存”是 build 27 的历史范围，当前规则与回退兼容性见 [草稿持久化](NEW_TASK_DRAFT_20261023.md)。

默认数据库：`~/Library/Application Support/SideletMyGoLab/sidelet-native.sqlite3`。`-data-dir` 可指定测试目录；`-memory` 不打开数据库。

通过 `internal/storage.NativeProfile` 和项目已有的 modernc SQLite 驱动持久化。数据库采用独立 application_id 和 schema version，避免把旧 WebView 库当成原生资料。`native_tasks` 每条任务一行，以稳定 ID 保存完整字段（payload 为 JSON 编码）；`native_profile` 保存设置、顺序、撤销窗口、资料标识、提醒序号等元数据。删除任务仍保留 tombstone，不复用 ID。两表在同一事务提交；任务行仅在字段变化时更新。WAL、synchronous FULL、单连接及原有资料目录进程锁共同保证一致性。

首次发现数据库没有已提交资料时，读取同目录 `profile.json`（支持 schema 1 / 2），校验全部内容后事务导入。原 JSON 保留且不再更新。之后 SQLite 优先，旧 JSON 即使损坏也不覆盖数据库；SQLite 损坏或未知版本会报错，不静默退回旧 JSON。旧 WebView 的 `Sidelet/sidelet.sqlite3` 自动导入尚未实现。

任务、设置、排序、提醒编号和回执、完成 / 稍后 / 临时任务 / 撤销信息均持久化；输入草稿、悬停和拖动预览不持久化。提交失败恢复内存模型，保留编辑草稿；事务中断不会留下只更新元数据的部分提交。备份应先退出应用，复制整个资料目录，包括可能存在的 WAL 文件。

验证：

- 原生及共享业务完整测试、race、vet 通过；模拟 SQL trigger 在任务更新时失败，确认此前元数据更新也回滚，重开数据库结果一致。
- JSON 首次导入、schema 1 升级、保留源文件、禁止重复导入、资料 ID 与提醒身份保持、坏数据 / 未来版本拒绝均有测试。
- 原有保存失败测试改为读取数据库逻辑快照，覆盖 WAL 内的数据，避免仅比较主库文件造成假通过。
- Mac 实机隔离目录创建“SQLite 重启验收”及两行中文备注；退出后重新启动，界面恢复正确；只读 SQL 查到同一任务，`PRAGMA quick_check` 为 `ok`。
- macOS arm64 构建 / 签名、Windows amd64 / arm64 交叉构建、官网 Svelte 检查通过。实机证据：`build/results/sqlite-native-20261015/`。

未对用户日常资料执行迁移。本轮没有重新测量内存，之前的内存测试使用 `-memory`，不能直接作为 SQLite 版的测量结果。
