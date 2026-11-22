import { askConfirm } from './confirm'
import { errorCode } from './errors'

// 启动同步空间里的实例: 团队里有人正在用 (40915)、本人另一台电脑在用 (40917)、或云端登录态下载失败时, 问用户是否仍要启动。

const downloadFailures = new Set([50401, 50402, 50403, 50404, 40920, 40924])

export async function guardedStart<T>(start: () => Promise<T>, startForce: () => Promise<T>): Promise<T | null> {
  try {
    return await start()
  } catch (err) {
    const code = errorCode(err)
    if (code === 40915 || code === 40917) {
      const mine = code === 40917
      const ok = await askConfirm({
        title: mine ? '实例正在另一台电脑上使用' : '实例正在被使用',
        content: mine
          ? '你在另一台电脑上开着这个实例。两边同时打开会让登录态互相覆盖，建议先在那台电脑上关闭它。'
          : '团队里有人正在使用这个实例。同时打开会让两边的登录态互相覆盖，建议先联系对方关闭。',
        confirmText: '仍要启动',
        danger: true,
      })
      return ok ? await startForce() : null
    }
    if (downloadFailures.has(code)) {
      const ok = await askConfirm({
        title: '云端登录态没能下载',
        content: '云端有这个实例更新的登录态，但现在下载不了。用本机的旧数据启动的话，关闭后会被云端版本覆盖。',
        confirmText: '用本机数据启动',
      })
      return ok ? await startForce() : null
    }
    throw err
  }
}
