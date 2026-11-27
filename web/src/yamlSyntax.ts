import { Composer, LineCounter, Parser, isScalar, visit, type CST, type YAMLError } from 'yaml';

export type YamlIssue = { line: number; column: number; message: string };
type Tone = 'key' | 'string' | 'number' | 'boolean' | 'null' | 'comment' | 'marker';
type Span = { start: number; end: number; tone: Tone };
export type YamlAnalysis = { source: string; text: string; lineStarts: number[]; singleDocument: boolean; issues: YamlIssue[]; spans: Span[]; plain: boolean; tooLarge: boolean };
export const MAX_YAML_BYTES = 256 * 1024;


function triviaSpans(tokens: CST.Token[], add: (start: number, end: number, tone: Tone) => void) {
  const stack = tokens.slice();
  while (stack.length) {
    const t = stack.pop()!;
    if ('items' in t) {
      for (const item of t.items) {
        for (const token of item.start) stack.push(token);
        if (item.sep) for (const token of item.sep) stack.push(token);
        if (item.key) stack.push(item.key);
        if (item.value) stack.push(item.value);
      }
    } else if (t.type === 'document') {
      for (const token of t.start) stack.push(token);
      if (t.value) stack.push(t.value);
    } else if (t.type === 'block-scalar') {
      for (const token of t.props) stack.push(token);
    } else if (t.type === 'comment') add(t.offset, t.offset + t.source.length, 'comment');
    else if (t.type === 'doc-start' || t.type === 'doc-end' || t.type === 'directive' || t.type === 'block-scalar-header' || t.type === 'anchor' || t.type === 'alias' || t.type === 'tag') {
      add(t.offset, t.offset + t.source.length, 'marker');
    }
    if ('end' in t && t.end) for (const token of t.end) stack.push(token);
  }
}

function issueMessage(error: YAMLError) {
  switch (error.code) {
    case 'DUPLICATE_KEY': return '映射中存在重复的键。';
    case 'MISSING_CHAR': return '缺少结束符或分隔符。' + error.message;
    case 'TAB_AS_INDENT': return '缩进应使用空格。';
    default: return error.message;
  }
}
function inspect(source: string, singleDocument: boolean, highlight: boolean): YamlAnalysis {
  const issues: YamlIssue[] = [];
  const spans: Span[] = [];
  const analysis = { source, text: source, lineStarts: [0], singleDocument, issues, spans, plain: false, tooLarge: false };
  if (source.length > MAX_YAML_BYTES || new TextEncoder().encode(source).length > MAX_YAML_BYTES) {
    issues.push({ line: 1, column: 1, message: 'YAML 超过 256 KiB，实时语法检查已暂停。' });
    analysis.tooLarge = true; analysis.plain = true; return analysis;
  }
  if (!source) return analysis;
  const text = source.includes('\r') ? source.replace(/\r\n?/g, '\n') : source;
  analysis.text = text;
  const lines = new LineCounter();
  const add = (start: number, end: number, tone: Tone) => {
    if (end > start) spans.push({ start, end, tone });
  };
  const report = (at: number, message: string) => {
    const pos = lines.linePos(Math.min(text.length, Math.max(0, at)));
    issues.push({ line: Math.max(1, pos.line), column: Math.max(1, pos.col), message });
  };
  try {
    const tokens = Array.from(new Parser(lines.addNewLine).parse(text));
    analysis.lineStarts = lines.lineStarts;
    // Chart source files use YAML 1.2; Kubernetes multi-document input uses YAML 1.1.
    const docs = new Composer({ version: singleDocument ? '1.2' : '1.1', prettyErrors: false }).compose(tokens, true, text.length);
    if (highlight) triviaSpans(tokens, add);
    let index = 0;
    for (const doc of docs) {
      if (++index === 2 && singleDocument) report(doc.range[0], '此处只接受一份 YAML 文档。');
      for (const error of doc.errors) report(error.pos[0], issueMessage(error));
      if (highlight && !analysis.plain) visit(doc, (key, scalar) => {
        if (!isScalar(scalar) || !scalar.range) return;
        let [start, end] = scalar.range;
        if (scalar.type === 'BLOCK_LITERAL' || scalar.type === 'BLOCK_FOLDED') {
          // The block header can carry a real YAML comment; the indented body is all text.
          const next = text.indexOf('\n', start);
          if (next < 0 || next >= end) return;
          start = next + 1;
        }
        const v = scalar.value;
        add(start, end, key === 'key' ? 'key' : v === null ? 'null' : typeof v === 'number' ? 'number' : typeof v === 'boolean' ? 'boolean' : 'string');
      });
    }
    spans.sort((a, b) => a.start - b.start);
    issues.sort((a, b) => a.line - b.line || a.column - b.column);
  } catch {
    // Keep arbitrary incomplete edits, including excessively nested input, fully editable.
    spans.length = 0; analysis.plain = true;
    issues.push({ line: 1, column: 1, message: '无法解析此 YAML，请检查结构与嵌套深度。' });
  }
  return analysis;
}

export const analyzeYaml = (source: string, singleDocument = false) => inspect(source, singleDocument, true);
export const checkYamlSyntax = (source: string, singleDocument = false) => inspect(source, singleDocument, false).issues;

// Vite bundles this module as a same-origin worker for large editor buffers.
if (typeof document === 'undefined' && typeof self !== 'undefined') {
  self.addEventListener('message', (event: MessageEvent<{ source: string; singleDocument: boolean }>) => {
    self.postMessage(analyzeYaml(event.data.source, event.data.singleDocument));
  });
}
