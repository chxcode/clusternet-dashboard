package kube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
)

type TriageStore struct {
	Namespace, Name    string
	CreateOnFirstWrite bool
}

const triagePrefix = "release-"

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// ResourceVersion also changes on unrelated writes. Bind operator notes to the
// release incarnation, desired generation and failure actually shown instead.
func releaseIncident(obj *unstructured.Unstructured) string {
	if obj.GetUID() == "" || !strings.EqualFold(getNestedString(obj, "status", "phase"), "failed") {
		return ""
	}
	fields := []string{string(obj.GetUID()), strconv.FormatInt(obj.GetGeneration(), 10),
		strconv.FormatInt(getNestedInt64(obj, "status", "version"), 10), getNestedString(obj, "status", "lastDeployed"),
		getNestedString(obj, "status", "description"), getNestedString(obj, "status", "notes")}
	data, _ := json.Marshal(fields)
	return digest(string(data))
}

func validTriage(state string) bool { return state == "investigating" || state == "resolved" }

func (e *Explorer) TriageStates(ctx context.Context, store TriageStore) (map[string]string, error) {
	if e.client == nil {
		return nil, apierrors.NewServiceUnavailable("workspace storage unavailable")
	}
	cm, err := e.client.CoreV1().ConfigMaps(store.Namespace).Get(ctx, store.Name, metav1.GetOptions{})
	result := map[string]string{}
	if apierrors.IsNotFound(err) && store.CreateOnFirstWrite {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for key, value := range cm.Data {
		incident, state, _ := strings.Cut(value, ":")
		if !strings.HasPrefix(key, triagePrefix) || !validTriage(state) || len(incident) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(incident); err == nil {
			result[incident] = state
		}
	}
	return result, nil
}

func (e *Explorer) SetTriage(ctx context.Context, store TriageStore, ns, name, uid, incident, before, state string) error {
	if uid == "" || incident == "" || (before != "open" && !validTriage(before)) || (state != "open" && !validTriage(state)) {
		return apierrors.NewBadRequest("invalid release identity or follow-up state")
	}
	if e.dynamic == nil || e.client == nil {
		return apierrors.NewServiceUnavailable("workspace storage unavailable")
	}
	key := triagePrefix + digest(uid)
	resource := e.client.CoreV1().ConfigMaps(store.Namespace)
	conflict := apierrors.NewConflict(helmReleaseGVR.GroupResource(), name, errors.New("release or follow-up state changed; reload the failed release"))
	// Retry storage races with other operators. Each attempt rechecks the object
	// and affected entry. A second failure gets a new incident key.
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		return err != conflict && (apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err))
	}, func() error {
		obj, err := e.dynamic.Resource(helmReleaseGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if string(obj.GetUID()) != uid || obj.GetDeletionTimestamp() != nil || releaseIncident(obj) != incident {
			return conflict
		}
		cm, err := resource.Get(ctx, store.Name, metav1.GetOptions{})
		missing := apierrors.IsNotFound(err) && store.CreateOnFirstWrite
		if err != nil && !missing {
			return err
		}
		current := "open"
		if !missing {
			savedIncident, savedState, _ := strings.Cut(cm.Data[key], ":")
			if savedIncident == incident {
				current = savedState
			}
		}
		if current == state {
			return nil
		}
		if current != before {
			return conflict
		}
		value := incident + ":" + state
		if missing {
			_, err = resource.Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: store.Name, Namespace: store.Namespace},
				Data: map[string]string{key: value}}, metav1.CreateOptions{})
			return err
		}
		if cm.ResourceVersion == "" {
			return apierrors.NewServiceUnavailable("workspace storage has no resource version")
		}
		var after *string
		if state != "open" {
			after = &value
		}
		patch, _ := json.Marshal(map[string]any{"metadata": map[string]string{"resourceVersion": cm.ResourceVersion}, "data": map[string]*string{key: after}})
		_, err = resource.Patch(ctx, store.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	})
}
