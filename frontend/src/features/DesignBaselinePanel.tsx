import { getDesktopBridge, type NetworkPhase } from '../bridge';
import { Button, EmptyState } from '../components/Primitives';
import uiStyles from '../components/ui.module.css';

interface DesignBaselinePanelProps {
  theme: 'light' | 'dark';
  onToggleTheme: () => void;
  lowResource: boolean;
  onToggleLowResource: () => void;
  onNetworkPhaseChange: (phase: NetworkPhase) => Promise<void>;
}

const TOKEN_SWATCHES = [
  { name: 'canvas', varName: '--canvas' },
  { name: 'surface', varName: '--surface' },
  { name: 'surface-muted', varName: '--surface-muted' },
  { name: 'ink', varName: '--ink' },
  { name: 'ink-muted', varName: '--ink-muted' },
  { name: 'accent', varName: '--accent' },
  { name: 'accent-graphic', varName: '--accent-graphic' },
  { name: 'accent-wash', varName: '--accent-wash' },
];

export function DesignBaselinePanel({
  theme,
  onToggleTheme,
  lowResource,
  onToggleLowResource,
  onNetworkPhaseChange,
}: DesignBaselinePanelProps) {
  const isMock = getDesktopBridge().isMock;

  return (
    <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', gap: '20px', maxWidth: '720px' }}>
      <section style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px' }}>
        <h2 className="serif-title" style={{ margin: 0, fontSize: '22px' }}>
          视觉基线对照
        </h2>
        <div style={{ display: 'flex', gap: '8px' }}>
          <Button variant="secondary" onClick={onToggleTheme}>
            {theme === 'light' ? '切换深色' : '切换浅色'}
          </Button>
          <Button variant="secondary" onClick={onToggleLowResource}>
            {lowResource ? '低资源：开' : '低资源：关'}
          </Button>
        </div>
      </section>

      <section style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: '8px' }}>
        {TOKEN_SWATCHES.map((sw) => (
          <div
            key={sw.name}
            style={{
              padding: '8px',
              border: '1px solid var(--line)',
              borderRadius: '8px',
              background: 'var(--surface-raised)',
            }}
          >
            <div
              style={{
                height: '28px',
                borderRadius: '6px',
                border: '1px solid var(--line)',
                background: `var(${sw.varName})`,
              }}
            />
            <div style={{ fontSize: '12px', marginTop: '6px', color: 'var(--ink-muted)' }}>{sw.name}</div>
          </div>
        ))}
      </section>

      <section style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', alignItems: 'center' }}>
        <Button variant="primary">主操作</Button>
        <Button variant="secondary">次操作</Button>
        <Button variant="ghost">轻量文字</Button>
        <Button variant="danger">清除数据</Button>
        <div style={{ width: '200px' }}>
          <input className={uiStyles.input} placeholder="控件描边与焦点环" />
        </div>
      </section>

      <section style={{ border: '1px solid var(--line)', borderRadius: '10px', background: 'var(--surface-muted)' }}>
        <EmptyState lead="没有未读话题" actionLabel="回到最新" onAction={() => {}} />
      </section>

      {isMock && (
        <section style={{ display: 'flex', flexWrap: 'wrap', gap: '8px', alignItems: 'center' }}>
          <span style={{ fontSize: '12px', color: 'var(--ink-muted)' }}>网络状态条验证：</span>
          <Button variant="ghost" onClick={() => void onNetworkPhaseChange('ready')}>
            在线
          </Button>
          <Button variant="ghost" onClick={() => void onNetworkPhaseChange('cooling_down')}>
            429 冷却
          </Button>
          <Button variant="ghost" onClick={() => void onNetworkPhaseChange('challenge_required')}>
            站点验证
          </Button>
          <Button variant="ghost" onClick={() => void onNetworkPhaseChange('offline')}>
            断网
          </Button>
        </section>
      )}
    </div>
  );
}
