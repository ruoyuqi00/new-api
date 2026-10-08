# 分组降智监测生产热切换

2026-10-08，Asia/Shanghai。用户验收本地界面后明确批准热切换。

## 发布标识

- 源码：`15f9b987c`，已推送 `fork` 的 `codex/tokenpay-fixed-package-20260927`。
- 源码归档 SHA-256：`2c021b8356665f4051448feff8380a9e6e2adef8c647fca72387adb937536a49`。
- 运行版本：`15f9b987c-quality-monitor-20261008`。
- 镜像：`yuapi:production-quality-monitor-20261008-15f9b987c`。
- 镜像 ID：`sha256:426eddc9097e09497432a11923c8552905e87264a7bfe37cd39cf9c133573d01`。
- Linux 二进制 SHA-256：`7530b9d967bbea29a82e10c9ee71b48a67f0c0aa4bde0b8e3d79816ec8c9cf4b`。
- 最终主实例：`yuapi-production-quality-monitor-15f9b987c`，私有端口 `127.0.0.1:13068`，启动于 `2026-10-08T13:18:52.247646728Z`。
- 前端主资源：`index.2299a036e6.js`，SHA-256 `f9ff62a2dd4ccf4ba006d29aa9d183bb192932ee9805ab289b315b1ac9b606fb`。

复用已验证生产镜像运行环境，仅替换应用二进制和许可证副本。前端为用户验收的实际品牌构建；保护的项目归属信息完整保留。未启动其他项目、Docker Compose 或额外数据库服务。

## 数据库

生产 MySQL 任务表约 5,300 条、数据 3.7 MB，迁移前无未提交 InnoDB 事务。先复制任务表结构到本次专用临时表，通过实际 GORM 列扩容及 82,510 字节最大配置快照的保存/读取一致性验证，再删除临时表。

于 `13:00:47–13:00:48 UTC` 创建监测计划和结果表，并将 `system_tasks.payload` 扩容至 LONGTEXT。只有这些表/列由专用迁移工具修改；迁移连接设置五秒元数据锁等待上限。正式主实例随后正常完成现有 AutoMigrate。计划与答案没有导入本地示例，切换后生产监测计划和结果均为零。

## 切换与核验

先启动禁止执行后台任务的 slave 候选 `yuapi-quality-monitor-candidate-15f9b987c`，监听 `127.0.0.1:13066`，验证健康状态、品牌资源哈希、六个页面和监测接口鉴权后，热加载本站两个 Caddy 路由。

旧容器 `yuapi-production-output-budget-59d352031` 保留存量请求。最后一条长连接仍有数据活动，因此最终主实例使用另一空闲私有端口 13068，避免抢占旧端口或中断请求。最终 master 启动、接管新请求后，旧实例达到连续三次零连接并正常停止。候选截至最后核验仍有一条存量连接；服务器后台 `finish-drain.sh` 在同样连续三次零连接标准达成后停止它并再次核验，完成标记为备份目录中的 `drain-finished`，过程写入 `drain-background.log`。没有强关连接或按超时截止停止处理中的实例。

最终主实例为 master，`SYSTEM_TASK_RUNNER_ENABLED=true`，指定执行节点为其自身；启动日志确认任务执行器运行。三个域名 `api.yuaiapi.com`、`yuaiapi.com`、`vip.yuaiapi.com` 均返回新版本，Caddy 持久化配置和运行时配置均指向最终容器。候选/最终主资源哈希与本地验收产物一致。

36 个受保护的既有容器 ID、启动时间、重启数及状态快照完全一致，其中包括 Sub2API、TokenPay 及其依赖。价格选项快照完全一致。切换窗口检查未发现本站代理 502/503/504 或最终应用 panic/fatal/schema 错误；这是本次观察结果，不代表后续上游永远无故障。

## 恢复

- 发布目录：`/opt/newapi/releases/quality-monitor-20261008-15f9b987c/`。
- 私有备份：`/opt/newapi/backups/20261008-quality-monitor-15f9b987c/`，包含原任务表快照、容器环境、价格/Caddy 快照和 `hot-switch.sh`。
- 应用回滚命令：`bash hot-switch.sh rollback`。备份快照仅供审计与恢复参考，不应覆盖实时余额/消费数据。
- 新表及 LONGTEXT 扩容为向后兼容追加，但旧版本 AutoMigrate 可能尝试缩回 TEXT。脚本在旧实例已停且现有任务快照超过 65,535 字节时拒绝盲目重启旧版，必须先保留扩容列兼容性；不可截断新任务数据。

线上入口：`https://yuaiapi.com/quality-monitor`。新计划默认暂停且私有；用户可自行配置分组、题目和探测间隔，监测调用产生上游成本而不扣用户余额。
