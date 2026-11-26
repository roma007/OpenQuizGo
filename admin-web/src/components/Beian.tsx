/**
 * 备案号（ICP + 公安联网备案，合规要求）。
 * 备案通过后站点对公网可见，页脚必须展示备案号并链接官方查询页。
 * 号码与 OpenChronicleAtlas、OpenApp 官网共用（同属 openapp.asia 备案），换证/换号时只改这里。
 */
export const ICP_NUMBER = '粤ICP备2026058093号-2'
export const PSB_NUMBER = '粤公网安备44011302006250号'
export const PSB_HREF =
  'https://beian.mps.gov.cn/#/query/webSearch?code=44011302006250'

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
      <span style={{ margin: '0 6px' }}>·</span>
      <a
        href={PSB_HREF}
        target="_blank"
        rel="noreferrer"
        style={{ color: 'inherit', whiteSpace: 'nowrap' }}
      >
        <img
          src="/beian.png"
          alt=""
          width={15}
          height={17}
          style={{ height: 16, width: 'auto', verticalAlign: '-3px', marginRight: 4 }}
        />
        {PSB_NUMBER}
      </a>
    </div>
  )
}
