// Package errcode 定义云端功能 (同步、团队、付费、端到端加密) 的对外错误标识。
//
// 机器层一律「数字码 + snake_case 英文短语」, 前端按码映射中文并展示数字码 (frontend/src/modules/cloud/errors.ts)。
// 原始异常 (HTTP 响应、SQLite 报错、文件路径) 只进日志, 不随 Error() 外泄。
// 号段: 400xx 请求不合法, 403xx 无权限, 404xx 不存在, 409xx 状态冲突, 503xx/504xx/505xx/506xx 外部服务。
package errcode

import "fmt"

// Error 是对外错误。Code 稳定, Phrase 是补充说明, cause 只用于日志。
type Error struct {
	Code   int    `json:"code"`
	Phrase string `json:"phrase"`
	cause  error
}

func (e *Error) Error() string { return fmt.Sprintf("%d %s", e.Code, e.Phrase) }

func (e *Error) Unwrap() error { return e.cause }

// Is 按错误码判等, 包装过的错误也认得出。
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Code == e.Code
}

// Cause 取原始异常, 只给日志用。
func (e *Error) Cause() error { return e.cause }

// New 定义一个错误码。
func New(code int, phrase string) *Error { return &Error{Code: code, Phrase: phrase} }

// Wrap 附上原始异常, Error() 仍只有码与短语。
func (e *Error) Wrap(cause error) *Error { return &Error{Code: e.Code, Phrase: e.Phrase, cause: cause} }

// 本地存储与编解码。
var (
	ErrDBQuery      = New(50005, "db_query_failed")
	ErrPayloadCodec = New(50006, "payload_codec_failed")
	ErrFileWrite    = New(50007, "file_write_failed")
)

// 登录。
var (
	// ErrAuthRequired 没有登录
	ErrAuthRequired = New(40100, "auth_required")
	// ErrAuthExpired 登录已失效, 需重新登录
	ErrAuthExpired = New(40104, "auth_session_expired")
)

// 空间与同步。
var (
	// ErrSpaceUnknown 空间不存在或已无权访问
	ErrSpaceUnknown = New(40404, "space_unknown")
	// ErrSpaceSame 复制的来源与目标是同一个空间
	ErrSpaceSame = New(40028, "space_same")
	// ErrSpaceMismatch 同一 id 已在本机另一个空间里
	ErrSpaceMismatch = New(40904, "space_id_conflict")
	// ErrSyncReadOnly 空间只读 (只读成员、订阅到期、组织超席位、已被移出)
	ErrSyncReadOnly = New(40905, "sync_space_read_only")
	// ErrLogoutUnpushed 同步空间里还有没推送的修改, 退出登录前需确认
	ErrLogoutUnpushed = New(40906, "logout_blocked_by_unpushed")
	// ErrLeaveUnpushed 离开团队前团队空间里还有没推送的修改
	ErrLeaveUnpushed = New(40910, "leave_blocked_by_unpushed")
	// ErrTeamSettingsAdminOnly 团队书签只有管理员能改
	ErrTeamSettingsAdminOnly = New(40911, "team_settings_admin_only")
	// ErrSyncNotEnabled 应用未开通数据同步 (服务端 40303)
	ErrSyncNotEnabled = New(40912, "sync_not_enabled")
	// ErrProfileInUse 实例正在被团队里的其他人使用
	ErrProfileInUse = New(40915, "profile_in_use")
	// ErrProfileInUseElsewhere 实例正在本人的另一台电脑上使用
	ErrProfileInUseElsewhere = New(40917, "profile_in_use_elsewhere")
	// ErrProfileRunning 实例正在运行, 不能做这个操作
	ErrProfileRunning = New(40916, "profile_running")
	// ErrSyncAccountDisabled 账号已被封禁
	ErrSyncAccountDisabled = New(40305, "sync_account_disabled")
	// ErrSyncUnavailable 同步服务连不上, 修改留在本机
	ErrSyncUnavailable = New(50401, "sync_unavailable")
	// ErrSyncPayload 拉取到的文档解不开或格式不认识
	ErrSyncPayload = New(50402, "sync_payload_invalid")
	// ErrSyncRequest 同步服务返回了意料之外的错误
	ErrSyncRequest = New(50403, "sync_request_failed")
	// ErrStatePack 打包或解包登录态失败
	ErrStatePack = New(50404, "profile_state_pack_failed")
	// ErrStateTooLarge 登录态超过上限
	ErrStateTooLarge = New(41302, "profile_state_too_large")
)

// 端到端加密。
var (
	// ErrKeysLocked 本机还没有解锁加密密钥
	ErrKeysLocked = New(40920, "e2e_locked")
	// ErrKeysNotEnrolled 还没开通端到端加密
	ErrKeysNotEnrolled = New(40921, "e2e_not_enrolled")
	// ErrKeysAlreadyEnrolled 别的设备已经开通过, 应改用恢复码或设备批准
	ErrKeysAlreadyEnrolled = New(40922, "e2e_already_enrolled")
	// ErrRecoveryCodeInvalid 恢复码格式或校验位不对
	ErrRecoveryCodeInvalid = New(40023, "recovery_code_invalid")
	// ErrRecoveryCodeWrong 恢复码与账号不符
	ErrRecoveryCodeWrong = New(40024, "recovery_code_wrong")
	// ErrDeviceRequestExpired 新设备授权请求已过期或已被拒绝
	ErrDeviceRequestExpired = New(40923, "device_request_expired")
	// ErrDeviceRequestUnknown 授权请求不存在
	ErrDeviceRequestUnknown = New(40405, "device_request_unknown")
	// ErrSpaceKeyPending 团队密钥还没授予本人, 等管理员在线
	ErrSpaceKeyPending = New(40924, "space_key_pending")
	// ErrKeyStore 本机密钥库读写失败
	ErrKeyStore = New(50405, "key_store_failed")
)

// 团队。
var (
	ErrOrgRejected  = New(40025, "org_request_rejected")
	ErrOrgForbidden = New(40301, "org_forbidden")
	ErrOrgNotFound  = New(40406, "org_not_found")
	ErrOrgRequest   = New(50501, "org_request_failed")
)

// 付费。
var (
	ErrBillingRejected  = New(40027, "billing_request_rejected")
	ErrBillingPeriods   = New(40030, "billing_periods_unsupported")
	ErrBillingForbidden = New(40303, "billing_forbidden")
	ErrBillingDisabled  = New(40913, "billing_disabled")
	ErrBillingRequest   = New(50601, "billing_request_failed")
	// ErrBillingTooMany 付费接口限流 (429)
	ErrBillingTooMany = New(42900, "billing_too_many_requests")
	// ErrRedeemInvalid 兑换码不存在、已用过、已过期或不适用
	ErrRedeemInvalid = New(40031, "redeem_code_invalid")
	// ErrRedeemLocked 兑换连续失败太多次, 暂时锁定
	ErrRedeemLocked = New(42901, "redeem_locked")
)
