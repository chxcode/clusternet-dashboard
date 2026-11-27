import { useEffect, useRef, useState } from 'react';
import { Check, Copy } from 'lucide-react';

export function CopyButton({ value, label }: { value: string; label: string }) {
  const [status, setStatus] = useState('');
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);

  async function copy() {
    let copied = false;
    try {
      await navigator.clipboard.writeText(value);
      copied = true;
    } catch {
      // Clipboard API may be unavailable on a private HTTP installation.
      const previous = document.activeElement;
      const input = document.createElement('textarea');
      input.value = value;
      input.style.cssText = 'position:fixed;left:-10000px;top:0';
      document.body.appendChild(input);
      input.select();
      try { copied = document.execCommand('copy'); } catch { /* manual selection remains available */ }
      input.remove();
      if (previous instanceof HTMLElement) previous.focus({ preventScroll: true });
    }
    clearTimeout(timer.current);
    setStatus(copied ? '已复制' : '复制失败，请选中文本复制');
    timer.current = setTimeout(() => setStatus(''), 2400);
  }
  return <span className="copy-control">
    <button type="button" className="copy-icon" aria-label={label} title={status || label} onClick={copy}>
      {status === '已复制' ? <Check size={14} /> : <Copy size={14} />}
    </button>
    <span role="status" className="copy-feedback">{status}</span>
  </span>;
}
