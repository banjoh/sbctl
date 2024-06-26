package sbctl

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubectl/pkg/scheme"
)

func init() {
	utilruntime.Must(extensionsv1.AddToScheme(scheme.Scheme))
}

func Decode(resource string, data []byte) (runtime.Object, *schema.GroupVersionKind, error) {
	originalData := data
	decode := scheme.Codecs.UniversalDeserializer().Decode
	decoded, gvk, err := decode(data, nil, nil)
	if err == nil {
		return decoded, gvk, nil
	}

	log.Warn("could not to decode data, will try adding list GVK", err)
	data, err = wrapListData(resource, data)
	if err != nil {
		log.Warn(err)
	} else {
		decoded, gvk, err = decode(data, nil, nil)
		if err != nil {
			log.Warn("could not decode wrapped data: ", err)
		}
	}

	if decoded == nil {
		// Try to decode object into an unstructured object
		var v unstructured.Unstructured
		err = json.Unmarshal(originalData, &v)
		if err == nil {
			o := v.DeepCopyObject()
			gvk := o.GetObjectKind().GroupVersionKind()
			return o, &gvk, nil
		}
		log.Warn("could not decode data into an unstructured object: ", err)

		// Try to decode object into an unstructured list
		var vList []unstructured.Unstructured
		err = json.Unmarshal(originalData, &vList)
		if err == nil && len(vList) > 0 {
			o := vList[0].DeepCopyObject()
			gvk := o.GetObjectKind().GroupVersionKind()
			list := unstructured.UnstructuredList{}
			list.SetGroupVersionKind(gvk)
			list.Items = append(list.Items, vList...)
			return &list, &gvk, nil
		}
		if err != nil {
			log.Warn("could not decode data into an unstructured list object: ", err)
		}
		return nil, nil, errors.Wrap(err, "could not decode data into a k8s object")
	}

	switch o := decoded.(type) {
	case *corev1.EventList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Event",
				Version: "v1",
			})
		}
	case *corev1.PodList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Pod",
				Version: "v1",
			})
		}
	case *corev1.LimitRangeList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "LimitRange",
				Version: "v1",
			})
		}
	case *corev1.ServiceList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Service",
				Version: "v1",
			})
		}
	case *corev1.NamespaceList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Namespace",
				Version: "v1",
			})
		}
	case *corev1.NodeList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Node",
				Version: "v1",
			})
		}
	case *corev1.PersistentVolumeList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "PersistentVolume",
				Version: "v1",
			})
		}
	case *corev1.PersistentVolumeClaimList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "PersistentVolumeClaim",
				Version: "v1",
			})
		}
	case *batchv1.JobList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "batch",
				Kind:    "Job",
				Version: "v1",
			})
		}
	case *batchv1beta1.CronJobList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "batch",
				Kind:    "CronJob",
				Version: "v1beta1",
			})
		}
	case *appsv1.DeploymentList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Kind:    "Deployment",
				Version: "v1",
			})
		}
	case *appsv1.ReplicaSetList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Kind:    "ReplicaSet",
				Version: "v1",
			})
		}
	case *appsv1.StatefulSetList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps",
				Kind:    "StatefulSet",
				Version: "v1",
			})
		}
	case *storagev1.StorageClassList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "storage.k8s.io",
				Kind:    "StorageClass",
				Version: "v1",
			})
		}
	case *extensionsv1.CustomResourceDefinitionList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apiextensions.k8s.io",
				Kind:    "CustomResourceDefinitionList",
				Version: "v1",
			})
		}
	case *corev1.ConfigMapList:
		for i := range o.Items {
			o.Items[i].GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "ConfigMap",
				Version: "v1",
			})
		}
	}

	return decoded, gvk, nil
}

func wrapListData(resource string, data []byte) ([]byte, error) {
	var kind, apiVersion string
	switch resource {
	case "pods":
		kind = "PodList"
		apiVersion = "v1"
	case "events":
		kind = "EventList"
		apiVersion = "v1"
	case "cronjobs":
		kind = "CronJobList"
		apiVersion = "batch/v1beta1"
	case "deployments":
		kind = "DeploymentList"
		apiVersion = "apps/v1"
	case "ingress", "ingresses":
		kind = "IngressList"
		apiVersion = "networking.k8s.io/v1"
	case "jobs":
		kind = "JobList"
		apiVersion = "batch/v1"
	case "limitranges":
		kind = "LimitRangeList"
		apiVersion = "v1"
	case "pvcs":
		kind = "PersistentVolumeClaimList"
		apiVersion = "v1"
	case "replicasets":
		kind = "ReplicaSetList"
		apiVersion = "apps/v1"
	case "services":
		kind = "ServiceList"
		apiVersion = "v1"
	case "statefulsets":
		kind = "StatefulSetList"
		apiVersion = "apps/v1"
	case "namespaces":
		kind = "NamespaceList"
		apiVersion = "v1"
	case "nodes":
		kind = "NodeList"
		apiVersion = "v1"
	case "persistentvolumes":
		kind = "PersistentVolumeList"
		apiVersion = "v1"
	case "persistentvolumeclaims":
		kind = "PersistentVolumeClaimList"
		apiVersion = "v1"
	case "storageclasses":
		kind = "StorageClassList"
		apiVersion = "storage.k8s.io/v1"
	case "customresourcedefinitions":
		kind = "CustomResourceDefinitionList"
		apiVersion = "apiextensions.k8s.io/v1"
	default:
		return nil, errors.Errorf("don't know how to wrap %s", resource)
	}

	return []byte(fmt.Sprintf(`{
		"kind": "%s",
		"apiVersion": "%s",
		"metadata": {
			"resourceVersion": "1"
		},
		"items": %s
	}`, kind, apiVersion, data)), nil
}

func ToUnstructured(o runtime.Object) (*unstructured.Unstructured, error) {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: data}, nil
}

func ToUnstructuredList(o runtime.Object) (*unstructured.UnstructuredList, error) {
	obj, err := ToUnstructured(o)
	if err != nil {
		return nil, err
	}

	return obj.ToList()
}

func SortUnstructuredList(list *unstructured.UnstructuredList) {
	sort.Slice(list.Items, func(i, j int) bool {
		iT := list.Items[i].GetCreationTimestamp()
		jT := list.Items[j].GetCreationTimestamp()
		return iT.Before(&jT)
	})
}
