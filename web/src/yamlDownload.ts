export function downloadYAML(raw: string, filename: string) {
  const url = URL.createObjectURL(new Blob([raw], { type: 'application/yaml;charset=utf-8' }));
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
