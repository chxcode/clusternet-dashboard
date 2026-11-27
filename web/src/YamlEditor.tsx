import { memo, useDeferredValue, useEffect, useId, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { analyzeYaml, MAX_YAML_BYTES, type YamlAnalysis, type YamlIssue } from './yamlSyntax';
import './yamlEditor.css';

export { analyzeYaml, checkYamlSyntax, MAX_YAML_BYTES, type YamlAnalysis, type YamlIssue } from './yamlSyntax';

type Props = {
  label: string; value: string; onChange?: (value: string) => void;
  readOnly?: boolean; disabled?: boolean; placeholder?: string; singleDocument?: boolean; className?: string;
  analysis?: YamlAnalysis;
};

// A long single-line flow collection remains editable without allocating a span per item.
const maxTokensPerLine = 256;
const lineHeight = 20;
const verticalPadding = 10;

export function useYamlAnalysis(value: string, singleDocument = false): YamlAnalysis {
  const deferred = useDeferredValue(value);
  const large = deferred.length >= 16 * 1024 && deferred.length <= MAX_YAML_BYTES;
  const immediate = useMemo(() => large ? undefined : analyzeYaml(deferred, singleDocument), [deferred, large, singleDocument]);
  const [background, setBackground] = useState<YamlAnalysis>(() => analyzeYaml(''));
  const idleWorker = useRef<Worker | undefined>(undefined);
  useEffect(() => () => idleWorker.current?.terminate(), []);
  useEffect(() => {
    if (!large) return;
    let active = true;
    let running: Worker | undefined;
    let watchdog: ReturnType<typeof setTimeout> | undefined;
    function release(reuse: boolean) {
      clearTimeout(watchdog);
      if (!running) return;
      running.onmessage = null; running.onerror = null; running.onmessageerror = null;
      if (reuse) idleWorker.current = running;
      else running.terminate();
      running = undefined;
    }
    const fallback = () => {
      release(false);
      if (active) setBackground(analyzeYaml(deferred, singleDocument));
    };
    const timer = setTimeout(() => {
      if (typeof Worker === 'undefined') { fallback(); return; }
      try {
        running = idleWorker.current ?? new Worker(new URL('./yamlSyntax.ts', import.meta.url), { type: 'module' });
        idleWorker.current = undefined;
        running.onmessage = (event: MessageEvent<YamlAnalysis>) => {
          if (event.data?.source !== deferred || event.data.singleDocument !== singleDocument) { fallback(); return; }
          release(true);
          if (active) setBackground(event.data);
        };
        running.onerror = fallback; running.onmessageerror = fallback;
        watchdog = setTimeout(fallback, 10000);
        running.postMessage({ source: deferred, singleDocument });
      } catch { fallback(); }
    }, 140);
    return () => { active = false; clearTimeout(timer); release(false); };
  }, [deferred, large, singleDocument]);
  return immediate ?? background;
}

function firstSpan(analysis: YamlAnalysis, offset: number) {
  let lo = 0, hi = analysis.spans.length;
  while (lo < hi) {
    const mid = (lo + hi) >>> 1;
    if (analysis.spans[mid].end <= offset) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

const PaintedText = memo(function PaintedText({ analysis, first, last }: { analysis: YamlAnalysis; first: number; last: number }) {
  const nodes: ReactNode[] = [];
  for (let line = first; line < Math.min(last, analysis.lineStarts.length); line++) {
    const from = analysis.lineStarts[line], to = analysis.lineStarts[line + 1] ?? analysis.text.length;
    let pos = from;
    const startIndex = firstSpan(analysis, from);
    const stopIndex = Math.min(startIndex + maxTokensPerLine, analysis.spans.length);
    for (let i = startIndex; i < stopIndex && analysis.spans[i].start < to; i++) {
      const token = analysis.spans[i], start = Math.max(pos, token.start), end = Math.min(to, token.end);
      if (end <= start) continue;
      if (start > pos) nodes.push(analysis.text.slice(pos, start));
      nodes.push(<span key={start} className={'yaml-token-' + token.tone}>{analysis.text.slice(start, end)}</span>);
      pos = end;
    }
    if (pos < to) nodes.push(analysis.text.slice(pos, to));
  }
  return <>{nodes}</>;
});

function LocalEditor(props: Props) {
  const analysis = useYamlAnalysis(props.value, props.singleDocument);
  return <EditorSurface {...props} analysis={analysis}/>;
}

export function YamlEditor(props: Props) {
  return props.analysis ? <EditorSurface {...props} analysis={props.analysis}/> : <LocalEditor {...props}/>;
}

function EditorSurface({ label, value, onChange, readOnly, disabled, placeholder, singleDocument = false, className,
  analysis }: Props & { analysis: YamlAnalysis }) {
  const id = useId(), issuesId = useId();
  const input = useRef<HTMLTextAreaElement>(null);
  const paint = useRef<HTMLPreElement>(null);
  const measure = useRef<HTMLSpanElement>(null);
  const [composing, setComposing] = useState(false);
  const [range, setRange] = useState({ first: 0, last: 48 });
  const current = analysis.source === value && analysis.singleDocument === singleDocument;
  const painted = current && !analysis.plain && !composing && Boolean(value);
  const problems = current && !analysis.tooLarge && analysis.issues.length > 0;
  const paused = current && analysis.tooLarge;

  function syncScroll() {
    const field = input.current;
    if (!field) return;
    if (paint.current) paint.current.style.transform = 'translate(' + -field.scrollLeft + 'px,' + -field.scrollTop + 'px)';
    const visibleLine = Math.max(0, Math.floor((field.scrollTop - verticalPadding) / lineHeight));
    const bucket = Math.floor(visibleLine / 12) * 12;
    const first = Math.max(0, bucket - 12), last = bucket + Math.ceil((field.clientHeight || 320) / lineHeight) + 24;
    setRange(prev => prev.first === first && prev.last === last ? prev : { first, last });
  }
  useLayoutEffect(syncScroll, [analysis, painted]);
  useEffect(() => {
    if (!input.current || typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(syncScroll);
    observer.observe(input.current);
    return () => observer.disconnect();
  }, []);

  function locate(issue: YamlIssue) {
    const field = input.current;
    if (!field || disabled) return;
    const text = field.value; // The native field represents CRLF as LF.
    let start = 0;
    for (let line = 1; line < issue.line; line++) {
      const next = text.indexOf('\n', start);
      if (next < 0) { start = text.length; break; }
      start = next + 1;
    }
    const eol = text.indexOf('\n', start);
    const at = Math.min(start + issue.column - 1, eol < 0 ? text.length : eol);
    const char = text.codePointAt(at);
    field.focus({ preventScroll: true });
    field.setSelectionRange(at, char === undefined || char === 10 ? at : at + (char > 0xffff ? 2 : 1));
    const style = getComputedStyle(field);
    const top = (issue.line - 1) * (parseFloat(style.lineHeight) || 20) + (parseFloat(style.paddingTop) || 0);
    if (top < field.scrollTop || top + 20 > field.scrollTop + field.clientHeight) field.scrollTop = Math.max(0, top - field.clientHeight / 3);
    if (measure.current) {
      measure.current.textContent = text.slice(start, at);
      const left = measure.current.getBoundingClientRect().width + (parseFloat(style.paddingLeft) || 0);
      measure.current.textContent = '';
      if (left < field.scrollLeft || left + 20 > field.scrollLeft + field.clientWidth) field.scrollLeft = Math.max(0, left - field.clientWidth / 2);
    }
    syncScroll();
  }
  return <div className={'yaml-editor' + (className ? ' ' + className : '')} data-invalid={problems || undefined}>
    <label htmlFor={id} className="yaml-title">{label}</label>
    <div className="yaml-stage" data-highlighted={painted}>
      {painted && <div className="yaml-overlay" aria-hidden="true"><pre className="yaml-paint" style={{ top: range.first * lineHeight }} ref={paint}><PaintedText analysis={analysis} first={range.first} last={range.last}/></pre></div>}
      <textarea id={id} ref={input} className="yaml-input" value={value} placeholder={placeholder} readOnly={readOnly || !onChange} disabled={disabled}
        wrap="off" dir="ltr" spellCheck={false} autoCapitalize="off" autoCorrect="off"
        aria-invalid={problems || undefined} aria-describedby={problems || paused ? issuesId : undefined}
        onChange={e => onChange?.(e.target.value)} onScroll={syncScroll}
        onCompositionStart={() => setComposing(true)} onCompositionEnd={() => setComposing(false)}/>
      <span aria-hidden="true" className="yaml-measure" ref={measure}/>
    </div>
    <div className="yaml-feedback" aria-live="polite">
      {!current && value.length >= 16 * 1024 && <small>正在校验 YAML…</small>}
      {paused && <p id={issuesId}>{analysis.issues[0].message}</p>}
      {problems && <div id={issuesId}><p>YAML 校验发现 {analysis.issues.length} 个问题</p><ol>
        {analysis.issues.slice(0, 8).map((issue, i) => <li key={i}><button type="button" disabled={disabled} onClick={() => locate(issue)}>
          第 {issue.line} 行，第 {issue.column} 列</button> {issue.message}</li>)}
      </ol>{analysis.issues.length > 8 && <small>显示前 8 项。</small>}</div>}
      {current && analysis.plain && !problems && !paused && <small>此文件使用纯文本显示，语法校验已完成。</small>}
    </div>
  </div>;
}
