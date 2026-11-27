import { downloadYAML } from './yamlDownload';
import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { FilePenLine, X } from 'lucide-react';
import { appendDocument, templates, validate, type Check, type Kind } from './workloadDraft';
import { useYamlAnalysis, YamlEditor } from './YamlEditor';
import './clusterLabels.css';
import './workloadDraftEditor.css';

function DraftDialog({ text, setText, onClose, onPreparePublish }: {
  text: string; setText: React.Dispatch<React.SetStateAction<string>>; onClose: () => void; onPreparePublish?: (text: string) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [kind, setKind] = useState<Kind>('Deployment');
  const [checked, setChecked] = useState<Check>();
  const [message, setMessage] = useState('');
  const [loading, setLoading] = useState(false);
  const [importError, setImportError] = useState('');
  const [reset, setReset] = useState(false);
  const reader = useRef<FileReader | null>(null);
  const analysis = useYamlAnalysis(text);
  const syntaxReady = analysis.source === text && !analysis.tooLarge && !analysis.issues.length;
  useEffect(() => {
    const node = dialog.current!;
    if (typeof node.showModal === 'function') node.showModal(); else node.setAttribute('open', '');
    return () => { reader.current?.abort(); node.close?.(); };
  }, []);

  function change(content: string | ((previous: string) => string)) {
    setText(content); setChecked(undefined); setMessage(''); setImportError(''); setReset(false);
  }
  function importFile(file?: File) {
    if (!file) return;
    const next = new FileReader();
    reader.current = next; setLoading(true); setImportError('');
    next.onload = () => {
      const content = String(next.result ?? '');
      if (content.trim()) { change(previous => appendDocument(previous, content)); setMessage(`已导入 ${file.name}`); }
      else setImportError('文件为空，请选择含有资源的 YAML 文件。');
    };
    next.onerror = () => setImportError('读取本地文件失败，请重新选择文件。');
    next.onloadend = () => { setLoading(false); reader.current = null; };
    next.readAsText(file);
  }
  function download() {
    const check = validate(text);
    setChecked(check);
    downloadYAML(text, 'workloads.yaml');
    setMessage(check.errors.length ? '已下载原文草稿，发布前请修正下方问题。' : '已下载 workloads.yaml。');
  }
  return createPortal(<dialog ref={dialog} className="label-editor workload-editor" aria-labelledby="native-yaml-title"
    onCancel={event => { event.preventDefault(); onClose(); }}>
    <header><h2 id="native-yaml-title">本地工作负载 YAML</h2><button type="button" className="label-icon" aria-label="关闭编辑器" onClick={onClose}><X size={17} /></button></header>
    <p className="label-target">在浏览器内编辑和导出 Kubernetes 资源文件。离开此页面前请下载草稿。</p>
    <div className="workload-toolbar">
      <label>模板<select value={kind} onChange={event => setKind(event.target.value as Kind)}>{Object.keys(templates).map(key => <option key={key}>{key}</option>)}</select></label>
      <button type="button" onClick={() => change(previous => appendDocument(previous, templates[kind]))}>添加模板</button>
      <label className="workload-import">{loading ? '导入中…' : text.trim() ? '追加 YAML 文件' : '导入 YAML 文件'}
        <input type="file" accept=".yaml,.yml,.json,application/yaml,text/yaml,application/json" disabled={loading}
          onChange={event => { importFile(event.target.files?.[0]); event.target.value = ''; }} /></label>
      {text && <button type="button" disabled={loading} onClick={() => setReset(true)}>新建空白文件</button>}
    </div>
    {reset && <div className="workload-reset">清空当前编辑内容并新建文件？ <button type="button" onClick={() => setReset(false)}>保留草稿</button>
      <button type="button" onClick={() => change('')}>确认新建</button></div>}
    {importError && <p className="label-error" role="alert">{importError}</p>}
    <YamlEditor className="workload-source" label="YAML 源文件" value={text} analysis={analysis} onChange={change}
      placeholder="粘贴 YAML 或添加模板，多份资源可用 --- 分隔。"/>
    {analysis.source === text && analysis.tooLarge && <p className="label-target">
      可点击“校验 YAML”完整检查本地草稿，也可下载原文。{onPreparePublish && '在线发布上限为 256 KiB。'}
    </p>}
    {message && <p role="status">{message}</p>}
    {checked && (checked.errors.length ? <div className="label-error" role="alert"><strong>本地校验发现 {checked.errors.length} 个问题</strong>
      <ul>{checked.errors.map((error, i) => <li key={i}>{error}</li>)}</ul></div> : <div role="status">本地基础校验通过 · {checked.resources.length} 个资源
      <ul>{checked.resources.map(resource => <li key={resource}>{resource}</li>)}</ul></div>)}
    <p className="label-impact">下载后可用 <code>kubectl clusternet apply -f workloads.yaml</code> 提交分发资源，并通过 Subscription 选择目标集群。</p>
    <footer><button type="button" disabled={loading} onClick={() => { setMessage(''); setChecked(validate(text)); }}>校验 YAML</button>
      <button type="button" className="label-save" disabled={!text.trim() || loading} onClick={download}>下载 YAML</button>
      {onPreparePublish && <button type="button" disabled={!text.trim() || loading || !syntaxReady} onClick={() => {
        if (!syntaxReady) return;
        const check = validate(text); setChecked(check);
        if (!check.errors.length) { onClose(); onPreparePublish(text); }
      }}>用此草稿准备发布</button>}</footer>
  </dialog>, document.body);
}

export function WorkloadDraftEditor({ onPreparePublish }: { onPreparePublish?: (text: string) => void }) {
  const [open, setOpen] = useState(false);
  const [text, setText] = useState('');
  return <><button type="button" className="label-edit" onClick={() => setOpen(true)}><FilePenLine size={14} /> 编辑本地 YAML</button>
    {open && <DraftDialog text={text} setText={setText} onPreparePublish={onPreparePublish} onClose={() => setOpen(false)} />}</>;
}
