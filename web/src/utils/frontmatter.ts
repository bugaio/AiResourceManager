/**
 * 极简 YAML frontmatter 解析器(仅支持 name/description 两个字段所需的子集)
 *
 * 支持的语法:
 *   name: value                    单行裸值
 *   name: "value"                  双引号
 *   name: 'value'                  单引号
 *   description: |                 字面量块(保留换行)
 *     line1
 *     line2
 *   description: >                 折叠块(换行变空格,空行变换行)
 *     line1
 *     line2
 *
 * 不支持: 嵌套对象、数组、多行流式字符串、锚点引用等(本项目场景用不到)
 */
export interface Frontmatter {
  name?: string
  description?: string
}

/** 提取 SKILL.md / agent.md 顶部 --- 包裹的 frontmatter,返回解析后的对象 */
export function parseFrontmatter(text: string): Frontmatter {
  const m = text.match(/^---\s*\r?\n([\s\S]*?)\r?\n---/)
  if (!m) return {}
  const lines = m[1].split(/\r?\n/)
  const out: Record<string, string> = {}

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    // 顶层 key 必须顶格(无前导空格),value 部分允许任意内容
    const km = line.match(/^([A-Za-z_][\w-]*)\s*:\s?(.*)$/)
    if (!km) continue

    const key = km[1]
    const rawVal = km[2]
    const trimmed = rawVal.trim()

    // 块标量: | 或 |- 或 |+ → 字面量(保留换行); > 或 >- 或 >+ → 折叠
    const blockMatch = trimmed.match(/^([|>])([+-]?)\s*$/)
    if (blockMatch) {
      const folded = blockMatch[1] === '>'
      const collected: string[] = []
      // 探测块的缩进基线 = 后续第一个非空行的前导空格数
      let baseIndent = -1
      let j = i + 1
      while (j < lines.length) {
        const cur = lines[j]
        if (cur.trim() === '') {
          // 空行先放着,等遇到首个非空行确定缩进后再决定怎么收
          collected.push('')
          j++
          continue
        }
        const indent = cur.match(/^(\s*)/)![1].length
        if (baseIndent === -1) {
          // 块内必须有缩进(>0),否则块结束
          if (indent === 0) break
          baseIndent = indent
        } else if (indent < baseIndent) {
          // 缩进退回 → 块结束
          break
        }
        collected.push(cur.slice(baseIndent))
        j++
      }
      // 去掉收集到的尾部空行(YAML 默认 clip: 保留单个尾换行,但展示用途够了)
      while (collected.length > 0 && collected[collected.length - 1] === '') {
        collected.pop()
      }
      let val: string
      if (folded) {
        // 折叠: 连续非空行合并为空格,空行 → 换行
        val = collected
          .reduce<string[]>((acc, cur) => {
            if (cur === '') {
              acc.push('')
            } else if (acc.length === 0 || acc[acc.length - 1] === '') {
              acc.push(cur)
            } else {
              acc[acc.length - 1] = acc[acc.length - 1] + ' ' + cur
            }
            return acc
          }, [])
          .join('\n')
      } else {
        val = collected.join('\n')
      }
      out[key] = val
      i = j - 1
      continue
    }

    // 单行裸值,去引号
    let val = trimmed
    if (
      (val.startsWith('"') && val.endsWith('"') && val.length >= 2) ||
      (val.startsWith("'") && val.endsWith("'") && val.length >= 2)
    ) {
      val = val.slice(1, -1)
    }
    out[key] = val
  }

  return { name: out.name, description: out.description }
}
