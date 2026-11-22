import type { ReactNode } from 'react'
import { Cpu, Globe, Keyboard, Monitor, Tags } from 'lucide-react'
import { SECTION_WIDTH, SectionHeader } from '../../../shared/layout/SectionPage'

function Step({ n, icon, title, children }: { n: number; icon: ReactNode; title: string; children: ReactNode }) {
  return (
    <li className="flex gap-3 px-4 py-3">
      <span className="w-7 h-7 shrink-0 rounded-md bg-[var(--color-bg-muted)] text-[var(--color-text-secondary)] flex items-center justify-center">
        {icon}
      </span>
      <div className="min-w-0">
        <h3 className="text-[13px] font-medium text-[var(--color-text-primary)]">
          <span className="text-[var(--color-text-muted)] tabular-nums mr-1.5">{n}.</span>
          {title}
        </h3>
        <div className="mt-1 text-xs leading-5 text-[var(--color-text-secondary)] space-y-1">{children}</div>
      </div>
    </li>
  )
}

function Kbd({ children }: { children: ReactNode }) {
  return <kbd className="px-1.5 py-0.5 rounded bg-[var(--color-bg-muted)] text-[11px] font-mono text-[var(--color-text-secondary)]">{children}</kbd>
}

export function GuideSection() {
  return (
    <div className={SECTION_WIDTH}>
      <SectionHeader title="使用教程" description="从零开始: 准备内核、添加代理、创建并启动实例" />
      <ol className="rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] divide-y divide-[var(--color-border-muted)]">
        <Step n={1} icon={<Cpu className="w-4 h-4" />} title="准备浏览器内核">
          <p>打开左侧「内核」，在「可下载内核」里下载一个，下载完成后设为默认。</p>
          <p>实例的内核选「跟随默认」时，以后切换默认内核会对这些实例一起生效。</p>
        </Step>
        <Step n={2} icon={<Globe className="w-4 h-4" />} title="添加代理">
          <p>打开左侧「代理」，添加 HTTP / HTTPS / SOCKS5 代理，带账号密码的也可以直接填。</p>
          <p>可以给代理设分组和标签，测速后能看到延迟与出口地区。</p>
        </Step>
        <Step n={3} icon={<Monitor className="w-4 h-4" />} title="创建实例并启动">
          <p>打开左侧「实例」，点「新建实例」，填名称、选代理，其余保持默认即可。</p>
          <p>保存后在列表里点启动按钮运行浏览器。新实例的默认内核、代理和指纹在「设置 → 新实例默认值」里改。</p>
        </Step>
        <Step n={4} icon={<Tags className="w-4 h-4" />} title="用分组和标签整理">
          <p>实例和代理都能放进一个分组、打多个标签；列表顶部点分组快速切换，标签用来筛选。</p>
          <p>分组与标签可以在「设置 → 分组与标签」里统一重命名或删除。</p>
        </Step>
        <Step n={5} icon={<Keyboard className="w-4 h-4" />} title="快捷键快速启动">
          <p>
            按 <Kbd>Ctrl</Kbd> + <Kbd>K</Kbd> 打开快速启动，输入实例名或标签搜索，
            <Kbd>↑</Kbd> <Kbd>↓</Kbd> 选择，<Kbd>Enter</Kbd> 启动，<Kbd>Esc</Kbd> 关闭。
          </p>
        </Step>
      </ol>
    </div>
  )
}
