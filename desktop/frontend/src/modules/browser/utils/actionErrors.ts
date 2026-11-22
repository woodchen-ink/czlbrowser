import { errorText } from '../../cloud/errors'

/** 操作失败的提示: 后端给了原因就用原因 (带数字码的转中文), 没给就提示去哪里看。 */
export function actionError(err: unknown, action: string): string {
  const raw = typeof err === 'string' ? err : err instanceof Error ? err.message : ''
  if (raw.trim()) return errorText(raw.trim())
  return `${action}, 没有返回原因。可在实例详情的"最近错误"或 设置 → 运行日志 里查看`
}
