import { parseAllDocuments } from 'yaml';

export const templates = {
  Deployment: `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 1
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: web
          image: nginx:alpine
          ports:
            - containerPort: 80
`,
  Service: `apiVersion: v1
kind: Service
metadata:
  name: web
spec:
  selector:
    app: web
  ports:
    - name: http
      port: 80
      targetPort: 80
`,
  ConfigMap: `apiVersion: v1
kind: ConfigMap
metadata:
  name: web-config
data:
  LOG_LEVEL: info
`,
};
export type Kind = keyof typeof templates;
type ObjectMap = Record<string, unknown>;
const isMap = (value: unknown): value is ObjectMap => value !== null && typeof value === 'object' && !Array.isArray(value);
const hasText = (value: unknown): value is string => typeof value === 'string' && value.trim().length > 0;
const isStringMap = (value: unknown): value is Record<string, string> => isMap(value) && Object.values(value).every(item => typeof item === 'string');
const owns = (object: object, key: string) => Object.prototype.hasOwnProperty.call(object, key);
const portNumber = (value: unknown) => typeof value === 'number' && Number.isInteger(value) && value > 0 && value <= 65535;
const dnsLabel = /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$/;
const dnsSubdomain = /^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?(?:\.[a-z0-9](?:[-a-z0-9]*[a-z0-9])?)*$/;

function deploymentError(spec: ObjectMap) {
  if (spec.replicas !== undefined && (typeof spec.replicas !== 'number' || !Number.isInteger(spec.replicas) || spec.replicas < 0)) return 'spec.replicas 应为非负整数。';
  const pod = isMap(spec.template) && isMap(spec.template.spec) ? spec.template.spec : undefined;
  if (!Array.isArray(pod?.containers) || !pod.containers.length || pod.containers.some(c => !isMap(c) || !hasText(c.name) || !hasText(c.image))) {
    return 'spec.template.spec.containers 至少需要一个包含 name 和 image 的容器。';
  }
  const selector = spec.selector;
  if (!isMap(selector)) return '缺少 spec.selector。';
  const labels = isMap(spec.template) && isMap(spec.template.metadata) ? spec.template.metadata.labels ?? {} : {};
  const exact = selector.matchLabels ?? {};
  const expressions = selector.matchExpressions ?? [];
  if (!isStringMap(labels) || !isStringMap(exact) || !Array.isArray(expressions)) return '请检查 Pod 标签与 spec.selector 的格式。';
  if (!Object.keys(exact).length && !expressions.length) return 'spec.selector 需要 matchLabels 或 matchExpressions。';
  if (Object.entries(exact).some(([key, value]) => labels[key] !== value)) return 'spec.selector.matchLabels 与 Pod 模板标签不匹配。';
  for (const expr of expressions) {
    if (!isMap(expr) || !hasText(expr.key)) return 'spec.selector.matchExpressions 中的条件缺少 key。';
    const values = expr.values ?? [];
    const has = owns(labels, expr.key);
    if (!Array.isArray(values) || !values.every(v => typeof v === 'string')) return 'matchExpressions 的 values 应为字符串列表。';
    let matches: boolean;
    if (expr.operator === 'In' || expr.operator === 'NotIn') {
      if (!values.length) return 'In 和 NotIn 条件至少需要一个值。';
      const included = has && values.includes(labels[expr.key]);
      matches = expr.operator === 'In' ? included : !included;
    } else if (expr.operator === 'Exists' || expr.operator === 'DoesNotExist') {
      if (values.length) return 'Exists 和 DoesNotExist 条件不应带有值。';
      matches = expr.operator === 'Exists' ? has : !has;
    } else return 'spec.selector.matchExpressions 使用了不支持的 operator。';
    if (!matches) return 'spec.selector.matchExpressions 与 Pod 模板标签不匹配。';
  }
}

function serviceError(spec: ObjectMap) {
  if (spec.type !== undefined && !['ClusterIP', 'NodePort', 'LoadBalancer', 'ExternalName'].some(type => type === spec.type)) return 'spec.type 不是受支持的 Service 类型。';
  if (spec.selector != null && !isStringMap(spec.selector)) return 'spec.selector 应为字符串标签映射。';
  if (spec.type === 'ExternalName' && !hasText(spec.externalName)) return 'ExternalName Service 需要 spec.externalName。';
  const ports = spec.ports ?? [];
  if (!Array.isArray(ports)) return 'spec.ports 应为列表。';
  if (!ports.length && spec.type !== 'ExternalName' && spec.clusterIP !== 'None') return 'spec.ports 至少需要一个端口。';
  for (const port of ports) {
    if (!isMap(port) || !portNumber(port.port)) return 'Service 的 port 应为 1 至 65535 之间的整数。';
    if (port.targetPort !== undefined && !portNumber(port.targetPort) && !hasText(port.targetPort)) return 'targetPort 应为端口号或命名端口。';
  }
}

function kindError(kind: Kind, item: ObjectMap) {
  if (kind === 'ConfigMap') {
    const data = item.data ?? {};
    const binary = item.binaryData ?? {};
    if (!isStringMap(data) || !isStringMap(binary)) return 'data 和 binaryData 的值应为字符串，数字或布尔文本请加引号。';
    if (Object.keys(binary).some(key => owns(data, key))) return 'data 和 binaryData 不能包含同名键。';
    if ([...Object.keys(data), ...Object.keys(binary)].some(key => key.length > 253 || !/^[-._a-zA-Z0-9]+$/.test(key))) return 'ConfigMap 的键名应为字母、数字、点、下划线或连字符。';
    if (Object.values(binary).some(value => !/^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$/.test(value.replace(/[\r\n]/g, '')))) return 'binaryData 的值需要使用 Base64 编码。';
    return;
  }
  if (!isMap(item.spec)) return '缺少资源的 spec 对象。';
  return kind === 'Deployment' ? deploymentError(item.spec) : serviceError(item.spec);
}

export type Check = { errors: string[]; resources: string[] };
export function validate(text: string): Check {
  const errors: string[] = [];
  const resources: string[] = [];
  const seen = new Set<string>();
  let docs: ReturnType<typeof parseAllDocuments>;
  try { docs = parseAllDocuments(text, { version: '1.1' }); }
  catch { return { errors: ['无法解析此 YAML 文档。'], resources: [] }; }
  for (const [index, doc] of docs.entries()) {
    const prefix = `文档 ${index + 1} · `;
    const problem = doc.errors[0] ?? doc.warnings[0];
    if (problem) { errors.push(prefix + problem.message); continue; }
    try {
      const item: unknown = doc.toJS({ maxAliasCount: 100 });
      if (item === null) continue;
      if (!isMap(item) || typeof item.kind !== 'string' || !owns(templates, item.kind)) {
        errors.push(prefix + '支持的资源类型为 Deployment、Service 和 ConfigMap。'); continue;
      }
      const kind = item.kind as Kind;
      const version = kind === 'Deployment' ? 'apps/v1' : 'v1';
      if (item.apiVersion !== version) { errors.push(prefix + `${kind} 需要 apiVersion ${version}。`); continue; }
      if (!isMap(item.metadata) || !hasText(item.metadata.name)) { errors.push(prefix + '缺少 metadata.name。'); continue; }
      const { name, namespace } = item.metadata;
      if (name.length > (kind === 'Service' ? 63 : 253) || !(kind === 'Service' ? dnsLabel : dnsSubdomain).test(name)) {
        errors.push(prefix + 'metadata.name 不是有效的 Kubernetes 资源名称。'); continue;
      }
      if (namespace != null && (!hasText(namespace) || namespace.length > 63 || !dnsLabel.test(namespace))) { errors.push(prefix + 'metadata.namespace 应为有效的命名空间名称。'); continue; }
      const identity = [kind, item.metadata.namespace ?? '', item.metadata.name].join('/');
      if (seen.has(identity)) { errors.push(prefix + '与前面的文档定义了同一个资源。'); continue; }
      seen.add(identity);
      const issue = kindError(kind, item);
      if (issue) errors.push(prefix + issue);
      else resources.push(`${kind} · ${item.metadata.namespace ? item.metadata.namespace + '/' : ''}${item.metadata.name}`);
    } catch { errors.push(prefix + '无法读取 YAML 引用或文档结构。'); }
  }
  if (!errors.length && !resources.length) errors.push('请先输入、导入 YAML 或添加模板。');
  return { errors, resources };
}

export function appendDocument(current: string, next: string) {
  if (!current.trim()) return next;
  const separator = /^---(?:\s|$)/.test(next) ? '' : '---\n';
  return current + (current.endsWith('\n') ? '' : '\n') + separator + next;
}
