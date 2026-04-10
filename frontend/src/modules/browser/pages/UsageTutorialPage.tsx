import { BookOpen, Download, Globe, Keyboard, Layers, Monitor } from 'lucide-react'
import { Card } from '../../../shared/components'
import type { ReactNode } from 'react'

function StepCard({
  icon,
  title,
  children,
}: {
  icon: ReactNode
  title: string
  children: ReactNode
}) {
  return (
    <Card>
      <div className="flex items-start gap-3">
        <div className="w-9 h-9 rounded-lg bg-[var(--color-accent-muted)] text-[var(--color-accent)] flex items-center justify-center shrink-0">
          {icon}
        </div>
        <div className="flex-1 min-w-0">
          <h2 className="text-base font-semibold text-[var(--color-text-primary)]">{title}</h2>
          <div className="mt-2 text-sm text-[var(--color-text-secondary)] leading-relaxed space-y-2">{children}</div>
        </div>
      </div>
    </Card>
  )
}

export function UsageTutorialPage() {
  return (
    <div className="space-y-5 animate-fade-in">
      <Card>
        <div className="flex items-start justify-between gap-4 flex-wrap">
          <div>
            <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-[var(--color-accent-muted)] text-[var(--color-accent)] text-xs font-medium mb-3">
              <BookOpen className="w-3.5 h-3.5" /> 使用教程
            </div>
            <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">从 0 到可用：内核、代理池、实例启动</h1>
            <p className="text-sm text-[var(--color-text-secondary)] mt-2">
              按照下面步骤，你可以完成内核下载、代理池配置、实例创建与启动。
            </p>
          </div>
        </div>
      </Card>

      <StepCard icon={<Download className="w-4 h-4" />} title="1) 下载并准备浏览器内核">
        <p>请优先使用应用内置下载功能准备内核：</p>
        <pre className="text-xs font-mono bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
{`左侧菜单 -> 指纹浏览器 -> 内核管理 -> 下载内核`}
        </pre>
        <p>下载完成后，内核会自动出现在“内核管理”列表中。</p>
      </StepCard>

      <StepCard icon={<Layers className="w-4 h-4" />} title="2) 在“内核管理”中确认可用内核">
        <p>进入左侧 <code>指纹浏览器 &gt; 内核管理</code>。</p>
        <p>确认已下载完成的内核状态正常，并设置一个默认内核。</p>
        <p>如下载异常，请直接重新发起下载。</p>
      </StepCard>

      <StepCard icon={<Globe className="w-4 h-4" />} title="3) 创建代理池（HTTP/SOCKS5/Vmess/Vless/Trojan）">
        <p>进入左侧 <code>指纹浏览器 &gt; 代理池配置</code>。</p>
        <p>你可以逐条添加代理，也可以通过 YAML 批量导入。</p>
        <p>保存后，实例编辑页里即可选择这些代理节点。</p>
      </StepCard>

      <StepCard icon={<Monitor className="w-4 h-4" />} title="4) 创建实例并启动">
        <p>进入 <code>指纹浏览器 &gt; 实例列表</code>，点击“新建配置”。</p>
        <p>设置实例名称、选择内核、选择代理（可选）、调整启动参数。</p>
        <p>保存后返回列表，点击“启动”即可运行浏览器实例。</p>
      </StepCard>

      <StepCard icon={<Keyboard className="w-4 h-4" />} title="5) 使用快捷键快速启动">
        <p>在应用内按 <code>Ctrl + K</code>（Mac 为 <code>Cmd + K</code>）即可呼出“快速启动浏览器”弹窗。</p>
        <p>你可以通过实例名、标签和关键字快速检索目标实例，并直接启动。</p>
        <p>弹窗内支持键盘操作：</p>
        <pre className="text-xs font-mono bg-[var(--color-bg-secondary)] border border-[var(--color-border-muted)] rounded-lg p-3 overflow-x-auto">
{`Ctrl/Cmd + K  呼出/收起快速启动弹窗
Enter          启动当前选中实例
↑ / ↓          在实例列表中切换选中项
Esc            关闭弹窗`}
        </pre>
      </StepCard>
    </div>
  )
}
