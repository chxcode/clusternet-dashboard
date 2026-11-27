package kube

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	yamlv3 "gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/yaml"
)

type nativeKind struct{ version, resource string }

var nativeKinds = map[string]nativeKind{"Deployment": {"apps/v1", "deployments"}, "Service": {"v1", "services"}, "ConfigMap": {"v1", "configmaps"}}

func shadowGVR(kind string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "shadow", Version: "v1alpha1", Resource: nativeKinds[kind].resource}
}

const nativeOwnerAnnotation = "dashboard.clusternet.io/native-workload"
const nativeIntentAnnotation = "dashboard.clusternet.io/native-intent-sha256"
const nativeInputAnnotation = "dashboard.clusternet.io/native-input-sha256"
const nativeHubLabel = "clusternet.io/created-by"

func nativeManagedKey(key string) bool {
	prefix, _, ok := strings.Cut(key, "/")
	return ok && (prefix == "clusternet.io" || strings.HasSuffix(prefix, ".clusternet.io") || key == "kubectl.kubernetes.io/last-applied-configuration")
}

func safeNativeNode(n *yamlv3.Node, depth int) bool {
	if depth > 48 || n.Anchor != "" || n.Kind == yamlv3.AliasNode || n.Style&yamlv3.TaggedStyle != 0 {
		return false
	}
	switch n.Kind {
	case yamlv3.MappingNode:
		seen := map[string]bool{}
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yamlv3.ScalarNode || k.Tag != "!!str" || k.Anchor != "" || k.Style&yamlv3.TaggedStyle != 0 || seen[k.Value] || !safeNativeNode(n.Content[i+1], depth+1) {
				return false
			}
			seen[k.Value] = true
		}
	case yamlv3.DocumentNode, yamlv3.SequenceNode:
		for _, x := range n.Content {
			if !safeNativeNode(x, depth+1) {
				return false
			}
		}
	case yamlv3.ScalarNode:
		return n.Tag == "!!str" || n.Tag == "!!int" || n.Tag == "!!bool" || n.Tag == "!!float" || n.Tag == "!!null"
	default:
		return false
	}
	return true
}

func parseNativeYAML(ns, document string) ([]*unstructured.Unstructured, error) {
	bad := errors.New("请输入最多 16 份有效的 Deployment、Service 或 ConfigMap YAML，使用订阅命名空间且对象身份唯一。")
	if len(document) == 0 || len(document) > 256<<10 {
		return nil, bad
	}
	dec := yamlv3.NewDecoder(strings.NewReader(document))
	var result []*unstructured.Unstructured
	seen := map[string]bool{}
	for {
		var n yamlv3.Node
		err := dec.Decode(&n)
		if err == io.EOF {
			break
		}
		if err != nil || len(result) >= 16 || !safeNativeNode(&n, 0) {
			return nil, bad
		}
		if len(n.Content) != 1 || n.Content[0].Kind != yamlv3.MappingNode {
			return nil, bad
		}
		data, err := yamlv3.Marshal(n.Content[0])
		if err != nil {
			return nil, bad
		}
		raw, err := yaml.YAMLToJSONStrict(data)
		if err != nil {
			return nil, bad
		}
		o := &unstructured.Unstructured{}
		if o.UnmarshalJSON(raw) != nil {
			return nil, bad
		}
		if o.GetNamespace() == "" {
			o.SetNamespace(ns)
		}
		if o.GetNamespace() != ns || !validSourceName(ns, o.GetName()) || nativeKinds[o.GetKind()].version != o.GetAPIVersion() || o.GetAPIVersion() == "" {
			return nil, bad
		}
		if err = validateNativeDocument(o); err != nil {
			return nil, err
		}
		key := o.GetKind() + "/" + o.GetName()
		if seen[key] {
			return nil, bad
		}
		seen[key] = true
		result = append(result, o)
	}
	if len(result) == 0 {
		return nil, bad
	}
	return result, nil
}

func validateNativeDocument(o *unstructured.Unstructured) error {
	// The hub copies the source name into the Manifest config.name label.
	if len(o.GetName()) > 63 {
		return errors.New("源名称用作 Manifest 标签值，最多 63 个字符。")
	}
	invalid := errors.New("资源字段、类型或选择条件无效。请移除服务器管理的字段和 Clusternet 保留的元数据。")
	meta, ok := o.Object["metadata"].(map[string]any)
	if !ok {
		return invalid
	}
	for k := range meta {
		switch k {
		case "name", "namespace", "labels", "annotations":
		default:
			return invalid
		}
	}
	if _, ok := o.Object["status"]; ok {
		return invalid
	}
	for _, fp := range []string{"labels", "annotations"} {
		m, _, err := unstructured.NestedStringMap(o.Object, "metadata", fp)
		if err != nil {
			return invalid
		}
		for k, v := range m {
			if nativeManagedKey(k) || len(validation.IsQualifiedName(k)) != 0 || (fp == "labels" && len(validation.IsValidLabelValue(v)) != 0) {
				return invalid
			}
		}
	}
	buf, err := json.Marshal(o.Object)
	if err != nil {
		return invalid
	}
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.DisallowUnknownFields()
	switch o.GetKind() {
	case "Deployment":
		var d appsv1.Deployment
		if dec.Decode(&d) != nil || d.Spec.Selector == nil || len(d.Spec.Template.Spec.Containers) == 0 || d.Spec.Replicas != nil && *d.Spec.Replicas < 0 {
			return invalid
		}
		q, err := metav1.LabelSelectorAsSelector(d.Spec.Selector)
		if err != nil || q.Empty() || !q.Matches(labels.Set(d.Spec.Template.Labels)) {
			return invalid
		}
		// Keep the original document shape. A typed Deployment round-trip
		// would add fields that the operator deliberately left out.
		pod, _, _ := unstructured.NestedFieldNoCopy(o.Object, "spec", "template", "spec")
		normalizePodResources(pod)
		names := map[string]bool{}
		for _, c := range d.Spec.Template.Spec.Containers {
			if len(validation.IsDNS1123Label(c.Name)) != 0 || strings.TrimSpace(c.Image) == "" || names[c.Name] {
				return invalid
			}
			names[c.Name] = true
		}
	case "Service":
		var s corev1.Service
		if dec.Decode(&s) != nil || len(validation.IsDNS1035Label(o.GetName())) != 0 {
			return invalid
		}
		if err := checkServiceAllocations(o); err != nil {
			return err
		}
		if s.Spec.Type == corev1.ServiceTypeExternalName {
			if len(validation.IsDNS1123Subdomain(s.Spec.ExternalName)) != 0 {
				return invalid
			}
		} else if len(s.Spec.Ports) == 0 && s.Spec.ClusterIP != "None" {
			return invalid
		}
		for _, p := range s.Spec.Ports {
			if p.Port < 1 || p.Port > 65535 {
				return invalid
			}
		}
	case "ConfigMap":
		var c corev1.ConfigMap
		if dec.Decode(&c) != nil {
			return invalid
		}
		for k := range c.Data {
			if len(validation.IsConfigMapKey(k)) != 0 {
				return invalid
			}
			if _, ok := c.BinaryData[k]; ok {
				return invalid
			}
		}
		for k := range c.BinaryData {
			if len(validation.IsConfigMapKey(k)) != 0 {
				return invalid
			}
		}
	default:
		return invalid
	}
	return nil
}

func checkServiceAllocations(o *unstructured.Unstructured) error {
	spec, _ := o.Object["spec"].(map[string]any)
	if v, ok := spec["clusterIP"]; ok && v != corev1.ClusterIPNone {
		return errors.New("Shadow 不保留固定 spec.clusterIP，请省略该字段；Headless Service 可使用 None。")
	}
	if v, ok := spec["clusterIPs"]; ok {
		ips, _ := v.([]any)
		if len(ips) != 1 || ips[0] != corev1.ClusterIPNone {
			return errors.New("Shadow 不保留固定 spec.clusterIPs，请省略该字段；Headless Service 可使用 [None]。")
		}
	}
	if _, ok := spec["healthCheckNodePort"]; ok {
		return errors.New("请省略 spec.healthCheckNodePort，由各集群分配健康检查端口。")
	}
	ports, _ := spec["ports"].([]any)
	for _, p := range ports {
		port, _ := p.(map[string]any)
		if _, ok := port["nodePort"]; ok {
			return errors.New("Shadow 不保留固定 spec.ports[].nodePort，请保留 Service 类型并由各集群分配端口。")
		}
	}
	return nil
}

func normalizePodResources(pod any) {
	normalize := func(m any) {
		values, _ := m.(map[string]any)
		for key, value := range values {
			if q, err := resource.ParseQuantity(fmt.Sprint(value)); err == nil {
				values[key] = q.String()
			}
		}
	}
	requirements := func(m any) {
		fields, _ := m.(map[string]any)
		normalize(fields["requests"])
		normalize(fields["limits"])
	}
	spec, _ := pod.(map[string]any)
	requirements(spec["resources"])
	normalize(spec["overhead"])
	for _, group := range []string{"containers", "initContainers", "ephemeralContainers"} {
		items, _ := spec[group].([]any)
		for _, item := range items {
			c, _ := item.(map[string]any)
			requirements(c["resources"])
		}
	}
	volumes, _ := spec["volumes"].([]any)
	for _, v := range volumes {
		vol, _ := v.(map[string]any)
		r, _, _ := unstructured.NestedFieldNoCopy(vol, "ephemeral", "volumeClaimTemplate", "spec", "resources")
		requirements(r)
	}
}
