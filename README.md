# 黑名单邮箱索引工具

社区用户查询黑名单邮箱、管理员维护黑名单与公告的轻量级 Web 工具。工具属性优先，注重页面美观度与交互体验。

## 功能特性

- **公开查询**：输入邮箱即时返回是否在黑名单中，展示拉黑原因（Markdown）、事件外链、相关人、拉黑时间
- **公告展示**：首页公告卡片，Markdown 渲染，无公告自动隐藏
- **管理后台**：黑名单增删改查、软删除回收站（可还原/永久删除）、公告编辑与即时预览、审计日志
- **安全**：密码 bcrypt（cost 12）、TOTP 双因素认证（RFC 6238）、JWT（HttpOnly Cookie）、登录失败限流、SQL 占位符防注入、Markdown 消毒
- **部署友好**：纯 Go SQLite 驱动（无 CGO）、Docker 多阶段构建、非 root 运行、/data 持久化

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25+ |
| 数据库 | SQLite（`modernc.org/sqlite`，纯 Go，WAL 模式） |
| 认证 | bcrypt + TOTP(RFC 6238) + JWT(HS256) |
| Markdown | `github.com/yuin/goldmark`（服务端渲染，默认转义原始 HTML） |
| 前端 | 原生 HTML/CSS/JS，无第三方 UI 框架；marked.js + DOMPurify（CDN） |
| 部署 | Docker 多阶段构建、docker-compose |

## 目录结构

```
.
├── main.go / embed.go          # 入口、静态资源嵌入
├── internal/
│   ├── config/                 # 环境变量加载与校验
│   ├── db/                     # 连接、迁移、完整性检查
│   ├── model/                  # 数据模型
│   ├── repository/             # 数据访问层
│   ├── service/                # 业务逻辑（auth/totp/jwt/markdown/…）
│   ├── handler/                # HTTP 处理
│   ├── middleware/              # 认证 / 日志 / 恢复
│   ├── logger/                 # 按日期滚动日志写入器
│   └── server/                 # 路由与静态资源
├── web/                        # 前端静态资源（嵌入二进制）
│   ├── index.html              # 首页
│   ├── admin.html              # 管理后台（单页）
│   └── assets/                 # css / js
├── Dockerfile
├── docker-compose.yml
└── go.mod / go.sum
```

## 快速开始

### Docker（推荐）

```bash
# 1. 修改 docker-compose.yml 中的 ADMIN_PASSWORD 与 TOTP_SECRET
# 2. 构建并启动
docker compose up -d --build

# 3. 查看日志，获取后台路径与 TOTP 绑定 URL
docker compose logs -f blacklist-index
```

首次启动时日志会输出：

- 后台管理入口：`dashboard` 字段对应的路径（形如 `/<8位随机串>`）
- TOTP 绑定 URL：`otpauth://...`，用 Google Authenticator / 1Password 等扫码后即可登录

### 本地运行

```bash
export ADMIN_PASSWORD='Aa12345678.'
export TOTP_SECRET='JBSWY3DPEHPK3PXP'   # 请替换为随机 Base32 密钥
export TIMEZONE='Asia/Shanghai'
export SITE_NAME='邮箱黑名单查询'
export PORT=8080

go run .
```

## 环境变量

| 变量名 | 说明 | 必填 | 默认值 |
|--------|------|------|--------|
| `ADMIN_PASSWORD` | 管理员密码（≥8 位，含大小写字母与数字） | 是 | - |
| `TOTP_SECRET` | TOTP 密钥（Base32，≥16 字符） | 是 | - |
| `TIMEZONE` | 时区（如 `Asia/Shanghai`） | 否 | `Asia/Shanghai` |
| `PORT` | 服务端口 | 否 | `8080` |
| `SITE_NAME` | 站点名称 | 否 | 邮箱黑名单查询 |
| `DATA_DIR` | 数据目录（数据库与日志） | 否 | `./data` |

> `ADMIN_PASSWORD` 与 `TOTP_SECRET` 仅在**首次启动创建管理员**时使用；之后修改环境变量不会覆盖已存在的管理员。

## 登录说明

- 用户名固定为 `admin`
- 登录需「管理员密码 + TOTP 动态口令」
- 连续失败 5 次将锁定该 IP 15 分钟
- 登录成功后签发 JWT（HttpOnly Cookie，8 小时有效）
- 所有登录/退出/增删/公告操作均写入审计日志

## API 接口

### 公开

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/check?email=x` | 查询邮箱（返回 `blocked`、`reason_html` 等） |
| GET | `/api/v1/announcement` | 获取生效公告（返回 `content_html`） |
| GET | `/health` | 健康检查（`status`、`db`、`uptime`） |

### 管理（需登录 JWT Cookie）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/login` | 登录 |
| POST | `/api/v1/admin/logout` | 退出登录 |
| GET | `/api/v1/admin/status` | 登录状态 |
| GET | `/api/v1/admin/list?q=&deleted=&page=&page_size=` | 黑名单列表 |
| POST | `/api/v1/admin/add` | 新增黑名单 |
| DELETE | `/api/v1/admin/delete/{id}` | 软删除（进回收站） |
| POST | `/api/v1/admin/restore/{id}` | 从回收站恢复 |
| DELETE | `/api/v1/admin/permanent/{id}` | 永久删除 |
| GET | `/api/v1/admin/announcement` | 获取公告原始 Markdown |
| PUT | `/api/v1/admin/announcement` | 保存公告 |
| GET | `/api/v1/admin/audit-logs?action=&page=&page_size=` | 审计日志 |

## 安全说明

- 所有 SQL 查询使用占位符，防止注入
- 密码 bcrypt 加密（cost ≥ 12）
- TOTP 标准 RFC 6238，允许前后各 1 个时间窗口容差
- JWT Cookie：`HttpOnly`、`SameSite=Lax`；`Secure` 在 HTTPS / `X-Forwarded-Proto: https` 时启用
- 管理后台路径为随机 8 位字符串，存于 `app_config` 表
- Docker 以非 root 用户运行
- 服务端 Markdown 用 goldmark 渲染（默认转义原始 HTML），前端再用 DOMPurify 消毒（纵深防御）
- 登录失败限流：同一 IP 连续失败 5 次锁定 15 分钟

## 设计说明

- **软删除**：黑名单表在需求基础上增加 `deleted_at` 列以支持回收站；`email` 的唯一性通过**部分唯一索引**（仅约束未删除记录）实现，从而支持删除后重新添加同名邮箱
- **JWT 密钥**：首次启动随机生成并持久化到 `app_config` 表，重启不失效
- **审计保留策略**：保留最近 10,000 条，每写入 100 条触发一次清理
- **时间处理**：所有时间以 `TIMEZONE` 配置时区存储与展示

## License

[MIT](LICENSE)