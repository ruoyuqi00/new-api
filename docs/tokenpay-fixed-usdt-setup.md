# TokenPay 固定 USDT 套餐配置

YuAPI 的 TokenPay 配置独立于易支付、Stripe 和 SHKeeper。它只负责配置套餐、签名建单、验证回调与幂等入账；钱包和扫链位于单独部署的 TokenPay 服务，不能把钱包私钥或数据库放在 YuAPI 中。

后台填写 TokenPay 的 HTTPS 服务域名、与服务端一致的 HMAC-SHA256 API 密钥、允许的网络和固定套餐。例如 `10 USDT -> 66 站内余额`。密钥留空保存时保留原值；页面只显示是否已配置，不回显密钥。TokenPay 服务端必须设置 `BaseCurrency=USD`、`Rate:USDT=1`、`UseDynamicAddress=true`、`DynamicAddressConfig:AmountMove=false`、`Signature:UseHmacSha256=true`，并关闭自动归集。服务的 `WebSiteUrl` 与后台填写的服务域名须一致。YuAPI 对外回调地址必须是 HTTPS 且 TokenPay 可达。

网络对应币种为 TRC20 `USDT_TRC20`、BSC `EVM_BSC_USDT_BEP20`、Polygon `EVM_Polygon_USDT_ERC20`。必须使用独立 TokenPay 分支：它在建单 `info` 返回绝对时间 `ExpireTimeUnix`；YuAPI 拒绝仅含无时区 `ExpireTime` 文本的原版响应，避免容器时区不同导致错误展示收款地址。BSC 和 Polygon 还需要该分支的 JSON-RPC 扫描适配；普通 TokenPay v1.2.0 不能把 JSON-RPC 地址填入 `ApiHost` 后直接使用。Polygon USDT 指定合约为 `0xc2132d05d31c914a87c6611c10748aeb04b58e8f`，与其他同名代币不能混用。TronGrid 未配置可用 API Key 时可能返回 429。

用户提交交易哈希只进入人工核对，每单最多三条不同哈希；原记录保留，YuAPI 后台 TokenPay 设置页按每页 50 条显示审核记录，可继续加载更早记录。提交本身不产生充值。成功入账必须由 TokenPay 已付款的签名回调触发，并匹配冻结的订单网络、地址和精确金额。上游建单结果不明时，YuAPI 使用原订单号限频恢复；在确认收款地址之前页面不会展示二维码或付款按钮。未发放地址的订单到期关闭。回调失败超过 TokenPay 的三次自动尝试后，操作员在其管理界面重试；过期订单先查链再补单。动态地址私钥位于 TokenPay SQLite，生产前应完成权限隔离、加密备份和恢复演练。BSC/Polygon 地址中的 USDT 不会自动归集，转出需要链上 gas。

代码与模拟测试不等于链上支付可用。每条链均须单独通过小额真实付款、回调、单次充值、故障恢复测试后才启用。当前阶段不部署、不热切换，也不操作其他项目的容器或实例。
