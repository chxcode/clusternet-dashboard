import type { Cluster } from './api';
import { CopyButton } from './CopyButton';
import { pathForPage } from './routing';
import './clusterEvidence.css';

const probes = ['readyz', 'livez', 'healthz'] as const;
const nodeStates = { readyNodes: '就绪', notReadyNodes: '未就绪', unknownNodes: '未知', lostNodes: '失联' } as const;
const resourceNames = { cpu: 'CPU', memory: '内存', pods: 'Pod' } as const;
const resourceKeys = ['cpu', 'memory', 'pods'] as const;
function reportedSwitch(value?: boolean) {
  return value === undefined ? '未上报' : value ? '已启用' : '未启用';
}
const formatTime = new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
});

export function ReportedTime({ value }: { value?: string }) {
  if (!value) return <span className="cw-muted">未上报</span>;
  const timestamp = new Date(value);
  return <time dateTime={value} title={value}>{Number.isNaN(timestamp.valueOf()) ? value : formatTime.format(timestamp)}</time>;
}

export function ClusterEvidence({ cluster }: { cluster: Cluster }) {
  const evidence = cluster.evidence;
  const conditions = evidence?.conditions ?? [];
  return (
    <section className="cluster-evidence" aria-label={`${cluster.name} 的集群诊断`}>
      <header>
        <h3>集群诊断</h3>
        <span>最近状态观测 · <ReportedTime value={cluster.lastObservedTime} /></span>
      </header>
      <div className="cluster-evidence-grid">
        <section>
          <h4>健康条件</h4>
          {conditions.length > 0 ? <ul className="cluster-conditions">
            {conditions.map((condition, index) => <li key={`${condition.type}-${index}`}>
              <strong>{condition.type} = {condition.status || '未上报'}</strong>
              {condition.reason && <code>{condition.reason}</code>}
              {condition.message && <p>{condition.message}</p>}
              <small>最近转换 · <ReportedTime value={condition.lastTransitionTime} /></small>
            </li>)}
          </ul> : <p>未上报健康条件</p>}
        </section>
        <section>
          <h4>父集群记录的健康探针</h4>
          <dl>{probes.map(key => <div key={key}><dt>{key}</dt><dd>
            {evidence?.[key] === undefined ? '未上报' : evidence[key] ? '通过' : '未通过'}
          </dd></div>)}</dl>
        </section>
        <section>
          <h4>上报与同步</h4>
          <dl>
            <div><dt>心跳上报周期</dt><dd>{evidence?.heartbeatFrequencySeconds === undefined ? '未上报' : `${evidence.heartbeatFrequencySeconds} 秒`}</dd></div>
            <div><dt>平台</dt><dd>{cluster.platform || '未上报'}</dd></div>
            <div><dt>集群类型</dt><dd>{evidence?.clusterType || '未上报'}</dd></div>
            <div><dt>同步模式</dt><dd>{evidence?.syncMode || '未上报'}</dd></div>
          </dl>
        </section>
        <section>
          <h4>集群身份</h4>
          <dl>
            <div><dt>注册 ID</dt><dd>{cluster.clusterId ? <><code>{cluster.clusterId}</code>{' '}
              <CopyButton value={cluster.clusterId} label="复制注册 ID" /></> : '未上报'}</dd></div>
            <div><dt>资源创建时间</dt><dd><ReportedTime value={evidence?.createdAt} /></dd></div>
          </dl>
          <a href={pathForPage('releases') + '?' + new URLSearchParams({ q: cluster.name })}>查看相关发布</a>
        </section>
        <section>
          <h4>网络与连接</h4>
          <dl>
            <div><dt>API Server</dt><dd>{cluster.apiserverURL || '未上报'}</dd></div>
            <div><dt>集群 CIDR</dt><dd>{evidence?.clusterCIDRs?.join('、') || '未上报'}</dd></div>
            {!!evidence?.serviceCIDRs?.length && <div><dt>Service CIDR</dt><dd>{evidence.serviceCIDRs.join('、')}</dd></div>}
            <div><dt>Socket 隧道</dt><dd>{reportedSwitch(evidence?.useSocket)}</dd></div>
            <div><dt>应用推送</dt><dd>{reportedSwitch(evidence?.appPusher)}</dd></div>
          </dl>
        </section>
        <section>
          <h4>调度预测器</h4>
          <dl>
            <div><dt>预测服务</dt><dd>{reportedSwitch(evidence?.predictorEnabled)}</dd></div>
            {evidence?.predictorDirectAccess !== undefined && <div><dt>直连访问</dt><dd>{reportedSwitch(evidence.predictorDirectAccess)}</dd></div>}
            {evidence?.predictorEndpoint && <div><dt>服务地址</dt><dd>{evidence.predictorEndpoint}</dd></div>}
          </dl>
        </section>
        <section>
          <h4>节点状态</h4>
          <dl>{(Object.keys(nodeStates) as (keyof typeof nodeStates)[]).map(key => <div key={key}>
            <dt>{nodeStates[key]}</dt><dd>{evidence?.nodeStatistics?.[key] ?? '未上报'}</dd>
          </div>)}</dl>
        </section>
        <section className="cluster-resources">
          <h4>资源容量</h4>
          <div role="table" aria-label="上报的资源容量与可分配量">
            <div role="row"><span role="columnheader">资源</span><span role="columnheader">总容量</span><span role="columnheader">可分配量</span></div>
            {resourceKeys.map(key => <div key={key} role="row">
              <span role="rowheader">{resourceNames[key]}</span><span role="cell">{evidence?.capacity?.[key] ?? '未上报'}</span>
              <span role="cell">{evidence?.allocatable?.[key] ?? '未上报'}</span>
            </div>)}
          </div>
        </section>
        <section>
          <h4>集群调度污点</h4>
          {evidence?.taints === undefined ? <p>未提供污点信息</p> : evidence.taints.length === 0 ? <p>未配置调度污点</p> : (
            <dl>{evidence.taints.map(taint => <div key={`${taint.key}/${taint.effect}`}>
              <dt><code>{taint.key}</code></dt><dd>{taint.effect || '未指定'}</dd>
            </div>)}</dl>
          )}
        </section>
      </div>
    </section>
  );
}
