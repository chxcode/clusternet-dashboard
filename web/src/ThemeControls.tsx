import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { Moon, Palette, Sun } from 'lucide-react';
import './theme.css';

const modes = { system: '跟随系统', light: '白天', dark: '夜间' } as const;
const accents = { teal: '松绿', blue: '海蓝', violet: '鸢紫' } as const;
type Appearance = { mode: keyof typeof modes; accent: keyof typeof accents };
const storageKey = 'clusternet.appearance';
const defaultAppearance: Appearance = { mode: 'system', accent: 'teal' };
const darkQuery = '(prefers-color-scheme: dark)';

function readAppearance(): Appearance {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) || '{}');
    return {
      mode: Object.keys(modes).includes(saved.mode) ? saved.mode : defaultAppearance.mode,
      accent: Object.keys(accents).includes(saved.accent) ? saved.accent : defaultAppearance.accent,
    };
  } catch {
    return defaultAppearance;
  }
}

export function ThemeControls() {
  const [appearance, setAppearance] = useState(readAppearance);
  const [systemDark, setSystemDark] = useState(() => window.matchMedia?.(darkQuery).matches ?? false);
  const options = useRef<HTMLDetailsElement>(null);
  const trigger = useRef<HTMLElement>(null);
  const dark = appearance.mode === 'system' ? systemDark : appearance.mode === 'dark';
  const toggleTitle = dark ? '切换到白天模式' : '切换到夜间模式';
  const ToggleIcon = dark ? Sun : Moon;

  useLayoutEffect(() => {
    document.documentElement.dataset.accent = appearance.accent;
    document.documentElement.dataset.mode = dark ? 'dark' : 'light';
  }, [appearance.accent, dark]);

  useEffect(() => {
    const query = window.matchMedia?.(darkQuery);
    const syncMode = () => setSystemDark(query?.matches ?? false);
    const syncStorage = (event: StorageEvent) => {
      if (event.key === storageKey || event.key === null) setAppearance(readAppearance());
    };
    const closeOutside = (event: PointerEvent) => {
      if (options.current?.open && event.target instanceof Node && !options.current.contains(event.target)) {
        options.current.open = false;
      }
    };
    syncMode();
    query?.addEventListener('change', syncMode);
    window.addEventListener('storage', syncStorage);
    document.addEventListener('pointerdown', closeOutside);
    return () => {
      query?.removeEventListener('change', syncMode);
      window.removeEventListener('storage', syncStorage);
      document.removeEventListener('pointerdown', closeOutside);
    };
  }, []);

  function change(next: Appearance) {
    setAppearance(next);
    try {
      localStorage.setItem(storageKey, JSON.stringify(next));
    } catch {
      // Browser privacy settings can limit preferences to the current tab.
    }
  }

  return <div className="theme-controls">
    <button className="theme-toggle" type="button" aria-label={toggleTitle} title={toggleTitle}
      onClick={() => change({ ...appearance, mode: dark ? 'light' : 'dark' })}>
      <ToggleIcon size={15} aria-hidden="true" /><span>{dark ? '白天' : '夜间'}</span>
    </button>
    <details ref={options} className="theme-options" onKeyDown={event => {
      if (event.key === 'Escape' && options.current?.open) {
        event.preventDefault();
        options.current.open = false;
        trigger.current?.focus();
      }
    }} onBlur={event => {
      if (options.current && event.relatedTarget instanceof Node && !options.current.contains(event.relatedTarget)) {
        options.current.open = false;
      }
    }}>
      <summary ref={trigger} className="theme-trigger" aria-label="外观设置" title="外观设置">
        <Palette size={15} aria-hidden="true" /><span>外观</span>
      </summary>
      <div className="theme-panel">
        <fieldset><legend>显示模式</legend>
          <div className="theme-choices">{(Object.keys(modes) as (keyof typeof modes)[]).map(mode =>
            <label className="theme-choice" key={mode}>
              <input name="display-mode" type="radio" checked={appearance.mode === mode}
                onChange={() => change({ ...appearance, mode })} /><span>{modes[mode]}</span>
            </label>)}</div>
        </fieldset>
        <fieldset><legend>主题色</legend>
          <div className="theme-choices">{(Object.keys(accents) as (keyof typeof accents)[]).map(accent =>
            <label className="theme-choice" key={accent}>
              <input name="theme-accent" type="radio" checked={appearance.accent === accent}
                onChange={() => change({ ...appearance, accent })} />
              <span><i data-swatch={accent} aria-hidden="true" />{accents[accent]}</span>
            </label>)}</div>
        </fieldset>
      </div>
    </details>
  </div>;
}
