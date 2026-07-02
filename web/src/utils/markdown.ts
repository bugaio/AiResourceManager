import { marked } from 'marked'
import DOMPurify from 'dompurify'

/** 把 markdown 文本渲染为「已净化」的 HTML 字符串
 *
 * 用于展示用户可编辑的文本（如分组说明）。因内容可编辑属不可信输入，
 * 必须经 DOMPurify 净化后再 v-html，防止 XSS。
 * - marked 同步模式解析（关闭 async）
 * - 换行按 GitHub 风格：单换行即 <br>
 */
export function renderMarkdown(src: string): string {
  if (!src) return ''
  const raw = marked.parse(src, { async: false, breaks: true, gfm: true }) as string
  return DOMPurify.sanitize(raw)
}

/**
 * 渲染 markdown 并为每个顶层块级元素注入 data-src-line（该块在源码中的起始行，从 0 计）。
 * 用于编辑区与预览区的「按内容滚动同步」：预览侧据此把源码行映射到像素位置。
 *
 * 实现：marked 先 lexer 分词，逐个顶层 token 用 raw 累加换行数得到起始行，
 * 再单独 parse 成 html，用一层带 data-src-line 的 <div> 包裹。
 */
export function renderMarkdownWithLines(src: string): string {
  if (!src) return ''
  const tokens = marked.lexer(src, { gfm: true, breaks: true })
  let line = 0
  const parts: string[] = []
  for (const token of tokens) {
    const startLine = line
    // 累加该 token 占用的行数（raw 内的换行数）
    const raw = (token as any).raw ?? ''
    line += countLines(raw)
    // space token 只是空行，不产出可视元素，跳过但行数已累加
    if (token.type === 'space') continue
    const html = marked.parser([token as any], { gfm: true, breaks: true }) as string
    parts.push(`<div class="md-block" data-src-line="${startLine}">${html}</div>`)
  }
  return DOMPurify.sanitize(parts.join(''), { ADD_ATTR: ['data-src-line'] })
}

/** 统计字符串包含的换行数（用于推进源码行号） */
function countLines(s: string): number {
  let n = 0
  for (let i = 0; i < s.length; i++) {
    if (s.charCodeAt(i) === 10) n++
  }
  return n
}
