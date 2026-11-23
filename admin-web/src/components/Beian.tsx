/**
 * 备案号（ICP 合规要求）。
 * 备案通过后站点对公网可见，页脚必须展示备案号并链接到工信部。
 * 号码与 OpenChronicleAtlas 共用（同属 openapp.asia 备案），换证/换号时只改这里。
 */
export const ICP_NUMBER = '粤ICP备2026058093号-2'

export default function Beian({ style }: { style?: React.CSSProperties }) {
  return (
    <div style={{ textAlign: 'center', fontSize: 12, color: '#8c8c8c', ...style }}>
      <a
        href="https://beian.miit.gov.cn"
        target="_blank"
        rel="noreferrer"
        style={{ color: 'inherit' }}
      >
        {ICP_NUMBER}
      </a>
    </div>
  )
}
